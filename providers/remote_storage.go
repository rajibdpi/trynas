package providers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rajibdpi/trynas/core"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/operations"
)

func (a *Module) OpenAttachedStorage(ctx context.Context, id string) (fs.Fs, func(), int) {
	a.Providers.Mu.Lock()
	connections, err := a.Connections()
	if err != nil {
		a.Providers.Mu.Unlock()
		return nil, nil, 500
	}
	var c core.StorageConnection
	found := false
	for _, candidate := range connections {
		if candidate.ID == id && candidate.Attached {
			c = candidate
			found = true
			break
		}
	}
	if !found {
		a.Providers.Mu.Unlock()
		return nil, nil, 404
	}
	// Serialize remote operations, including streams, to avoid overlapping writes
	// and token refreshes against the same connection.
	if a.Providers.RemoteOps > 0 {
		a.Providers.Mu.Unlock()
		return nil, nil, 409
	}
	a.Providers.RemoteOps++
	a.Providers.Mu.Unlock()
	var options configmap.Simple
	finish := func() {
		a.Providers.Mu.Lock()
		defer a.Providers.Mu.Unlock()
		a.Providers.RemoteOps--
		if options != nil && options["token"] != "" {
			current, e := a.Connections()
			if e == nil {
				for i := range current {
					if current[i].ID == id {
						current[i].Options["token"] = options["token"]
						_ = a.SaveConnections(current)
						break
					}
				}
			}
		}
	}
	f, options, err := NewProviderFS(ctx, c)
	if err != nil {
		finish()
		return nil, nil, 502
	}
	return f, finish, 200
}

func RemoteFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, fs.ErrorObjectNotFound) || errors.Is(err, fs.ErrorDirNotFound) {
		core.Bad(w, "remote item not found", 404)
		return
	}
	core.Bad(w, "storage operation failed; check the connection and remote permissions", 502)
}

func (a *Module) RemoteStorage(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	if u.Role != "admin" {
		core.Bad(w, "administrator required", 403)
		return
	}
	id := r.URL.Query().Get("storage")
	p, err := core.CleanPath(r.URL.Query().Get("path"))
	if err != nil {
		core.Bad(w, "invalid storage path", 400)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet && r.URL.Query().Get("download") != "1" && r.URL.Query().Get("refresh") != "true" {
		if data := a.cachedListing(id, p); data != nil {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(data)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	f, finish, status := a.OpenAttachedStorage(ctx, id)
	if status != 200 {
		switch status {
		case 404:
			core.Bad(w, "attached storage not found", 404)
		case 409:
			core.Bad(w, "Loading…", 409)
		default:
			core.Bad(w, "could not connect to storage", status)
		}
		return
	}
	defer finish()
	if r.Method != http.MethodGet {
		defer a.invalidateListings(id)
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		if r.URL.Query().Get("download") == "1" {
			obj, err := f.NewObject(ctx, p)
			if err != nil {
				RemoteFailure(w, err)
				return
			}
			reader, err := obj.Open(ctx)
			if err != nil {
				RemoteFailure(w, err)
				return
			}
			defer reader.Close()
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(p)}))
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			if obj.Size() >= 0 {
				w.Header().Set("Content-Length", strconv.FormatInt(obj.Size(), 10))
			}
			_, _ = io.Copy(w, reader)
			return
		}
		entries, err := f.List(ctx, p)
		if err != nil {
			RemoteFailure(w, err)
			return
		}
		items := []map[string]any{}
		for _, entry := range entries {
			rp, e := core.CleanPath(entry.Remote())
			if e != nil || path.Dir(rp) != DirectoryParent(p) {
				continue
			}
			_, dir := entry.(fs.Directory)
			items = append(items, map[string]any{"name": path.Base(rp), "path": rp, "dir": dir, "size": entry.Size(), "modified": entry.ModTime(ctx)})
		}
		sort.Slice(items, func(i, j int) bool {
			di, dj := items[i]["dir"].(bool), items[j]["dir"].(bool)
			if di != dj {
				return di
			}
			return strings.ToLower(items[i]["name"].(string)) < strings.ToLower(items[j]["name"].(string))
		})
		data, err := json.Marshal(map[string]any{"items": items, "path": p})
		if err != nil {
			core.Bad(w, "could not encode storage listing", 500)
			return
		}
		a.cacheListing(id, p, data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
		return
	}
	if r.URL.Path == "/api/storage/upload" {
		r.Body = http.MaxBytesReader(w, r.Body, 10<<30)
		reader, err := r.MultipartReader()
		if err != nil {
			core.Bad(w, "multipart upload required", 400)
			return
		}
		count := 0
		for {
			part, e := reader.NextPart()
			if e == io.EOF {
				break
			}
			if e != nil {
				core.Bad(w, "invalid or oversized upload", 400)
				return
			}
			if part.FileName() == "" {
				part.Close()
				continue
			}
			name := part.FileName()
			clean, e := core.CleanPath(name)
			if e != nil || clean == "" || strings.Contains(clean, "/") {
				part.Close()
				core.Bad(w, "invalid file name", 400)
				return
			}
			target := core.RelJoin(p, clean)
			// Avoid silently replacing remote content when uploading from the browser.
			if _, e = f.NewObject(ctx, target); e == nil {
				part.Close()
				core.Bad(w, "a file with this name already exists", 409)
				return
			} else if !errors.Is(e, fs.ErrorObjectNotFound) {
				part.Close()
				RemoteFailure(w, e)
				return
			}
			_, e = operations.Rcat(ctx, f, target, part, time.Now(), nil)
			part.Close()
			if e != nil {
				RemoteFailure(w, e)
				return
			}
			count++
		}
		if count == 0 {
			core.Bad(w, "choose at least one file", 400)
			return
		}
		a.Log(u.ID, "remote-upload", id+"/"+p, u.IP)
		core.JSON(w, map[string]any{"files": count})
		return
	}
	var input struct {
		Action  string `json:"action"`
		Name    string `json:"name"`
		Dir     bool   `json:"dir"`
		Confirm bool   `json:"confirm"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		core.Bad(w, "invalid request", 400)
		return
	}
	switch input.Action {
	case "mkdir":
		name, e := core.CleanPath(input.Name)
		if e != nil || name == "" || strings.Contains(name, "/") {
			core.Bad(w, "invalid folder name", 400)
			return
		}
		err = f.Mkdir(ctx, core.RelJoin(p, name))
	case "delete":
		if p == "" || !input.Confirm {
			core.Bad(w, "confirm deletion of a non-root item", 400)
			return
		}
		if input.Dir {
			err = operations.Purge(ctx, f, p)
		} else {
			var obj fs.Object
			obj, err = f.NewObject(ctx, p)
			if err == nil {
				err = obj.Remove(ctx)
			}
		}
	default:
		core.Bad(w, "unsupported storage action", 400)
		return
	}
	if err != nil {
		RemoteFailure(w, err)
		return
	}
	a.Log(u.ID, "remote-"+input.Action, id+"/"+p, u.IP)
	core.JSON(w, map[string]bool{"ok": true})
}

func DirectoryParent(p string) string {
	if p == "" {
		return "."
	}
	return p
}

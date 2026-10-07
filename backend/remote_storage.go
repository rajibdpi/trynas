package main

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

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/operations"
)

func (a *App) openAttachedStorage(ctx context.Context, id string) (fs.Fs, func(), int) {
	a.providers.mu.Lock()
	connections, err := a.connections()
	if err != nil {
		a.providers.mu.Unlock()
		return nil, nil, 500
	}
	var c storageConnection
	found := false
	for _, candidate := range connections {
		if candidate.ID == id && candidate.Attached {
			c = candidate
			found = true
			break
		}
	}
	if !found {
		a.providers.mu.Unlock()
		return nil, nil, 404
	}
	if a.providers.job != nil && a.providers.job.Status == "running" {
		a.providers.mu.Unlock()
		return nil, nil, 409
	}
	// Serialize remote operations, including streams, to avoid overlapping writes
	// and token refreshes against the same connection.
	if a.providers.remoteOps > 0 {
		a.providers.mu.Unlock()
		return nil, nil, 409
	}
	a.providers.remoteOps++
	a.providers.mu.Unlock()
	var options configmap.Simple
	finish := func() {
		a.providers.mu.Lock()
		defer a.providers.mu.Unlock()
		a.providers.remoteOps--
		if options != nil && options["token"] != "" {
			current, e := a.connections()
			if e == nil {
				for i := range current {
					if current[i].ID == id {
						current[i].Options["token"] = options["token"]
						_ = a.saveConnections(current)
						break
					}
				}
			}
		}
	}
	f, options, err := newProviderFS(ctx, c)
	if err != nil {
		finish()
		return nil, nil, 502
	}
	return f, finish, 200
}
func remoteFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, fs.ErrorObjectNotFound) || errors.Is(err, fs.ErrorDirNotFound) {
		bad(w, "remote item not found", 404)
		return
	}
	bad(w, "storage operation failed; check the connection and remote permissions", 502)
}
func (a *App) remoteStorage(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	if u.Role != "admin" {
		bad(w, "administrator required", 403)
		return
	}
	id := r.URL.Query().Get("storage")
	p, err := cleanPath(r.URL.Query().Get("path"))
	if err != nil {
		bad(w, "invalid storage path", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	f, finish, status := a.openAttachedStorage(ctx, id)
	if status != 200 {
		switch status {
		case 404:
			bad(w, "attached storage not found", 404)
		case 409:
			bad(w, "Loading…", 409)
		default:
			bad(w, "could not connect to storage", status)
		}
		return
	}
	defer finish()
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		if r.URL.Query().Get("download") == "1" {
			obj, err := f.NewObject(ctx, p)
			if err != nil {
				remoteFailure(w, err)
				return
			}
			reader, err := obj.Open(ctx)
			if err != nil {
				remoteFailure(w, err)
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
			remoteFailure(w, err)
			return
		}
		items := []map[string]any{}
		for _, entry := range entries {
			rp, e := cleanPath(entry.Remote())
			if e != nil || path.Dir(rp) != directoryParent(p) {
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
		jsonOut(w, map[string]any{"items": items, "path": p})
		return
	}
	if r.URL.Path == "/api/storage/upload" {
		r.Body = http.MaxBytesReader(w, r.Body, 10<<30)
		reader, err := r.MultipartReader()
		if err != nil {
			bad(w, "multipart upload required", 400)
			return
		}
		count := 0
		for {
			part, e := reader.NextPart()
			if e == io.EOF {
				break
			}
			if e != nil {
				bad(w, "invalid or oversized upload", 400)
				return
			}
			if part.FileName() == "" {
				part.Close()
				continue
			}
			name := part.FileName()
			clean, e := cleanPath(name)
			if e != nil || clean == "" || strings.Contains(clean, "/") {
				part.Close()
				bad(w, "invalid file name", 400)
				return
			}
			target := relJoin(p, clean)
			// Avoid silently replacing remote content when uploading from the browser.
			if _, e = f.NewObject(ctx, target); e == nil {
				part.Close()
				bad(w, "a file with this name already exists", 409)
				return
			} else if !errors.Is(e, fs.ErrorObjectNotFound) {
				part.Close()
				remoteFailure(w, e)
				return
			}
			_, e = operations.Rcat(ctx, f, target, part, time.Now(), nil)
			part.Close()
			if e != nil {
				remoteFailure(w, e)
				return
			}
			count++
		}
		if count == 0 {
			bad(w, "choose at least one file", 400)
			return
		}
		a.log(u.ID, "remote-upload", id+"/"+p)
		jsonOut(w, map[string]any{"files": count})
		return
	}
	var input struct {
		Action  string `json:"action"`
		Name    string `json:"name"`
		Dir     bool   `json:"dir"`
		Confirm bool   `json:"confirm"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		bad(w, "invalid request", 400)
		return
	}
	switch input.Action {
	case "mkdir":
		name, e := cleanPath(input.Name)
		if e != nil || name == "" || strings.Contains(name, "/") {
			bad(w, "invalid folder name", 400)
			return
		}
		err = f.Mkdir(ctx, relJoin(p, name))
	case "delete":
		if p == "" || !input.Confirm {
			bad(w, "confirm deletion of a non-root item", 400)
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
		bad(w, "unsupported storage action", 400)
		return
	}
	if err != nil {
		remoteFailure(w, err)
		return
	}
	a.log(u.ID, "remote-"+input.Action, id+"/"+p)
	jsonOut(w, map[string]bool{"ok": true})
}
func directoryParent(p string) string {
	if p == "" {
		return "."
	}
	return p
}

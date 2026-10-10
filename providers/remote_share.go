package providers

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/rajibdpi/trynas/core"
	"github.com/rajibdpi/trynas/share"
)

func (a *Module) ValidateRemoteShare(ctx context.Context, storage string, paths []string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	remotes, finish, err := a.actionConnections(ctx, []string{storage})
	if err != nil {
		return err
	}
	defer finish()
	for _, p := range paths {
		clean, e := core.CleanPath(p)
		if e != nil || clean == "" {
			return fmt.Errorf("select a non-root item")
		}
		exists, e := remoteExists(ctx, remotes[storage], clean)
		if e != nil {
			return e
		}
		if !exists {
			return fmt.Errorf("item not found")
		}
	}
	return nil
}

func (a *Module) ServeRemoteShare(w http.ResponseWriter, r *http.Request, storage string, paths []string) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	remotes, finish, err := a.actionConnections(ctx, []string{storage})
	if err != nil {
		core.Bad(w, "share unavailable; storage may be detached or busy", 503)
		return
	}
	defer finish()
	f := remotes[storage]
	for _, p := range paths {
		if clean, e := core.CleanPath(p); e != nil || clean == "" {
			core.Bad(w, "invalid shared path", 404)
			return
		}
	}
	if item, provided := r.URL.Query()["item"]; provided {
		if len(item) != 1 {
			core.Bad(w, "invalid shared item", 400)
			return
		}
		clean, e := core.CleanPath(item[0])
		if e != nil || clean == "" {
			core.Bad(w, "invalid shared item", 400)
			return
		}
		allowed := false
		for _, root := range paths {
			if core.Under(clean, root) {
				allowed = true
			}
		}
		if !allowed {
			core.Bad(w, "item not shared", 404)
			return
		}
		paths = []string{clean}
	}
	if r.URL.Query().Get("download") != "1" {
		title := path.Base(paths[0])
		if len(paths) > 1 {
			title = fmt.Sprintf("%d shared items", len(paths))
		}
		items := []core.SharedPageItem{}
		for _, p := range paths {
			exists, e := remoteExists(ctx, f, p)
			if e != nil || !exists {
				core.Bad(w, "shared item not found", 404)
				return
			}
			folder := true
			size := int64(0)
			if obj, e := f.NewObject(ctx, p); e == nil {
				folder = false
				size = obj.Size()
			}
			if len(paths) == 1 && !folder {
				share.ServeSharePage(w, r, title, size, false, nil, false)
				return
			}
			items = append(items, core.SharedPageItem{Name: path.Base(p), Kind: map[bool]string{true: "Folder", false: "File"}[folder], Folder: folder, DownloadURL: "?download=1&item=" + url.QueryEscape(p)})
		}
		share.ServeSharePage(w, r, title, 0, true, items, false, len(paths) > 1)
		return
	}
	if len(paths) == 1 {
		if obj, e := f.NewObject(ctx, paths[0]); e == nil {
			reader, e := obj.Open(ctx)
			if e != nil {
				RemoteFailure(w, e)
				return
			}
			defer reader.Close()
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(paths[0])}))
			_, _ = io.Copy(w, reader)
			return
		}
	}
	archive, err := makeRemoteZip(ctx, f, paths)
	if err != nil {
		core.Bad(w, "could not prepare shared archive", 502)
		return
	}
	defer os.Remove(archive)
	file, err := os.Open(archive)
	if err != nil {
		core.Bad(w, "share unavailable", 500)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		core.Bad(w, "share unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="shared-items.zip"`)
	http.ServeContent(w, r, "shared-items.zip", info.ModTime(), file)
}

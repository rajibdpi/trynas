package files

import (
	"encoding/json"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rajibdpi/trynas/core"
)

func (a *Module) ListFiles(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	f, e := core.Safe(a.Root, p)
	if e != nil {
		core.Bad(w, "invalid path", 400)
		return
	}
	es, e := os.ReadDir(f)
	if e != nil {
		core.Bad(w, e.Error(), 404)
		return
	}
	u := r.Context().Value(core.UserKey).(core.User)
	fav := a.FavoriteSet(u.ID)
	out := []map[string]any{}
	for _, x := range es {
		if x.Type()&os.ModeSymlink != 0 || strings.HasPrefix(x.Name(), ".trynas-") {
			continue
		}
		info, ie := x.Info()
		if ie != nil {
			continue
		}
		rp := core.RelJoin(p, x.Name())
		if !a.Can(u, rp, false) && !(x.IsDir() && a.CanBrowse(u, rp)) {
			continue
		}
		out = append(out, map[string]any{"name": x.Name(), "path": rp, "dir": x.IsDir(), "size": info.Size(), "modified": info.ModTime(), "favorite": fav[rp], "writable": a.Can(u, rp, true)})
	}
	sort.Slice(out, func(i, j int) bool {
		di := out[i]["dir"].(bool)
		dj := out[j]["dir"].(bool)
		if di != dj {
			return di
		}
		return strings.ToLower(out[i]["name"].(string)) < strings.ToLower(out[j]["name"].(string))
	})
	core.JSON(w, map[string]any{"path": p, "items": out, "writable": a.Can(u, p, true)})
}

func (a *Module) SearchFiles(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if q == "" {
		core.JSON(w, []any{})
		return
	}
	base := r.URL.Query().Get("path")
	start, e := core.Safe(a.Root, base)
	if e != nil {
		core.Bad(w, "invalid path", 400)
		return
	}
	out := []map[string]any{}
	_ = filepath.Walk(start, func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return nil
		}
		relPath, _ := filepath.Rel(a.Root, p)
		if i.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if strings.HasPrefix(i.Name(), ".trynas-") {
			return filepath.SkipDir
		}
		user := r.Context().Value(core.UserKey).(core.User)
		if !a.Can(user, filepath.ToSlash(relPath), false) {
			if i.IsDir() && !a.CanBrowse(user, filepath.ToSlash(relPath)) {
				return filepath.SkipDir
			}
			return nil
		}
		if p == a.Trash {
			return filepath.SkipDir
		}
		if p == start {
			return nil
		}
		if strings.Contains(strings.ToLower(i.Name()), q) {
			rel, _ := filepath.Rel(a.Root, p)
			out = append(out, map[string]any{"name": i.Name(), "path": filepath.ToSlash(rel), "dir": i.IsDir(), "size": i.Size(), "modified": i.ModTime(), "writable": a.Can(user, filepath.ToSlash(rel), true)})
			if len(out) >= 100 {
				return filepath.SkipAll
			}
		}
		return nil
	})
	core.JSON(w, out)
}

func (a *Module) Mkdir(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	var x struct{ Path string }
	_ = json.NewDecoder(r.Body).Decode(&x)
	f, e := core.Safe(a.Root, x.Path)
	if e != nil {
		core.Bad(w, "invalid path", 400)
		return
	}
	if e = os.MkdirAll(f, 0755); e != nil {
		core.Bad(w, e.Error(), 500)
		return
	}
	a.Log(u.ID, "mkdir", x.Path, u.IP)
	core.JSON(w, map[string]bool{"ok": true})
}

func (a *Module) Download(w http.ResponseWriter, r *http.Request) {
	f, e := core.Safe(a.Root, r.URL.Query().Get("path"))
	if e != nil {
		core.Bad(w, "invalid", 400)
		return
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(f)}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, f)
}

func (a *Module) Preview(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	f, e := core.Safe(a.Root, p)
	if e != nil {
		core.Bad(w, "invalid", 400)
		return
	}
	st, e := os.Stat(f)
	if e != nil || st.IsDir() {
		core.Bad(w, "not previewable", 400)
		return
	}
	ct := core.ImageContentType(filepath.Ext(f))
	if ct == "" {
		ct = mime.TypeByExtension(strings.ToLower(filepath.Ext(f)))
	}
	if strings.EqualFold(filepath.Ext(f), ".svgz") {
		w.Header().Set("Content-Encoding", "gzip")
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", "inline; filename=\""+strings.ReplaceAll(filepath.Base(f), "\"", "")+"\"")
	http.ServeFile(w, r, f)
}

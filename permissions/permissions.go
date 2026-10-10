package permissions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rajibdpi/trynas/core"
)

func (a *Module) Can(u core.User, p string, write bool) bool {
	p, err := core.CleanPath(p)
	if err != nil {
		return false
	}
	if u.Role == "admin" {
		return true
	}
	rows, err := a.DB.Query("SELECT path,access FROM permissions WHERE user_id=?", u.ID)
	if err != nil {
		return false
	}
	defer rows.Close()
	access := "none"
	longest := -1
	for rows.Next() {
		var base, level string
		if rows.Scan(&base, &level) != nil {
			return false
		}
		if core.Under(p, base) && len(base) > longest {
			longest = len(base)
			access = level
		}
	}
	return access == "write" || (!write && access == "read")
}

func (a *Module) CanBrowse(u core.User, p string) bool {
	if a.Can(u, p, false) {
		return true
	}
	p, err := core.CleanPath(p)
	if err != nil || p == "" {
		return false
	}
	rows, err := a.DB.Query("SELECT path,access FROM permissions WHERE user_id=?", u.ID)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var base, level string
		_ = rows.Scan(&base, &level)
		if level != "none" && core.Under(base, p) {
			return true
		}
	}
	return false
}

func (a *Module) AllowTree(u core.User, p string, write bool) error {
	if !a.Can(u, p, write) {
		return fmt.Errorf("folder permission denied")
	}
	full, err := core.Safe(a.Root, p)
	if err != nil {
		return err
	}
	return filepath.Walk(full, func(f string, st os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(a.Root, f)
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not supported")
		}
		if !a.Can(u, filepath.ToSlash(rel), write) {
			return fmt.Errorf("nested folder permission denied")
		}
		return nil
	})
}

func (a *Module) Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := r.Context().Value(core.UserKey).(core.User)
		p := r.URL.Path
		if strings.HasPrefix(p, "/api/users") || p == "/api/permissions" || p == "/api/system" || p == "/api/network" || p == "/api/activity" || p == "/api/settings" || strings.HasPrefix(p, "/api/storage/") {
			if u.Role != "admin" {
				core.Bad(w, "administrator access required", 403)
				return
			}
		}
		if r.Method == "POST" || r.Method == "DELETE" {
			a.Mutation.Lock()
			defer a.Mutation.Unlock()
		}
		if r.Method == "GET" && strings.HasPrefix(p, "/api/files") {
			target := r.URL.Query().Get("path")
			_, err := core.Safe(a.Root, target)
			ok := a.Can(u, target, false)
			if p == "/api/files" || p == "/api/files/search" {
				ok = a.CanBrowse(u, target)
			}
			if err != nil {
				core.Bad(w, err.Error(), 400)
				return
			}
			if p == "/api/files/download" {
				full, _ := core.Safe(a.Root, target)
				st, e := os.Stat(full)
				if e != nil || st.IsDir() {
					core.Bad(w, "select a file to download", 400)
					return
				}
			}
			if !ok {
				core.Bad(w, "folder permission denied", 403)
				return
			}
		}
		if r.Method == "POST" && (p == "/api/files/folder" || p == "/api/favorites") {
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
			if err != nil {
				core.Bad(w, "request too large", 413)
				return
			}
			var x struct{ Path string }
			if json.Unmarshal(body, &x) != nil {
				core.Bad(w, "invalid request", 400)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			target, err := core.CleanPath(x.Path)
			if err != nil {
				core.Bad(w, err.Error(), 400)
				return
			}
			if !a.Can(u, target, p == "/api/files/folder") {
				core.Bad(w, "folder permission denied", 403)
				return
			}
			if p == "/api/shares" {
				f, err := core.Safe(a.Root, target)
				if err != nil {
					core.Bad(w, "invalid path", 400)
					return
				}
				st, err := os.Stat(f)
				if err != nil || (!st.IsDir() && !st.Mode().IsRegular()) {
					core.Bad(w, "select a file or folder to share", 400)
					return
				}
			}
			if p == "/api/files/folder" {
				f, err := core.Safe(a.Root, target)
				if err != nil {
					core.Bad(w, err.Error(), 400)
					return
				}
				if _, err = os.Stat(f); err == nil {
					core.Bad(w, "destination exists", 409)
					return
				}
			}
			// Canonicalize saved favorites, shares and folder audit paths.
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			payload["path"] = target
			body, _ = json.Marshal(payload)
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		next.ServeHTTP(w, r)
	})
}

func (a *Module) Permissions(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query("SELECT p.user_id,u.username,p.path,p.access FROM permissions p JOIN users u ON u.id=p.user_id ORDER BY u.username,p.path")
	if err != nil {
		core.Bad(w, "permissions unavailable", 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int
		var name, p, access string
		_ = rows.Scan(&id, &name, &p, &access)
		out = append(out, map[string]any{"userId": id, "username": name, "path": p, "access": access})
	}
	core.JSON(w, out)
}

func (a *Module) SavePermission(w http.ResponseWriter, r *http.Request) {
	var x struct {
		UserID       int `json:"userId"`
		Path, Access string
	}
	if !core.Decode(w, r, &x) {
		return
	}
	p, err := core.CleanPath(x.Path)
	if err != nil {
		core.Bad(w, err.Error(), 400)
		return
	}
	if x.Access != "none" && x.Access != "read" && x.Access != "write" && x.Access != "inherit" {
		core.Bad(w, "invalid access level", 400)
		return
	}
	var role string
	if a.DB.QueryRow("SELECT role FROM users WHERE id=?", x.UserID).Scan(&role) != nil {
		core.Bad(w, "user not found", 404)
		return
	}
	if role == "admin" {
		core.Bad(w, "administrators have full access", 400)
		return
	}
	if x.Access == "inherit" {
		_, err = a.DB.Exec("DELETE FROM permissions WHERE user_id=? AND path=?", x.UserID, p)
	} else {
		f, e := core.Safe(a.Root, p)
		if e != nil {
			core.Bad(w, e.Error(), 400)
			return
		}
		st, e := os.Stat(f)
		if e != nil || !st.IsDir() {
			core.Bad(w, "select an existing folder", 400)
			return
		}
		_, err = a.DB.Exec("INSERT INTO permissions(user_id,path,access) VALUES(?,?,?) ON CONFLICT(user_id,path) DO UPDATE SET access=excluded.access", x.UserID, p, x.Access)
	}
	if err != nil {
		core.Bad(w, "permission update failed", 500)
		return
	}
	a.Log(r.Context().Value(core.UserKey).(core.User).ID, "permission", fmt.Sprintf("user=%d folder=%s access=%s", x.UserID, p, x.Access), core.ClientIP(r))
	core.JSON(w, map[string]bool{"ok": true})
}

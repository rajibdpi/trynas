package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

func cleanPath(p string) (string, error) {
	if strings.ContainsAny(p, "\\\x00:") || strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("use a relative storage path")
	}
	for _, part := range strings.Split(p, "/") {
		if runtime.GOOS == "windows" && part != "" && part != "." {
			base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
			reserved := base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9')
			if strings.ContainsAny(part, "<>\"|?*") || strings.TrimRight(part, ". ") != part || reserved {
				return "", fmt.Errorf("invalid Windows file name")
			}
		}
		if part == ".." || strings.HasPrefix(part, ".trynas-") {
			return "", fmt.Errorf("reserved or invalid path")
		}
	}
	p = path.Clean(p)
	if p == "." {
		p = ""
	}
	return p, nil
}
func under(p, base string) bool { return p == base || base == "" || strings.HasPrefix(p, base+"/") }
func (a *App) migrateV5() error {
	_, err := a.db.Exec(`CREATE TABLE IF NOT EXISTS user_homes(user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,path TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS user_limits(user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,quota_bytes INTEGER NOT NULL DEFAULT 0 CHECK(quota_bytes>=0));
 CREATE TABLE IF NOT EXISTS permissions(user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,path TEXT NOT NULL,access TEXT NOT NULL CHECK(access IN ('none','read','write')),PRIMARY KEY(user_id,path));
 CREATE TABLE IF NOT EXISTS file_owners(path TEXT PRIMARY KEY,user_id INTEGER NOT NULL REFERENCES users(id),bytes INTEGER NOT NULL CHECK(bytes>=0));
 CREATE TABLE IF NOT EXISTS app_settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS share_items(share_id INTEGER NOT NULL REFERENCES shares(id) ON DELETE CASCADE,path TEXT NOT NULL,PRIMARY KEY(share_id,path));
 CREATE TABLE IF NOT EXISTS share_passwords(share_id INTEGER PRIMARY KEY REFERENCES shares(id) ON DELETE CASCADE,password_hash TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS share_sessions(token TEXT PRIMARY KEY,share_id INTEGER NOT NULL REFERENCES shares(id) ON DELETE CASCADE,expires_at TEXT NOT NULL);`)
	if err != nil {
		return err
	}
	return a.normalizeShareExpiries()
}
func (a *App) can(u User, p string, write bool) bool {
	p, err := cleanPath(p)
	if err != nil {
		return false
	}
	if u.Role == "admin" {
		return true
	}
	rows, err := a.db.Query("SELECT path,access FROM permissions WHERE user_id=?", u.ID)
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
		if under(p, base) && len(base) > longest {
			longest = len(base)
			access = level
		}
	}
	return access == "write" || (!write && access == "read")
}
func (a *App) canBrowse(u User, p string) bool {
	if a.can(u, p, false) {
		return true
	}
	p, err := cleanPath(p)
	if err != nil || p == "" {
		return false
	}
	rows, err := a.db.Query("SELECT path,access FROM permissions WHERE user_id=?", u.ID)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var base, level string
		_ = rows.Scan(&base, &level)
		if level != "none" && under(base, p) {
			return true
		}
	}
	return false
}
func (a *App) allowTree(u User, p string, write bool) error {
	if !a.can(u, p, write) {
		return fmt.Errorf("folder permission denied")
	}
	full, err := safe(a.root, p)
	if err != nil {
		return err
	}
	return filepath.Walk(full, func(f string, st os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(a.root, f)
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not supported")
		}
		if !a.can(u, filepath.ToSlash(rel), write) {
			return fmt.Errorf("nested folder permission denied")
		}
		return nil
	})
}
func (a *App) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := r.Context().Value(userKey).(User)
		p := r.URL.Path
		if strings.HasPrefix(p, "/api/users") || p == "/api/permissions" || p == "/api/hardware" || p == "/api/system" || p == "/api/network" || p == "/api/activity" || p == "/api/settings" {
			if u.Role != "admin" {
				bad(w, "administrator access required", 403)
				return
			}
		}
		if r.Method == "POST" || r.Method == "DELETE" {
			a.mutation.Lock()
			defer a.mutation.Unlock()
		}
		if r.Method == "GET" && strings.HasPrefix(p, "/api/files") {
			target := r.URL.Query().Get("path")
			_, err := safe(a.root, target)
			ok := a.can(u, target, false)
			if p == "/api/files" || p == "/api/files/search" {
				ok = a.canBrowse(u, target)
			}
			if err != nil {
				bad(w, err.Error(), 400)
				return
			}
			if p == "/api/files/download" {
				full, _ := safe(a.root, target)
				st, e := os.Stat(full)
				if e != nil || st.IsDir() {
					bad(w, "select a file to download", 400)
					return
				}
			}
			if !ok {
				bad(w, "folder permission denied", 403)
				return
			}
		}
		if r.Method == "POST" && (p == "/api/files/folder" || p == "/api/favorites") {
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
			if err != nil {
				bad(w, "request too large", 413)
				return
			}
			var x struct{ Path string }
			if json.Unmarshal(body, &x) != nil {
				bad(w, "invalid request", 400)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			target, err := cleanPath(x.Path)
			if err != nil {
				bad(w, err.Error(), 400)
				return
			}
			if !a.can(u, target, p == "/api/files/folder") {
				bad(w, "folder permission denied", 403)
				return
			}
			if p == "/api/shares" {
				f, err := safe(a.root, target)
				if err != nil {
					bad(w, "invalid path", 400)
					return
				}
				st, err := os.Stat(f)
				if err != nil || (!st.IsDir() && !st.Mode().IsRegular()) {
					bad(w, "select a file or folder to share", 400)
					return
				}
			}
			if p == "/api/files/folder" {
				f, err := safe(a.root, target)
				if err != nil {
					bad(w, err.Error(), 400)
					return
				}
				if _, err = os.Stat(f); err == nil {
					bad(w, "destination exists", 409)
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
func decode(w http.ResponseWriter, r *http.Request, x any) bool {
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(x) != nil {
		bad(w, "invalid request", 400)
		return false
	}
	return true
}
func (a *App) usage(uid int) (int64, int64, error) {
	var used, quota int64
	err := a.db.QueryRow("SELECT COALESCE(SUM(bytes),0) FROM file_owners WHERE user_id=?", uid).Scan(&used)
	if err != nil {
		return 0, 0, err
	}
	err = a.db.QueryRow("SELECT COALESCE((SELECT quota_bytes FROM user_limits WHERE user_id=?),0)", uid).Scan(&quota)
	return used, quota, err
}
func (a *App) checkQuota(uid int, extra int64) error {
	used, limit, err := a.usage(uid)
	if err != nil {
		return err
	}
	if limit > 0 && (extra > limit-used) {
		return fmt.Errorf("storage quota exceeded")
	}
	return nil
}
func (a *App) quota(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	used, limit, err := a.usage(u.ID)
	if err != nil {
		bad(w, "quota unavailable", 500)
		return
	}
	jsonOut(w, map[string]any{"used": used, "limit": limit, "unlimited": limit == 0})
}
func (a *App) users(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`SELECT u.id,u.username,u.role,COALESCE(l.quota_bytes,0),COALESCE((SELECT SUM(bytes) FROM file_owners WHERE user_id=u.id),0) FROM users u LEFT JOIN user_limits l ON l.user_id=u.id ORDER BY u.id`)
	if err != nil {
		bad(w, "users unavailable", 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var u User
		var quota, used int64
		if rows.Scan(&u.ID, &u.Username, &u.Role, &quota, &used) != nil {
			bad(w, "users unavailable", 500)
			return
		}
		out = append(out, map[string]any{"id": u.ID, "username": u.Username, "role": u.Role, "quota": quota, "used": used})
	}
	jsonOut(w, out)
}
func (a *App) saveUser(w http.ResponseWriter, r *http.Request) {
	var x struct {
		ID                       int
		Username, Password, Role string
		Quota                    int64
	}
	if !decode(w, r, &x) {
		return
	}
	creating := x.ID == 0
	x.Username = strings.TrimSpace(x.Username)
	if x.Username == "" || len(x.Username) > 64 || (x.Role != "admin" && x.Role != "user") || x.Quota < 0 || (x.ID == 0 && x.Password == "") || (x.Password != "" && (len(x.Password) < 8 || len(x.Password) > 72)) {
		bad(w, "valid username, role, quota and an 8–72 byte password required", 400)
		return
	}
	var home string
	if creating {
		name, e := cleanPath(x.Username)
		if e != nil || name == "" || name != x.Username || strings.Contains(name, "/") {
			bad(w, "username must be a valid single folder name", 400)
			return
		}
		home, e = safe(a.root, name)
		if e != nil {
			bad(w, "invalid home folder name", 400)
			return
		}
		if _, e = os.Lstat(home); e == nil {
			bad(w, "a storage item already exists with this username; choose another username", 409)
			return
		} else if !os.IsNotExist(e) {
			bad(w, "cannot check home folder", 500)
			return
		}
	}
	if x.ID > 0 {
		var old string
		if a.db.QueryRow("SELECT role FROM users WHERE id=?", x.ID).Scan(&old) != nil {
			bad(w, "user not found", 404)
			return
		}
		var admins int
		_ = a.db.QueryRow("SELECT COUNT(*) FROM users WHERE role='admin'").Scan(&admins)
		if old == "admin" && x.Role != "admin" && admins <= 1 {
			bad(w, "keep at least one administrator", 409)
			return
		}
		used, _, err := a.usage(x.ID)
		if err != nil {
			bad(w, "quota unavailable", 500)
			return
		}
		if x.Quota > 0 && x.Quota < used {
			bad(w, "quota cannot be below current usage", 409)
			return
		}
	}
	tx, err := a.db.Begin()
	if err != nil {
		bad(w, "database unavailable", 500)
		return
	}
	defer tx.Rollback()
	var hash []byte
	if x.Password != "" {
		hash, err = bcrypt.GenerateFromPassword([]byte(x.Password), 12)
		if err != nil {
			bad(w, "invalid password", 400)
			return
		}
	}
	if x.ID == 0 {
		res, e := tx.Exec("INSERT INTO users(username,password_hash,role) VALUES(?,?,?)", x.Username, string(hash), x.Role)
		err = e
		if err == nil {
			id, _ := res.LastInsertId()
			x.ID = int(id)
		}
	} else {
		_, err = tx.Exec("UPDATE users SET username=?,role=? WHERE id=?", x.Username, x.Role, x.ID)
		if err == nil && len(hash) > 0 {
			_, err = tx.Exec("UPDATE users SET password_hash=? WHERE id=?", string(hash), x.ID)
		}
	}
	if err != nil {
		bad(w, "username already exists or update failed", 409)
		return
	}
	if _, err = tx.Exec("INSERT INTO user_limits(user_id,quota_bytes) VALUES(?,?) ON CONFLICT(user_id) DO UPDATE SET quota_bytes=excluded.quota_bytes", x.ID, x.Quota); err != nil {
		bad(w, "quota update failed", 500)
		return
	}
	if len(hash) > 0 {
		if _, err = tx.Exec("DELETE FROM sessions WHERE user_id=?", x.ID); err != nil {
			bad(w, "session update failed", 500)
			return
		}
	}
	homeCreated := false
	committed := false
	defer func() {
		if homeCreated && !committed {
			_ = os.Remove(home)
		}
	}()
	if creating {
		if err = os.Mkdir(home, 0750); err != nil {
			bad(w, "could not create user home folder", 500)
			return
		}
		homeCreated = true
		if _, err = tx.Exec("INSERT INTO user_homes(user_id,path) VALUES(?,?)", x.ID, x.Username); err != nil {
			bad(w, "could not save home folder", 500)
			return
		}
		if _, err = tx.Exec("INSERT INTO permissions(user_id,path,access) VALUES(?,?,'write')", x.ID, x.Username); err != nil {
			bad(w, "could not grant home folder access", 500)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		bad(w, "user update failed", 500)
		return
	}
	committed = true
	a.log(r.Context().Value(userKey).(User).ID, "save-user", x.Username)
	jsonOut(w, map[string]any{"id": x.ID})
}
func (a *App) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	u := r.Context().Value(userKey).(User)
	if err != nil || id == u.ID {
		bad(w, "cannot delete your own account", 400)
		return
	}
	var role string
	if a.db.QueryRow("SELECT role FROM users WHERE id=?", id).Scan(&role) != nil {
		bad(w, "user not found", 404)
		return
	}
	var admins int
	_ = a.db.QueryRow("SELECT COUNT(*) FROM users WHERE role='admin'").Scan(&admins)
	if role == "admin" && admins <= 1 {
		bad(w, "keep at least one administrator", 409)
		return
	}
	var n int
	_ = a.db.QueryRow("SELECT COUNT(*) FROM file_owners WHERE user_id=?", id).Scan(&n)
	if n > 0 {
		bad(w, "remove owned files, including trash, before deleting this user", 409)
		return
	}
	if _, err = a.db.Exec("DELETE FROM users WHERE id=?", id); err != nil {
		bad(w, "user deletion failed", 500)
		return
	}
	a.log(u.ID, "delete-user", strconv.Itoa(id))
	jsonOut(w, map[string]bool{"ok": true})
}
func (a *App) permissions(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query("SELECT p.user_id,u.username,p.path,p.access FROM permissions p JOIN users u ON u.id=p.user_id ORDER BY u.username,p.path")
	if err != nil {
		bad(w, "permissions unavailable", 500)
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
	jsonOut(w, out)
}
func (a *App) savePermission(w http.ResponseWriter, r *http.Request) {
	var x struct {
		UserID       int `json:"userId"`
		Path, Access string
	}
	if !decode(w, r, &x) {
		return
	}
	p, err := cleanPath(x.Path)
	if err != nil {
		bad(w, err.Error(), 400)
		return
	}
	if x.Access != "none" && x.Access != "read" && x.Access != "write" && x.Access != "inherit" {
		bad(w, "invalid access level", 400)
		return
	}
	var role string
	if a.db.QueryRow("SELECT role FROM users WHERE id=?", x.UserID).Scan(&role) != nil {
		bad(w, "user not found", 404)
		return
	}
	if role == "admin" {
		bad(w, "administrators have full access", 400)
		return
	}
	if x.Access == "inherit" {
		_, err = a.db.Exec("DELETE FROM permissions WHERE user_id=? AND path=?", x.UserID, p)
	} else {
		f, e := safe(a.root, p)
		if e != nil {
			bad(w, e.Error(), 400)
			return
		}
		st, e := os.Stat(f)
		if e != nil || !st.IsDir() {
			bad(w, "select an existing folder", 400)
			return
		}
		_, err = a.db.Exec("INSERT INTO permissions(user_id,path,access) VALUES(?,?,?) ON CONFLICT(user_id,path) DO UPDATE SET access=excluded.access", x.UserID, p, x.Access)
	}
	if err != nil {
		bad(w, "permission update failed", 500)
		return
	}
	a.log(r.Context().Value(userKey).(User).ID, "permission", fmt.Sprintf("user=%d folder=%s access=%s", x.UserID, p, x.Access))
	jsonOut(w, map[string]bool{"ok": true})
}

func (a *App) profile(u User) map[string]any {
	home := ""
	if u.Role != "admin" {
		_ = a.db.QueryRow("SELECT path FROM user_homes WHERE user_id=?", u.ID).Scan(&home)
		if home == "" {
			_ = a.db.QueryRow("SELECT path FROM permissions WHERE user_id=? AND access IN ('read','write') AND path<>'' ORDER BY CASE WHEN path=? THEN 0 ELSE 1 END,length(path),path LIMIT 1", u.ID, u.Username).Scan(&home)
		}
	}
	return map[string]any{"id": u.ID, "username": u.Username, "role": u.Role, "home": home, "rootAccessible": a.can(u, "", false)}
}

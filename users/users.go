package users

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	chi "github.com/go-chi/chi/v5"
	"github.com/rajibdpi/trynas/core"
	"golang.org/x/crypto/bcrypt"
)

func (a *Module) Usage(uid int) (int64, int64, error) {
	var used, quota int64
	err := a.DB.QueryRow("SELECT COALESCE(SUM(bytes),0) FROM file_owners WHERE user_id=?", uid).Scan(&used)
	if err != nil {
		return 0, 0, err
	}
	err = a.DB.QueryRow("SELECT COALESCE((SELECT quota_bytes FROM user_limits WHERE user_id=?),0)", uid).Scan(&quota)
	return used, quota, err
}

func (a *Module) CheckQuota(uid int, extra int64) error {
	used, limit, err := a.Usage(uid)
	if err != nil {
		return err
	}
	if limit > 0 && (extra > limit-used) {
		return fmt.Errorf("storage quota exceeded")
	}
	return nil
}

func (a *Module) Quota(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	used, limit, err := a.Usage(u.ID)
	if err != nil {
		core.Bad(w, "quota unavailable", 500)
		return
	}
	core.JSON(w, map[string]any{"used": used, "limit": limit, "unlimited": limit == 0})
}

func (a *Module) Users(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(`SELECT u.id,u.username,u.role,COALESCE(l.quota_bytes,0),COALESCE((SELECT SUM(bytes) FROM file_owners WHERE user_id=u.id),0) FROM users u LEFT JOIN user_limits l ON l.user_id=u.id ORDER BY u.id`)
	if err != nil {
		core.Bad(w, "users unavailable", 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var u core.User
		var quota, used int64
		if rows.Scan(&u.ID, &u.Username, &u.Role, &quota, &used) != nil {
			core.Bad(w, "users unavailable", 500)
			return
		}
		out = append(out, map[string]any{"id": u.ID, "username": u.Username, "role": u.Role, "quota": quota, "used": used})
	}
	core.JSON(w, out)
}

func (a *Module) SaveUser(w http.ResponseWriter, r *http.Request) {
	var x struct {
		ID                       int
		Username, Password, Role string
		Quota                    int64
	}
	if !core.Decode(w, r, &x) {
		return
	}
	creating := x.ID == 0
	x.Username = strings.TrimSpace(x.Username)
	if x.Username == "" || len(x.Username) > 64 || (x.Role != "admin" && x.Role != "user") || x.Quota < 0 || (x.ID == 0 && x.Password == "") || (x.Password != "" && (len(x.Password) < 8 || len(x.Password) > 72)) {
		core.Bad(w, "valid username, role, quota and an 8–72 byte password required", 400)
		return
	}
	var home string
	if creating {
		name, e := core.CleanPath(x.Username)
		if e != nil || name == "" || name != x.Username || strings.Contains(name, "/") {
			core.Bad(w, "username must be a valid single folder name", 400)
			return
		}
		home, e = core.Safe(a.Root, name)
		if e != nil {
			core.Bad(w, "invalid home folder name", 400)
			return
		}
		if _, e = os.Lstat(home); e == nil {
			core.Bad(w, "a storage item already exists with this username; choose another username", 409)
			return
		} else if !os.IsNotExist(e) {
			core.Bad(w, "cannot check home folder", 500)
			return
		}
	}
	if x.ID > 0 {
		var old string
		if a.DB.QueryRow("SELECT role FROM users WHERE id=?", x.ID).Scan(&old) != nil {
			core.Bad(w, "user not found", 404)
			return
		}
		var admins int
		_ = a.DB.QueryRow("SELECT COUNT(*) FROM users WHERE role='admin'").Scan(&admins)
		if old == "admin" && x.Role != "admin" && admins <= 1 {
			core.Bad(w, "keep at least one administrator", 409)
			return
		}
		used, _, err := a.Usage(x.ID)
		if err != nil {
			core.Bad(w, "quota unavailable", 500)
			return
		}
		if x.Quota > 0 && x.Quota < used {
			core.Bad(w, "quota cannot be below current usage", 409)
			return
		}
	}
	tx, err := a.DB.Begin()
	if err != nil {
		core.Bad(w, "database unavailable", 500)
		return
	}
	defer tx.Rollback()
	var hash []byte
	if x.Password != "" {
		hash, err = bcrypt.GenerateFromPassword([]byte(x.Password), 12)
		if err != nil {
			core.Bad(w, "invalid password", 400)
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
		core.Bad(w, "username already exists or update failed", 409)
		return
	}
	if _, err = tx.Exec("INSERT INTO user_limits(user_id,quota_bytes) VALUES(?,?) ON CONFLICT(user_id) DO UPDATE SET quota_bytes=excluded.quota_bytes", x.ID, x.Quota); err != nil {
		core.Bad(w, "quota update failed", 500)
		return
	}
	if len(hash) > 0 {
		if _, err = tx.Exec("DELETE FROM sessions WHERE user_id=?", x.ID); err != nil {
			core.Bad(w, "session update failed", 500)
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
			core.Bad(w, "could not create user home folder", 500)
			return
		}
		homeCreated = true
		if _, err = tx.Exec("INSERT INTO user_homes(user_id,path) VALUES(?,?)", x.ID, x.Username); err != nil {
			core.Bad(w, "could not save home folder", 500)
			return
		}
		if _, err = tx.Exec("INSERT INTO permissions(user_id,path,access) VALUES(?,?,'write')", x.ID, x.Username); err != nil {
			core.Bad(w, "could not grant home folder access", 500)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		core.Bad(w, "user update failed", 500)
		return
	}
	committed = true
	a.Log(r.Context().Value(core.UserKey).(core.User).ID, "save-user", x.Username)
	core.JSON(w, map[string]any{"id": x.ID})
}

func (a *Module) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	u := r.Context().Value(core.UserKey).(core.User)
	if err != nil || id == u.ID {
		core.Bad(w, "cannot delete your own account", 400)
		return
	}
	var role string
	if a.DB.QueryRow("SELECT role FROM users WHERE id=?", id).Scan(&role) != nil {
		core.Bad(w, "user not found", 404)
		return
	}
	var admins int
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM users WHERE role='admin'").Scan(&admins)
	if role == "admin" && admins <= 1 {
		core.Bad(w, "keep at least one administrator", 409)
		return
	}
	var n int
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM file_owners WHERE user_id=?", id).Scan(&n)
	if n > 0 {
		core.Bad(w, "remove owned files, including trash, before deleting this user", 409)
		return
	}
	if _, err = a.DB.Exec("DELETE FROM users WHERE id=?", id); err != nil {
		core.Bad(w, "user deletion failed", 500)
		return
	}
	a.Log(u.ID, "delete-user", strconv.Itoa(id))
	core.JSON(w, map[string]bool{"ok": true})
}

func (a *Module) Profile(u core.User) map[string]any {
	home := ""
	if u.Role != "admin" {
		_ = a.DB.QueryRow("SELECT path FROM user_homes WHERE user_id=?", u.ID).Scan(&home)
		if home == "" {
			_ = a.DB.QueryRow("SELECT path FROM permissions WHERE user_id=? AND access IN ('read','write') AND path<>'' ORDER BY CASE WHEN path=? THEN 0 ELSE 1 END,length(path),path LIMIT 1", u.ID, u.Username).Scan(&home)
		}
	}
	return map[string]any{"id": u.ID, "username": u.Username, "role": u.Role, "home": home, "rootAccessible": a.Can(u, "", false)}
}

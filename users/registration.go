package users

import (
	"database/sql"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rajibdpi/trynas/core"
)

func registrationAdmin(w http.ResponseWriter, r *http.Request) bool {
	user, ok := r.Context().Value(core.UserKey).(core.User)
	if !ok || user.Role != "admin" {
		core.Bad(w, "administrator required", 403)
		return false
	}
	return true
}

func (a *Module) RegistrationRequests(w http.ResponseWriter, r *http.Request) {
	if !registrationAdmin(w, r) {
		return
	}
	rows, err := a.DB.Query("SELECT id,username,created_at FROM registration_requests ORDER BY id")
	if err != nil {
		core.Bad(w, "requests unavailable", 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int
		var name, at string
		if err := rows.Scan(&id, &name, &at); err != nil {
			core.Bad(w, "requests unavailable", 500)
			return
		}
		out = append(out, map[string]any{"id": id, "username": name, "createdAt": at})
	}
	if rows.Err() != nil {
		core.Bad(w, "requests unavailable", 500)
		return
	}
	core.JSON(w, out)
}

func (a *Module) ApproveRegistration(w http.ResponseWriter, r *http.Request) {
	if !registrationAdmin(w, r) {
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		core.Bad(w, "invalid request ID", 400)
		return
	}
	var input struct{ Quota int64 }
	if !core.Decode(w, r, &input) {
		return
	}
	if input.Quota < 0 {
		core.Bad(w, "invalid quota", 400)
		return
	}
	tx, err := a.DB.Begin()
	if err != nil {
		core.Bad(w, "database unavailable", 500)
		return
	}
	defer tx.Rollback()
	var name, hash string
	if err = tx.QueryRow("SELECT username,password_hash FROM registration_requests WHERE id=?", id).Scan(&name, &hash); err != nil {
		if err == sql.ErrNoRows {
			core.Bad(w, "request not found", 404)
		} else {
			core.Bad(w, "request unavailable", 500)
		}
		return
	}
	clean, err := core.CleanPath(name)
	if err != nil || clean == "" || clean != name || strings.Contains(name, "/") {
		core.Bad(w, "invalid home folder name", 400)
		return
	}
	home, err := core.Safe(a.Root, name)
	if err != nil {
		core.Bad(w, "invalid home folder name", 400)
		return
	}
	if _, err = os.Lstat(home); err == nil {
		core.Bad(w, "storage item already exists with this username", 409)
		return
	} else if !os.IsNotExist(err) {
		core.Bad(w, "cannot check home folder", 500)
		return
	}
	result, err := tx.Exec("INSERT INTO users(username,password_hash,role) VALUES(?,?,'user')", name, hash)
	if err != nil {
		core.Bad(w, "username unavailable", 409)
		return
	}
	uid, err := result.LastInsertId()
	if err != nil {
		core.Bad(w, "account creation failed", 500)
		return
	}
	if err = os.Mkdir(home, 0750); err != nil {
		core.Bad(w, "could not create user home folder", 500)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(home)
		}
	}()
	for _, query := range []string{"INSERT INTO user_homes(user_id,path) VALUES(?,?)", "INSERT INTO permissions(user_id,path,access) VALUES(?,?,'write')"} {
		if _, err = tx.Exec(query, uid, name); err != nil {
			core.Bad(w, "home configuration failed", 500)
			return
		}
	}
	if _, err = tx.Exec("INSERT INTO user_limits(user_id,quota_bytes) VALUES(?,?)", uid, input.Quota); err != nil {
		core.Bad(w, "quota configuration failed", 500)
		return
	}
	if _, err = tx.Exec("DELETE FROM registration_requests WHERE id=?", id); err != nil {
		core.Bad(w, "approval failed", 500)
		return
	}
	if err = tx.Commit(); err != nil {
		core.Bad(w, "approval failed", 500)
		return
	}
	committed = true
	a.Log(r.Context().Value(core.UserKey).(core.User).ID, "approve-user", name)
	core.JSON(w, map[string]any{"id": uid})
}

func (a *Module) RejectRegistration(w http.ResponseWriter, r *http.Request) {
	if !registrationAdmin(w, r) {
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		core.Bad(w, "invalid request ID", 400)
		return
	}
	result, err := a.DB.Exec("DELETE FROM registration_requests WHERE id=?", id)
	if err != nil {
		core.Bad(w, "rejection failed", 500)
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		core.Bad(w, "rejection failed", 500)
		return
	}
	if n == 0 {
		core.Bad(w, "request not found", 404)
		return
	}
	a.Log(r.Context().Value(core.UserKey).(core.User).ID, "reject-registration", strconv.Itoa(id))
	core.JSON(w, map[string]bool{"ok": true})
}

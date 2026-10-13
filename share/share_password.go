package share

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	chi "github.com/go-chi/chi/v5"
	"github.com/rajibdpi/trynas/core"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strconv"
)

func (a *Module) RegenerateSharePassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	u := r.Context().Value(core.UserKey).(core.User)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		core.Bad(w, "invalid share", 400)
		return
	}
	tx, err := a.DB.Begin()
	if err != nil {
		core.Bad(w, "could not update password", 500)
		return
	}
	defer tx.Rollback()
	var exists int
	err = tx.QueryRow("SELECT 1 FROM shares s JOIN share_passwords p ON p.share_id=s.id WHERE s.id=? AND s.user_id=?", id, u.ID).Scan(&exists)
	if err == sql.ErrNoRows {
		core.Bad(w, "protected share not found", 404)
		return
	}
	if err != nil {
		core.Bad(w, "could not update password", 500)
		return
	}
	random := make([]byte, 18)
	if _, err = rand.Read(random); err != nil {
		core.Bad(w, "could not generate password", 500)
		return
	}
	password := base64.RawURLEncoding.EncodeToString(random)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		core.Bad(w, "could not protect password", 500)
		return
	}
	if _, err = tx.Exec("UPDATE share_passwords SET password_hash=? WHERE share_id=?", string(hash), id); err != nil {
		core.Bad(w, "could not update password", 500)
		return
	}
	if _, err = tx.Exec("DELETE FROM share_sessions WHERE share_id=?", id); err != nil {
		core.Bad(w, "could not update password", 500)
		return
	}
	if err = tx.Commit(); err != nil {
		core.Bad(w, "could not update password", 500)
		return
	}
	a.Log(u.ID, "share-password-regenerate", strconv.FormatInt(id, 10), u.IP)
	core.JSON(w, map[string]string{"password": password})
}

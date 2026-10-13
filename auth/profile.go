package auth

import (
	"encoding/json"
	"github.com/rajibdpi/trynas/core"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"
)

func (a *Module) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	var input struct{ Username, DisplayName, Email, CurrentPassword, NewPassword string }
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		core.Bad(w, "invalid profile", 400)
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = strings.TrimSpace(input.Email)
	cleaned, err := core.CleanPath(input.Username)
	if err != nil || cleaned != input.Username || cleaned == "" || strings.Contains(cleaned, "/") || len(cleaned) > 64 || strings.ContainsAny(cleaned, "\r\n\t") {
		core.Bad(w, "username must be a single valid name of at most 64 bytes", 400)
		return
	}
	if utf8.RuneCountInString(input.DisplayName) > 100 || strings.ContainsAny(input.DisplayName, "\r\n") {
		core.Bad(w, "display name must be at most 100 characters", 400)
		return
	}
	if input.Email != "" {
		address, err := mail.ParseAddress(input.Email)
		if err != nil || address.Address != input.Email || len(input.Email) > 254 {
			core.Bad(w, "enter a valid email address", 400)
			return
		}
	}
	if input.NewPassword != "" && (len(input.NewPassword) < 8 || len(input.NewPassword) > 72) {
		core.Bad(w, "password must be 8–72 bytes", 400)
		return
	}
	sensitive := input.Username != u.Username || input.NewPassword != ""
	if sensitive {
		if !a.AllowAttempt(w, r, "profile-password:"+u.Username) {
			return
		}
		var hash string
		if a.DB.QueryRow("SELECT password_hash FROM users WHERE id=?", u.ID).Scan(&hash) != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.CurrentPassword)) != nil {
			core.Bad(w, "current password is incorrect", 400)
			return
		}
	}
	tx, err := a.DB.Begin()
	if err != nil {
		core.Bad(w, "could not save profile", 500)
		return
	}
	defer tx.Rollback()
	var exists int
	if err = tx.QueryRow("SELECT COUNT(*) FROM users WHERE username=? AND id<>?", input.Username, u.ID).Scan(&exists); err != nil {
		core.Bad(w, "could not save profile", 500)
		return
	}
	if exists > 0 {
		core.Bad(w, "username is already in use", 409)
		return
	}
	if _, err = tx.Exec("UPDATE users SET username=? WHERE id=?", input.Username, u.ID); err != nil {
		core.Bad(w, "could not save profile", 500)
		return
	}
	if input.NewPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			core.Bad(w, "could not update password", 500)
			return
		}
		if _, err = tx.Exec("UPDATE users SET password_hash=? WHERE id=?", string(hash), u.ID); err != nil {
			core.Bad(w, "could not update password", 500)
			return
		}
		token := ""
		if cookie, err := r.Cookie("trynas_session"); err == nil {
			token = cookie.Value
		}
		if _, err = tx.Exec("DELETE FROM sessions WHERE user_id=? AND token<>?", u.ID, token); err != nil {
			core.Bad(w, "could not update sessions", 500)
			return
		}
	}
	if _, err = tx.Exec("INSERT INTO user_profiles(user_id,display_name,email) VALUES(?,?,?) ON CONFLICT(user_id) DO UPDATE SET display_name=excluded.display_name,email=excluded.email", u.ID, input.DisplayName, input.Email); err != nil {
		core.Bad(w, "could not save profile", 500)
		return
	}
	if err = tx.Commit(); err != nil {
		core.Bad(w, "could not save profile", 500)
		return
	}
	u.Username = input.Username
	a.Log(u.ID, "profile-update", "", u.IP)
	core.JSON(w, a.Profile(u))
}

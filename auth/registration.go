package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rajibdpi/trynas/core"
	"golang.org/x/crypto/bcrypt"
)

func (a *Module) RegistrationOptions(w http.ResponseWriter, r *http.Request) {
	settings, err := a.ReadSettings()
	if err != nil {
		core.Bad(w, "registration unavailable", 503)
		return
	}
	core.JSON(w, map[string]bool{"enabled": settings.RegistrationEnabled})
}

func (a *Module) Register(w http.ResponseWriter, r *http.Request) {
	var input struct{ Username, Password string }
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		core.Bad(w, "invalid registration", 400)
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	if !a.AllowAttempt(w, r, "register:"+strings.ToLower(input.Username)) {
		return
	}
	name, err := core.CleanPath(input.Username)
	if err != nil || name == "" || name != input.Username || strings.Contains(name, "/") || len(name) > 64 || len(input.Password) < 8 || len(input.Password) > 72 {
		core.Bad(w, "valid single-folder username and 8–72 byte password required", 400)
		return
	}
	// Share the write lock with admin approval, creation, and settings changes.
	a.Mutation.Lock()
	defer a.Mutation.Unlock()
	settings, err := a.ReadSettings()
	if err != nil {
		core.Bad(w, "registration unavailable", 503)
		return
	}
	if !settings.RegistrationEnabled {
		core.Bad(w, "self-registration is disabled", 403)
		return
	}
	var count int
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM users WHERE username=?", name).Scan(&count); err != nil {
		core.Bad(w, "registration unavailable", 503)
		return
	}
	if count > 0 {
		core.Bad(w, "username unavailable or already requested", 409)
		return
	}
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM registration_requests").Scan(&count); err != nil || count >= 1000 {
		core.Bad(w, "registration queue unavailable", 503)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		core.Bad(w, "invalid password", 400)
		return
	}
	if _, err := a.DB.Exec("INSERT INTO registration_requests(username,password_hash) VALUES(?,?)", name, string(hash)); err != nil {
		core.Bad(w, "username unavailable or already requested", 409)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	core.JSON(w, map[string]string{"status": "pending", "message": "Account requested. An administrator must approve it before you can sign in."})
}

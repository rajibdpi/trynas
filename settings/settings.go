package settings

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rajibdpi/trynas/core"
)

func (a *Module) ReadSettings() (core.AppSettings, error) {
	settings := core.AppSettings{AppName: "TryNAS", FileView: "grid", ShareExpiry: "7"}
	var data string
	err := a.DB.QueryRow("SELECT value FROM app_settings WHERE key='basic'").Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return settings, nil
		}
		return settings, err
	}
	err = json.Unmarshal([]byte(data), &settings)
	return settings, err
}

func (a *Module) Settings(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(core.UserKey).(core.User)
	if user.Role != "admin" && r.URL.Path != "/api/preferences" {
		core.Bad(w, "administrator required", 403)
		return
	}
	if r.Method == http.MethodPost {
		if user.Role != "admin" {
			core.Bad(w, "administrator required", 403)
			return
		}
		var settings core.AppSettings
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&settings); err != nil {
			core.Bad(w, "invalid settings", 400)
			return
		}
		settings.AppName = strings.TrimSpace(settings.AppName)
		if settings.AppName == "" || utf8.RuneCountInString(settings.AppName) > 40 || strings.ContainsAny(settings.AppName, "\r\n") {
			core.Bad(w, "app name must be 1–40 characters", 400)
			return
		}
		if settings.FileView != "grid" && settings.FileView != "list" {
			core.Bad(w, "choose list or grid view", 400)
			return
		}
		switch settings.ShareExpiry {
		case "never", "1", "7", "15", "30":
		default:
			core.Bad(w, "invalid default share expiry", 400)
			return
		}
		data, _ := json.Marshal(settings)
		if _, err := a.DB.Exec("INSERT INTO app_settings(key,value) VALUES('basic',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(data)); err != nil {
			core.Bad(w, "could not save settings", 500)
			return
		}
		a.Log(user.ID, "settings", "basic")
		core.JSON(w, settings)
		return
	}
	settings, err := a.ReadSettings()
	if err != nil {
		core.Bad(w, "could not load settings", 500)
		return
	}
	result := map[string]any{"settings": settings}
	if user.Role == "admin" && r.URL.Path != "/api/preferences" {
		result["server"] = map[string]any{"storageRoot": a.Root, "address": core.Env("ADDR", "127.0.0.1:8080"), "cookieSecure": core.Env("COOKIE_SECURE", "false") == "true", "corsOrigin": core.Env("CORS_ORIGIN", "http://localhost:3000"), "smartUseSudo": core.Env("SMART_USE_SUDO", "false") == "true"}
	}
	core.JSON(w, result)
}

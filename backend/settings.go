package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"
)

type AppSettings struct {
	AppName     string `json:"appName"`
	FileView    string `json:"fileView"`
	ShareExpiry string `json:"shareExpiry"`
}

func (a *App) readSettings() (AppSettings, error) {
	settings := AppSettings{"TryNAS", "grid", "7"}
	var data string
	err := a.db.QueryRow("SELECT value FROM app_settings WHERE key='basic'").Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return settings, nil
		}
		return settings, err
	}
	err = json.Unmarshal([]byte(data), &settings)
	return settings, err
}
func (a *App) settings(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userKey).(User)
	if r.Method == http.MethodPost {
		if user.Role != "admin" {
			bad(w, "administrator required", 403)
			return
		}
		var settings AppSettings
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&settings); err != nil {
			bad(w, "invalid settings", 400)
			return
		}
		settings.AppName = strings.TrimSpace(settings.AppName)
		if settings.AppName == "" || utf8.RuneCountInString(settings.AppName) > 40 || strings.ContainsAny(settings.AppName, "\r\n") {
			bad(w, "app name must be 1–40 characters", 400)
			return
		}
		if settings.FileView != "grid" && settings.FileView != "list" {
			bad(w, "choose list or grid view", 400)
			return
		}
		switch settings.ShareExpiry {
		case "never", "1", "7", "15", "30":
		default:
			bad(w, "invalid default share expiry", 400)
			return
		}
		data, _ := json.Marshal(settings)
		if _, err := a.db.Exec("INSERT INTO app_settings(key,value) VALUES('basic',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(data)); err != nil {
			bad(w, "could not save settings", 500)
			return
		}
		a.log(user.ID, "settings", "basic")
		jsonOut(w, settings)
		return
	}
	settings, err := a.readSettings()
	if err != nil {
		bad(w, "could not load settings", 500)
		return
	}
	result := map[string]any{"settings": settings}
	if user.Role == "admin" {
		result["server"] = map[string]any{"storageRoot": a.root, "address": env("ADDR", ":8080"), "cookieSecure": env("COOKIE_SECURE", "false") == "true", "corsOrigin": env("CORS_ORIGIN", "http://localhost:3000"), "smartUseSudo": env("SMART_USE_SUDO", "false") == "true"}
	}
	jsonOut(w, result)
}

package settings

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rajibdpi/trynas/core"
)

var themeColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (a *Module) ReadSettings() (core.AppSettings, error) {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if !a.cachedAt.IsZero() && time.Since(a.cachedAt) < 30*time.Second {
		return a.cached, nil
	}
	settings := core.AppSettings{TrashRetentionDays: 30, AppName: "TryNAS", FileView: "grid", ShareExpiry: "never", ThemeMode: "system", LightPrimary: "#be123c", DarkPrimary: "#fb7185", LightBackground: "#f4f6fa", DarkBackground: "#101522", LightCard: "#ffffff", DarkCard: "#181f2f"}
	var data string
	err := a.DB.QueryRow("SELECT value FROM app_settings WHERE key='basic'").Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			a.cached, a.cachedAt = settings, time.Now()
			return settings, nil
		}
		return settings, err
	}
	err = json.Unmarshal([]byte(data), &settings)
	if err == nil {
		a.cached, a.cachedAt = settings, time.Now()
	}
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
		if settings.TrashRetentionDays < 0 || settings.TrashRetentionDays > 3650 {
			core.Bad(w, "trash retention must be 0–3650 days", 400)
			return
		}
		settings.AppName = strings.TrimSpace(settings.AppName)
		settings.LogoURL = strings.TrimSpace(settings.LogoURL)
		settings.FooterText = strings.TrimSpace(settings.FooterText)
		if utf8.RuneCountInString(settings.FooterText) > 200 {
			core.Bad(w, "footer text must be at most 200 characters", 400)
			return
		}
		if settings.LogoURL != "" {
			logo, err := url.Parse(settings.LogoURL)
			if err != nil || len(settings.LogoURL) > 2048 || strings.ContainsAny(settings.LogoURL, "\r\n\\") || !((logo.Scheme == "https" || logo.Scheme == "http") && logo.Host != "" && logo.User == nil || logo.Scheme == "" && logo.Host == "" && strings.HasPrefix(settings.LogoURL, "/") && !strings.HasPrefix(settings.LogoURL, "//")) {
				core.Bad(w, "logo must be an HTTP(S) URL or a path starting with /", 400)
				return
			}
		}
		if settings.AppName == "" || utf8.RuneCountInString(settings.AppName) > 40 || strings.ContainsAny(settings.AppName, "\r\n") {
			core.Bad(w, "app name must be 1–40 characters", 400)
			return
		}
		if settings.FileView != "grid" && settings.FileView != "list" && settings.FileView != "large" {
			core.Bad(w, "choose large tiles, tiles, or list view", 400)
			return
		}
		switch settings.ShareExpiry {
		case "never", "1", "7", "15", "30":
		default:
			core.Bad(w, "invalid default share expiry", 400)
			return
		}
		if settings.ThemeMode != "" && settings.ThemeMode != "system" && settings.ThemeMode != "light" && settings.ThemeMode != "dark" {
			core.Bad(w, "choose system, light, or dark theme", 400)
			return
		}
		for _, color := range []string{settings.LightPrimary, settings.DarkPrimary, settings.LightBackground, settings.DarkBackground, settings.LightCard, settings.DarkCard} {
			if color != "" && !themeColorPattern.MatchString(color) {
				core.Bad(w, "theme colors must use #RRGGBB format", 400)
				return
			}
		}
		data, _ := json.Marshal(settings)
		a.cacheMu.Lock()
		if _, err := a.DB.Exec("INSERT INTO app_settings(key,value) VALUES('basic',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(data)); err != nil {
			a.cacheMu.Unlock()
			core.Bad(w, "could not save settings", 500)
			return
		}
		a.cached, a.cachedAt = settings, time.Now()
		a.cacheMu.Unlock()
		a.Log(user.ID, "settings", "basic", user.IP)
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
		result["server"] = map[string]any{"proxyUploadLimitMB": proxyUploadLimitMB(), "storageRoot": a.Root, "address": core.Env("ADDR", "127.0.0.1:8080"), "cookieSecure": core.Env("COOKIE_SECURE", "false") == "true", "corsOrigin": core.Env("CORS_ORIGIN", "http://localhost:3000")}
	}
	core.JSON(w, result)
}

// Theme exposes only appearance values so signed-out pages can use saved colors.
func (a *Module) Theme(w http.ResponseWriter, r *http.Request) {
	settings, err := a.ReadSettings()
	if err != nil {
		core.Bad(w, "could not load theme", 500)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	core.JSON(w, map[string]string{
		"appName": settings.AppName, "logoUrl": settings.LogoURL,
		"themeMode":    settings.ThemeMode,
		"lightPrimary": settings.LightPrimary, "darkPrimary": settings.DarkPrimary,
		"lightBackground": settings.LightBackground, "darkBackground": settings.DarkBackground,
		"lightCard": settings.LightCard, "darkCard": settings.DarkCard,
	})
}

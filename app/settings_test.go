package app

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rajibdpi/trynas/core"
)

func TestBasicAppSettings(t *testing.T) {
	a, admin, user := setup(t)
	w := request(a, admin, "GET", "/api/settings", nil, a.Settings)
	expect(t, w, 200)
	var response struct{ Settings core.AppSettings }
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Settings.ShowHiddenFiles || response.Settings.AppName != "TryNAS" || response.Settings.FileView != "grid" || response.Settings.ShareExpiry != "never" {
		t.Fatal("incorrect defaults")
	}
	configured := core.AppSettings{ShowHiddenFiles: true, AppName: "Home NAS", LogoURL: "https://example.com/logo.png", FooterText: "© My home", ThemeMode: "dark", LightPrimary: "#0369a1", DarkPrimary: "#38bdf8", LightBackground: "#f4f6fa", DarkBackground: "#101522", LightCard: "#ffffff", DarkCard: "#181f2f", FileView: "list", ShareExpiry: "15"}
	expect(t, jsonRequest(a, user, "POST", "/api/settings", configured, a.Settings), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/settings", configured, a.Settings), 200)
	saved, err := a.ReadSettings()
	if err != nil || saved != configured {
		t.Fatal("settings not persisted", err)
	}
	expect(t, request(a, user, "GET", "/api/settings", nil, a.Settings), 403)
	limited := request(a, user, "GET", "/api/preferences", nil, a.Settings)
	expect(t, limited, 200)
	if !strings.Contains(limited.Body.String(), `"showHiddenFiles":true`) || strings.Contains(limited.Body.String(), "storageRoot") || !strings.Contains(limited.Body.String(), "Home NAS") {
		t.Fatal("incorrect user settings response")
	}
	for _, badSettings := range []core.AppSettings{{AppName: "", FileView: "grid", ShareExpiry: "7"}, {AppName: "Test", FileView: "bad", ShareExpiry: "7"}, {AppName: "Test", FileView: "grid", ShareExpiry: "365"}, {AppName: strings.Repeat("a", 41), FileView: "list", ShareExpiry: "never"}} {
		expect(t, jsonRequest(a, admin, "POST", "/api/settings", badSettings, a.Settings), 400)
	}
	for _, logo := range []string{"javascript:alert(1)", "data:image/svg+xml,test", "//example.com/logo.png", "https://user:password@example.com/logo.png"} {
		invalid := configured
		invalid.LogoURL = logo
		expect(t, jsonRequest(a, admin, "POST", "/api/settings", invalid, a.Settings), 400)
	}
	for _, color := range []string{"red", "#123", "#gggggg", "#123456; color: red"} {
		invalid := configured
		invalid.LightPrimary = color
		expect(t, jsonRequest(a, admin, "POST", "/api/settings", invalid, a.Settings), 400)
	}
	invalidMode := configured
	invalidMode.ThemeMode = "invalid"
	expect(t, jsonRequest(a, admin, "POST", "/api/settings", invalidMode, a.Settings), 400)
	invalid := configured
	invalid.FooterText = strings.Repeat("x", 201)
	expect(t, jsonRequest(a, admin, "POST", "/api/settings", invalid, a.Settings), 400)
	after, _ := a.ReadSettings()
	if after != configured {
		t.Fatal("invalid settings changed saved configuration")
	}
}

func TestPublicTheme(t *testing.T) {
	a, admin, _ := setup(t)
	configured := core.AppSettings{AppName: "Private NAS", FileView: "grid", ShareExpiry: "never", ThemeMode: "dark", LightPrimary: "#1e64d6", DarkPrimary: "#38bdf8", LightBackground: "#f4f6fa", DarkBackground: "#101522", LightCard: "#ffffff", DarkCard: "#181f2f"}
	expect(t, jsonRequest(a, admin, "POST", "/api/settings", configured, a.Settings), 200)
	w := httptest.NewRecorder()
	a.Router().ServeHTTP(w, httptest.NewRequest("GET", "/api/branding/theme", nil))
	expect(t, w, 200)
	var theme map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &theme); err != nil {
		t.Fatal(err)
	}
	if len(theme) != 7 || theme["themeMode"] != "dark" || theme["lightPrimary"] != "#1e64d6" || theme["darkCard"] != "#181f2f" {
		t.Fatalf("unexpected public theme: %v", theme)
	}
	if strings.Contains(w.Body.String(), "Private NAS") || strings.Contains(w.Body.String(), "storageRoot") || strings.Contains(w.Body.String(), "registrationEnabled") {
		t.Fatal("public theme exposed unrelated settings")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("public theme must not be cached")
	}
	script := httptest.NewRecorder()
	a.Router().ServeHTTP(script, httptest.NewRequest("GET", "/api/branding/theme.js", nil))
	expect(t, script, 200)
	if !strings.Contains(script.Header().Get("Content-Type"), "javascript") || !strings.Contains(script.Body.String(), "/api/branding/theme") {
		t.Fatal("missing public share theme script")
	}
}

package app

import (
	"encoding/json"
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
	if response.Settings.AppName != "TryNAS" || response.Settings.FileView != "grid" || response.Settings.ShareExpiry != "7" {
		t.Fatal("incorrect defaults")
	}
	configured := core.AppSettings{AppName: "Home NAS", LogoURL: "https://example.com/logo.png", FooterText: "© My home", FileView: "list", ShareExpiry: "15"}
	expect(t, jsonRequest(a, user, "POST", "/api/settings", configured, a.Settings), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/settings", configured, a.Settings), 200)
	saved, err := a.ReadSettings()
	if err != nil || saved != configured {
		t.Fatal("settings not persisted", err)
	}
	expect(t, request(a, user, "GET", "/api/settings", nil, a.Settings), 403)
	limited := request(a, user, "GET", "/api/preferences", nil, a.Settings)
	expect(t, limited, 200)
	if strings.Contains(limited.Body.String(), "storageRoot") || !strings.Contains(limited.Body.String(), "Home NAS") {
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
	invalid := configured
	invalid.FooterText = strings.Repeat("x", 201)
	expect(t, jsonRequest(a, admin, "POST", "/api/settings", invalid, a.Settings), 400)
	after, _ := a.ReadSettings()
	if after != configured {
		t.Fatal("invalid settings changed saved configuration")
	}
}

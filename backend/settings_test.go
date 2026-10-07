package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBasicAppSettings(t *testing.T) {
	a, admin, user := setup(t)
	w := request(a, admin, "GET", "/api/settings", nil, a.settings)
	expect(t, w, 200)
	var response struct{ Settings AppSettings }
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Settings.AppName != "TryNAS" || response.Settings.FileView != "grid" || response.Settings.ShareExpiry != "7" {
		t.Fatal("incorrect defaults")
	}
	configured := AppSettings{"Home NAS", "list", "15"}
	expect(t, jsonRequest(a, user, "POST", "/api/settings", configured, a.settings), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/settings", configured, a.settings), 200)
	saved, err := a.readSettings()
	if err != nil || saved != configured {
		t.Fatal("settings not persisted", err)
	}
	limited := request(a, user, "GET", "/api/settings", nil, a.settings)
	expect(t, limited, 200)
	if strings.Contains(limited.Body.String(), "storageRoot") || !strings.Contains(limited.Body.String(), "Home NAS") {
		t.Fatal("incorrect user settings response")
	}
	for _, badSettings := range []AppSettings{{"", "grid", "7"}, {"Test", "bad", "7"}, {"Test", "grid", "365"}, {strings.Repeat("a", 41), "list", "never"}} {
		expect(t, jsonRequest(a, admin, "POST", "/api/settings", badSettings, a.settings), 400)
	}
	after, _ := a.readSettings()
	if after != configured {
		t.Fatal("invalid settings changed saved configuration")
	}
}

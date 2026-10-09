package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rajib/trynas/core"
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
	configured := core.AppSettings{"Home NAS", "list", "15"}
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
	for _, badSettings := range []core.AppSettings{{"", "grid", "7"}, {"Test", "bad", "7"}, {"Test", "grid", "365"}, {strings.Repeat("a", 41), "list", "never"}} {
		expect(t, jsonRequest(a, admin, "POST", "/api/settings", badSettings, a.Settings), 400)
	}
	after, _ := a.ReadSettings()
	if after != configured {
		t.Fatal("invalid settings changed saved configuration")
	}
}

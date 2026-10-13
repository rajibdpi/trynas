package app

import (
	"encoding/json"
	"github.com/joho/godotenv"
	"os"
	"path/filepath"
	"testing"
)

func TestServerConfiguration(t *testing.T) {
	a, admin, user := setup(t)
	file := filepath.Join(t.TempDir(), "server.env")
	t.Setenv("TRYNAS_CONFIG_FILE", file)
	if err := os.WriteFile(file, []byte("ADMIN_PASSWORD=keep-this-secret\nDB_PATH=./data/test.db\n"), 0600); err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"storageRoot": "./new-storage", "address": "127.0.0.1:8081", "corsOrigin": "http://localhost:3000", "cookieSecure": false}
	expect(t, jsonRequest(a, user, "POST", "/api/settings/server", input, a.SaveServerConfiguration), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/settings/server", input, a.SaveServerConfiguration), 200)
	saved, err := godotenv.Read(file)
	if err != nil || saved["ADDR"] != "127.0.0.1:8081" || saved["ADMIN_PASSWORD"] != "keep-this-secret" {
		t.Fatal("incorrect saved configuration", err)
	}
	input["address"] = "invalid"
	expect(t, jsonRequest(a, admin, "POST", "/api/settings/server", input, a.SaveServerConfiguration), 400)
}

func TestServerProxyUploadLimit(t *testing.T) {
	a, admin, user := setup(t)
	file := filepath.Join(t.TempDir(), "server.env")
	t.Setenv("TRYNAS_CONFIG_FILE", file)
	t.Setenv("PROXY_UPLOAD_LIMIT_MB", "")
	input := map[string]any{"storageRoot": a.Root, "address": "127.0.0.1:8080", "corsOrigin": "http://localhost:3000", "cookieSecure": false, "proxyUploadLimitMB": 128}
	expect(t, jsonRequest(a, user, "POST", "/api/settings/server", input, a.SaveServerConfiguration), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/settings/server", input, a.SaveServerConfiguration), 200)
	saved, err := godotenv.Read(file)
	if err != nil || saved["PROXY_UPLOAD_LIMIT_MB"] != "128" {
		t.Fatal("upload limit not persisted", err)
	}
	response := request(a, admin, "GET", "/api/settings", nil, a.Settings)
	expect(t, response, 200)
	var result struct {
		Server struct{ ProxyUploadLimitMB int }
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Server.ProxyUploadLimitMB != 128 {
		t.Fatal("saved limit not shown in Settings", err)
	}
	for _, limit := range []any{0, -1, 10241, 1.5, "128"} {
		input["proxyUploadLimitMB"] = limit
		expect(t, jsonRequest(a, admin, "POST", "/api/settings/server", input, a.SaveServerConfiguration), 400)
	}
	delete(input, "proxyUploadLimitMB")
	expect(t, jsonRequest(a, admin, "POST", "/api/settings/server", input, a.SaveServerConfiguration), 200)
	saved, err = godotenv.Read(file)
	if err != nil || saved["PROXY_UPLOAD_LIMIT_MB"] != "128" {
		t.Fatal("older client erased upload setting", err)
	}
}

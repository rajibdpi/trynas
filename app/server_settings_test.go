package app

import (
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
	input := map[string]any{"storageRoot": "./new-storage", "address": "127.0.0.1:8081", "corsOrigin": "http://localhost:3000", "cookieSecure": false, "smartUseSudo": true}
	expect(t, jsonRequest(a, user, "POST", "/api/settings/server", input, a.SaveServerConfiguration), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/settings/server", input, a.SaveServerConfiguration), 200)
	saved, err := godotenv.Read(file)
	if err != nil || saved["ADDR"] != "127.0.0.1:8081" || saved["ADMIN_PASSWORD"] != "keep-this-secret" || saved["SMART_USE_SUDO"] != "true" {
		t.Fatal("incorrect saved configuration", err)
	}
	input["address"] = "invalid"
	expect(t, jsonRequest(a, admin, "POST", "/api/settings/server", input, a.SaveServerConfiguration), 400)
}

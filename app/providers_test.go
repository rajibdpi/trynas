package app

import (
	"strings"
	"testing"

	"github.com/rajibdpi/trynas/core"
)

func TestStorageConnectionsAccessAndSecrets(t *testing.T) {
	a, admin, user := setup(t)
	payload := map[string]any{"action": "save", "connection": core.StorageConnection{Name: "Office", Provider: "smb", Root: "documents", Options: map[string]string{"host": "nas.example", "user": "alice", "pass": "provider-super-secret"}}}
	expect(t, jsonRequest(a, user, "POST", "/api/storage/providers", payload, a.StorageProviders), 403)
	expect(t, request(a, user, "GET", "/api/storage/providers", nil, a.StorageProviders), 403)
	w := jsonRequest(a, admin, "POST", "/api/storage/providers", payload, a.StorageProviders)
	expect(t, w, 200)
	if strings.Contains(w.Body.String(), "provider-super-secret") {
		t.Fatal("save exposes credentials")
	}
	c, err := a.Connections()
	if err != nil || len(c) != 1 || c[0].Options["pass"] != "provider-super-secret" {
		t.Fatal("connection did not persist", err)
	}
	var raw string
	if err = a.DB.QueryRow("SELECT value FROM app_settings WHERE key='storage_connections'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "provider-super-secret") || strings.Contains(raw, "nas.example") {
		t.Fatal("unencrypted credentials in database")
	}
	if _, err = a.DecryptConnections([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	w = request(a, admin, "GET", "/api/storage/providers", nil, a.StorageProviders)
	expect(t, w, 200)
	if strings.Contains(w.Body.String(), "options") || strings.Contains(w.Body.String(), "provider-super-secret") {
		t.Fatal("list exposes credentials")
	}
	for _, c := range []core.StorageConnection{{Name: "Bad", Provider: "local", Root: "/etc"}, {Name: "Bad", Provider: "sftp", Options: map[string]string{"host": "server", "user": "user", "pass": "p", "known_hosts_file": "none"}}, {Name: "Bad", Provider: "drive", Options: map[string]string{"token": "{}"}}, {Name: "Bad", Provider: "smb", Options: map[string]string{"host": "h", "user": "u", "shell": "bad"}}} {
		expect(t, jsonRequest(a, admin, "POST", "/api/storage/providers", map[string]any{"action": "save", "connection": c}, a.StorageProviders), 400)
	}
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/providers", map[string]any{"action": "delete", "id": c[0].ID}, a.StorageProviders), 200)
}

func TestStorageSyncActionsRemoved(t *testing.T) {
	a, admin, _ := setup(t)
	for _, action := range []string{"transfer", "cancel"} {
		expect(t, jsonRequest(a, admin, "POST", "/api/storage/providers", map[string]any{"action": action}, a.StorageProviders), 400)
	}
	// Legacy sync history must not appear in the connections response.
	if _, err := a.DB.Exec(`INSERT INTO app_settings(key,value) VALUES('storage_job','{"status":"failed"}')`); err != nil {
		t.Fatal(err)
	}
	w := request(a, admin, "GET", "/api/storage/providers", nil, a.StorageProviders)
	expect(t, w, 200)
	if strings.Contains(w.Body.String(), `"job"`) || strings.Contains(w.Body.String(), "failed") {
		t.Fatal("obsolete sync history returned", w.Body.String())
	}
}

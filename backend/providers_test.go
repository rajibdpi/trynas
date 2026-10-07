package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStorageConnectionsAccessAndSecrets(t *testing.T) {
	a, admin, user := setup(t)
	payload := map[string]any{"action": "save", "connection": storageConnection{Name: "Office", Provider: "smb", Root: "documents", Options: map[string]string{"host": "nas.example", "user": "alice", "pass": "provider-super-secret"}}}
	expect(t, jsonRequest(a, user, "POST", "/api/storage/providers", payload, a.storageProviders), 403)
	expect(t, request(a, user, "GET", "/api/storage/providers", nil, a.storageProviders), 403)
	w := jsonRequest(a, admin, "POST", "/api/storage/providers", payload, a.storageProviders)
	expect(t, w, 200)
	if strings.Contains(w.Body.String(), "provider-super-secret") {
		t.Fatal("save exposes credentials")
	}
	c, err := a.connections()
	if err != nil || len(c) != 1 || c[0].Options["pass"] != "provider-super-secret" {
		t.Fatal("connection did not persist", err)
	}
	var raw string
	if err = a.db.QueryRow("SELECT value FROM app_settings WHERE key='storage_connections'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "provider-super-secret") || strings.Contains(raw, "nas.example") {
		t.Fatal("unencrypted credentials in database")
	}
	if _, err = a.decryptConnections([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	w = request(a, admin, "GET", "/api/storage/providers", nil, a.storageProviders)
	expect(t, w, 200)
	if strings.Contains(w.Body.String(), "options") || strings.Contains(w.Body.String(), "provider-super-secret") {
		t.Fatal("list exposes credentials")
	}
	for _, c := range []storageConnection{{Name: "Bad", Provider: "local", Root: "/etc"}, {Name: "Bad", Provider: "sftp", Options: map[string]string{"host": "server", "user": "user", "pass": "p", "known_hosts_file": "none"}}, {Name: "Bad", Provider: "drive", Options: map[string]string{"token": "{}"}}, {Name: "Bad", Provider: "smb", Options: map[string]string{"host": "h", "user": "u", "shell": "bad"}}} {
		expect(t, jsonRequest(a, admin, "POST", "/api/storage/providers", map[string]any{"action": "save", "connection": c}, a.storageProviders), 400)
	}
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/providers", map[string]any{"action": "delete", "id": c[0].ID}, a.storageProviders), 200)
}
func waitStorageJob(t *testing.T, a *App) transferJob {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		a.providers.mu.Lock()
		if a.providers.job != nil && a.providers.job.Status != "running" {
			job := *a.providers.job
			a.providers.mu.Unlock()
			return job
		}
		a.providers.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("transfer did not finish")
	return transferJob{}
}
func TestStorageCopyMirrorAndPreview(t *testing.T) {
	a, admin, _ := setup(t)
	remote := t.TempDir()
	c := storageConnection{ID: "test-local", Name: "Test remote", Provider: "local", Root: remote, Options: map[string]string{}}
	if err := a.saveConnections([]storageConnection{c}); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{"folder/a.txt": "hello", ".trynas-trash/private.txt": "secret"} {
		full := filepath.Join(a.root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(direction, mode, path string, dry, confirm bool) transferJob {
		t.Helper()
		w := jsonRequest(a, admin, "POST", "/api/storage/providers", map[string]any{"action": "transfer", "id": c.ID, "direction": direction, "mode": mode, "localPath": path, "dryRun": dry, "confirmDelete": confirm}, a.storageProviders)
		expect(t, w, 200)
		job := waitStorageJob(t, a)
		if job.Status != "completed" {
			t.Fatalf("transfer status: %+v", job)
		}
		return job
	}
	run("upload", "copy", "", true, false)
	if _, err := os.Stat(filepath.Join(remote, "folder/a.txt")); !os.IsNotExist(err) {
		t.Fatal("preview wrote a file")
	}
	run("upload", "copy", "", false, false)
	if data, err := os.ReadFile(filepath.Join(remote, "folder/a.txt")); err != nil || string(data) != "hello" {
		t.Fatal("copy failed", err)
	}
	if _, err := os.Stat(filepath.Join(remote, ".trynas-trash")); !os.IsNotExist(err) {
		t.Fatal("copied internal trash")
	}
	if err := os.WriteFile(filepath.Join(remote, "extra.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	run("upload", "copy", "", false, false)
	if _, err := os.Stat(filepath.Join(remote, "extra.txt")); err != nil {
		t.Fatal("copy deleted extra file")
	}
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/providers", map[string]any{"action": "transfer", "id": c.ID, "direction": "upload", "mode": "mirror", "localPath": "", "dryRun": false}, a.storageProviders), 400)
	run("upload", "mirror", "", false, true)
	if _, err := os.Stat(filepath.Join(remote, "extra.txt")); !os.IsNotExist(err) {
		t.Fatal("mirror did not delete extra file")
	}
	run("download", "copy", "new-preview", true, false)
	if _, err := os.Stat(filepath.Join(a.root, "new-preview")); !os.IsNotExist(err) {
		t.Fatal("preview created destination")
	}
	run("download", "copy", "import", false, false)
	if data, err := os.ReadFile(filepath.Join(a.root, "import/folder/a.txt")); err != nil || string(data) != "hello" {
		t.Fatal("download failed", err)
	}
	var owner, size int
	if err := a.db.QueryRow("SELECT user_id,bytes FROM file_owners WHERE path='import/folder/a.txt'").Scan(&owner, &size); err != nil || owner != admin.ID || size != 5 {
		t.Fatal("import ownership not recorded", err)
	}
	if _, err := os.Stat(filepath.Join(a.root, ".trynas-trash/private.txt")); err != nil {
		t.Fatal("internal trash modified")
	}
	for _, p := range []string{"../outside", ".trynas-trash"} {
		expect(t, jsonRequest(a, admin, "POST", "/api/storage/providers", map[string]any{"action": "transfer", "id": c.ID, "direction": "download", "mode": "copy", "localPath": p}, a.storageProviders), 400)
	}
	if err := os.Symlink(remote, filepath.Join(a.root, "link")); err != nil {
		t.Fatal(err)
	}
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/providers", map[string]any{"action": "transfer", "id": c.ID, "direction": "upload", "mode": "copy", "localPath": "link"}, a.storageProviders), 400)
	var saved string
	if err := a.db.QueryRow("SELECT value FROM app_settings WHERE key='storage_job'").Scan(&saved); err != nil {
		t.Fatal(err)
	}
	var job transferJob
	if json.Unmarshal([]byte(saved), &job) != nil || job.Status != "completed" {
		t.Fatal("job status not persisted")
	}
}

func TestStorageCancellationAndRestart(t *testing.T) {
	a, admin, _ := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	a.providers.job = &transferJob{ID: "running", Status: "running", cancel: cancel}
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/providers", map[string]any{"action": "save"}, a.storageProviders), 409)
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/providers", map[string]any{"action": "cancel"}, a.storageProviders), 200)
	if ctx.Err() == nil {
		t.Fatal("cancellation not delivered")
	}
	if err := a.saveStorageJob(a.providers.job); err != nil {
		t.Fatal(err)
	}
	a.providers.job = nil
	w := request(a, admin, "GET", "/api/storage/providers", nil, a.storageProviders)
	expect(t, w, 200)
	if !strings.Contains(w.Body.String(), "interrupted") {
		t.Fatal("restart does not identify interrupted job")
	}
}

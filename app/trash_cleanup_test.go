package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTrashRetention(t *testing.T) {
	a, admin, _ := setup(t)
	settings, err := a.ReadSettings()
	if err != nil || settings.TrashRetentionDays != 30 {
		t.Fatal("expected 30-day default", err)
	}
	for _, name := range []string{"old.txt", "recent.txt"} {
		if err := os.WriteFile(filepath.Join(a.Root, name), []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := a.TrashOne(admin, name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.DB.Exec("UPDATE trash SET deleted_at=datetime('now','-31 days') WHERE original_path='old.txt'"); err != nil {
		t.Fatal(err)
	}
	settings.TrashRetentionDays = 0
	expect(t, jsonRequest(a, admin, "POST", "/api/settings", settings, a.Settings), 200)
	if err := a.CleanupExpired(time.Now()); err != nil {
		t.Fatal(err)
	}
	var count int
	a.DB.QueryRow("SELECT count(*) FROM trash").Scan(&count)
	if count != 2 {
		t.Fatal("disabled cleanup removed trash")
	}
	settings.TrashRetentionDays = 30
	expect(t, jsonRequest(a, admin, "POST", "/api/settings", settings, a.Settings), 200)
	var oldName string
	a.DB.QueryRow("SELECT trash_name FROM trash WHERE original_path='old.txt'").Scan(&oldName)
	if err := a.CleanupExpired(time.Now()); err != nil {
		t.Fatal(err)
	}
	a.DB.QueryRow("SELECT count(*) FROM trash").Scan(&count)
	if count != 1 {
		t.Fatal("incorrect remaining trash", count)
	}
	if _, err := os.Stat(filepath.Join(a.Trash, oldName)); !os.IsNotExist(err) {
		t.Fatal("old trash still exists")
	}
	settings.TrashRetentionDays = -1
	expect(t, jsonRequest(a, admin, "POST", "/api/settings", settings, a.Settings), 400)
}

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFolderUploadReusesDirectories(t *testing.T) {
	a, admin, user := setup(t)
	if err := os.Mkdir(filepath.Join(a.Root, "Album"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Root, "existing.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	expect(t, jsonRequest(a, admin, "POST", "/api/files/folder", map[string]any{"path": "Album", "existOk": true}, a.Mkdir), 200)
	expect(t, jsonRequest(a, admin, "POST", "/api/files/folder", map[string]any{"path": "Album"}, a.Mkdir), 409)
	expect(t, jsonRequest(a, admin, "POST", "/api/files/folder", map[string]any{"path": "existing.txt", "existOk": true}, a.Mkdir), 409)
	expect(t, jsonRequest(a, user, "POST", "/api/files/folder", map[string]any{"path": "Album", "existOk": true}, a.Mkdir), 403)
	data, _ := os.ReadFile(filepath.Join(a.Root, "existing.txt"))
	if string(data) != "keep" {
		t.Fatal("existing file changed")
	}
}

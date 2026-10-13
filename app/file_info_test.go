package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFileInfoMetadataAndPermissions(t *testing.T) {
	a, admin, user := setup(t)
	if err := os.Mkdir(filepath.Join(a.Root, "docs"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Root, "docs", "note.txt"), []byte("hello"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(a.Root, "docs", "nested"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Root, "docs", ".trynas-secret"), []byte("hidden"), 0600); err != nil {
		t.Fatal(err)
	}
	a.Log(admin.ID, "upload", "docs/note.txt")
	response := request(a, admin, "GET", "/api/files/info?path=docs/note.txt", nil, a.FileInfo)
	expect(t, response, 200)
	var info map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info["size"] != float64(5) || info["extension"] != "txt" || info["permissions"] != "0640" || info["modified"] == nil {
		t.Fatalf("wrong metadata: %v", info)
	}
	if _, exists := info["created"]; exists && info["createdBy"] != admin.Username {
		t.Fatalf("creation event not attributed: %v", info)
	}
	folder := request(a, admin, "GET", "/api/files/info?path=docs", nil, a.FileInfo)
	expect(t, folder, 200)
	if err := json.Unmarshal(folder.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info["files"] != float64(1) || info["folders"] != float64(1) {
		t.Fatalf("wrong folder counts: %v", info)
	}
	expect(t, request(a, user, "GET", "/api/files/info?path=docs/note.txt", nil, a.FileInfo), 403)
	grant(t, a, user, "docs/note.txt", "read")
	expect(t, request(a, user, "GET", "/api/files/info?path=docs/note.txt", nil, a.FileInfo), 200)
	expect(t, request(a, admin, "GET", "/api/files/info?path=../outside", nil, a.FileInfo), 400)
	expect(t, request(a, admin, "GET", "/api/files/info?path=missing", nil, a.FileInfo), 404)
	if err := os.Symlink(filepath.Join(a.Root, "docs", "note.txt"), filepath.Join(a.Root, "link")); err == nil {
		expect(t, request(a, admin, "GET", "/api/files/info?path=link", nil, a.FileInfo), 400)
	}
}

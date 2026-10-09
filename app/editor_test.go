package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFileEditor(t *testing.T) {
	a, admin, user := setup(t)
	full := filepath.Join(a.Root, "edit.txt")
	if err := os.WriteFile(full, []byte("original\n"), 0600); err != nil {
		t.Fatal(err)
	}
	opened := request(a, admin, "GET", "/api/files/edit?path=edit.txt", nil, a.EditFile)
	expect(t, opened, 200)
	var doc struct {
		Content, Version string
		Writable         bool
	}
	if err := json.Unmarshal(opened.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Content != "original\n" || !doc.Writable {
		t.Fatal("incorrect editor response")
	}
	input := map[string]string{"path": "edit.txt", "content": "changed\n", "version": doc.Version}
	expect(t, jsonRequest(a, user, "POST", "/api/files/edit", input, a.EditFile), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/files/edit", input, a.EditFile), 200)
	expect(t, jsonRequest(a, admin, "POST", "/api/files/edit", input, a.EditFile), 409)
	data, err := os.ReadFile(full)
	if err != nil || string(data) != "changed\n" {
		t.Fatal("file was not saved", err)
	}
	expect(t, request(a, admin, "GET", "/api/files/edit?path=../outside", nil, a.EditFile), 400)
	if err := os.WriteFile(full, []byte{0, 1, 2}, 0600); err != nil {
		t.Fatal(err)
	}
	expect(t, request(a, admin, "GET", "/api/files/edit?path=edit.txt", nil, a.EditFile), 400)
}

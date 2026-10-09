package app

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestUnzip(t *testing.T) {
	a, admin, user := setup(t)
	create := func(name, entry string) {
		file, err := os.Create(filepath.Join(a.Root, name))
		if err != nil {
			t.Fatal(err)
		}
		writer := zip.NewWriter(file)
		part, err := writer.Create(entry)
		if err != nil {
			t.Fatal(err)
		}
		part.Write([]byte("hello"))
		writer.Close()
		file.Close()
	}
	create("test.zip", "Docs/file.txt")
	input := map[string]string{"path": "test.zip", "destination": "extracted"}
	expect(t, jsonRequest(a, user, "POST", "/api/files/unzip", input, a.Unzip), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/files/unzip", input, a.Unzip), 200)
	data, err := os.ReadFile(filepath.Join(a.Root, "extracted", "Docs", "file.txt"))
	if err != nil || string(data) != "hello" {
		t.Fatal("incorrect extraction", err)
	}
	expect(t, jsonRequest(a, admin, "POST", "/api/files/unzip", input, a.Unzip), 409)
	create("unsafe.zip", "../outside.txt")
	expect(t, jsonRequest(a, admin, "POST", "/api/files/unzip", map[string]string{"path": "unsafe.zip", "destination": "unsafe"}, a.Unzip), 400)
	if _, err := os.Stat(filepath.Join(a.Root, "unsafe")); !os.IsNotExist(err) {
		t.Fatal("unsafe extraction created destination")
	}
}

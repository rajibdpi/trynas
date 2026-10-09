package app

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestZipSelectionCreatesOneArchive(t *testing.T) {
	a, admin, _ := setup(t)
	for _, name := range []string{"one.txt", "two.txt"} {
		if err := os.WriteFile(filepath.Join(a.Root, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	expect(t, jsonRequest(a, admin, "POST", "/api/files/zip", map[string]any{"paths": []string{"one.txt", "two.txt", "one.txt"}, "destination": "combined.zip"}, a.ZipV5), 200)
	archive, err := zip.OpenReader(filepath.Join(a.Root, "combined.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != 2 || archive.File[0].Name != "one.txt" || archive.File[1].Name != "two.txt" {
		t.Fatal("incorrect combined archive")
	}
	expect(t, jsonRequest(a, admin, "POST", "/api/files/zip", map[string]any{"paths": []string{"one.txt"}, "destination": "combined.zip"}, a.ZipV5), 409)
}

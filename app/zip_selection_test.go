package app

import (
	"archive/zip"
	"fmt"
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

func TestArchiveCreationBeyondFormerLimits(t *testing.T) {
	t.Setenv("ARCHIVE_CREATE_MAX_ENTRIES", "0")
	t.Setenv("ARCHIVE_CREATE_MAX_BYTES", "0")
	a, admin, _ := setup(t)
	folder := filepath.Join(a.Root, "many")
	if err := os.Mkdir(folder, 0755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10001; i++ {
		if err := os.WriteFile(filepath.Join(folder, fmt.Sprintf("%05d.txt", i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	large, err := os.Create(filepath.Join(folder, "large.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := large.Truncate((1 << 30) + 1); err != nil {
		large.Close()
		t.Fatal(err)
	}
	large.Close()
	expect(t, jsonRequest(a, admin, "POST", "/api/files/zip", map[string]any{"paths": []string{"many"}, "destination": "large.zip"}, a.ZipV5), 200)
	archive, err := zip.OpenReader(filepath.Join(a.Root, "large.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != 10003 {
		t.Fatalf("archive has %d entries", len(archive.File))
	}
	for _, entry := range archive.File {
		if entry.Name == "many/large.bin" && entry.UncompressedSize64 == (1<<30)+1 {
			return
		}
	}
	t.Fatal("large file missing or truncated")
}

func TestArchiveCreationConfiguredLimits(t *testing.T) {
	for _, test := range []struct {
		name, entries, bytes string
		status               int
	}{
		{"entry limit", "1", "0", 413},
		{"size limit", "0", "4", 413},
		{"exact size", "0", "5", 200},
		{"unset", "", "", 200},
		{"invalid", "invalid", "0", 500},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ARCHIVE_CREATE_MAX_ENTRIES", test.entries)
			t.Setenv("ARCHIVE_CREATE_MAX_BYTES", test.bytes)
			a, admin, _ := setup(t)
			write(t, a, "folder/file.txt", "hello")
			expect(t, jsonRequest(a, admin, "POST", "/api/files/zip", map[string]any{"paths": []string{"folder"}, "destination": "test.zip"}, a.ZipV5), test.status)
			if test.status != 200 {
				if _, err := os.Stat(filepath.Join(a.Root, "test.zip")); !os.IsNotExist(err) {
					t.Fatal("failed archive was published")
				}
			}
		})
	}
}

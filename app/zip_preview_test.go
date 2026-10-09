package app

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestZipPreviewDoesNotExtract(t *testing.T) {
	a, admin, user := setup(t)
	file, err := os.Create(filepath.Join(a.Root, "test.zip"))
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, name := range []string{"Docs/", "Docs/test.txt", "../outside.txt"} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if name != "Docs/" {
			if _, err = entry.Write([]byte("contents")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	file.Close()
	response := request(a, admin, "GET", "/api/files/archive?path=test.zip", nil, a.ZipContents)
	expect(t, response, 200)
	var data struct {
		Total   int
		Entries []struct {
			Name      string
			Directory bool
			Size      int
		}
	}
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data.Total != 3 || !data.Entries[0].Directory || data.Entries[1].Size != 8 {
		t.Fatal("incorrect ZIP metadata", data)
	}
	if _, err := os.Stat(filepath.Join(a.Root, "Docs")); !os.IsNotExist(err) {
		t.Fatal("archive was extracted")
	}
	expect(t, request(a, user, "GET", "/api/files/archive?path=test.zip", nil, a.ZipContents), 403)
	if err := os.WriteFile(filepath.Join(a.Root, "invalid.zip"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	expect(t, request(a, admin, "GET", "/api/files/archive?path=invalid.zip", nil, a.ZipContents), 400)
}

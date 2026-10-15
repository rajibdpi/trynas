package app

import (
	"archive/tar"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveFormatsRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("7zz"); err != nil {
		if _, err = exec.LookPath("7z"); err != nil {
			t.Skip("7-Zip is not installed")
		}
	}
	for _, extension := range []string{"zip", "7z", "xz", "bz2", "gz", "tar", "wim"} {
		t.Run(extension, func(t *testing.T) {
			a, admin, user := setup(t)
			write(t, a, "hello.txt", "archive round trip")
			archive := "packed." + extension
			input := map[string]any{"paths": []string{"hello.txt"}, "destination": archive}
			expect(t, jsonRequest(a, user, "POST", "/api/files/zip", input, a.ZipV5), 403)
			expect(t, jsonRequest(a, admin, "POST", "/api/files/zip", input, a.ZipV5), 200)
			expect(t, jsonRequest(a, admin, "POST", "/api/files/zip", input, a.ZipV5), 409)
			expect(t, jsonRequest(a, admin, "POST", "/api/files/unzip", map[string]string{"path": archive, "destination": "unpacked"}, a.Unzip), 200)
			found := false
			err := filepath.Walk(filepath.Join(a.Root, "unpacked"), func(p string, st os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if st.IsDir() {
					return nil
				}
				data, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				if string(data) == "archive round trip" {
					found = true
				}
				return nil
			})
			if err != nil || !found {
				t.Fatalf("missing extracted contents: %v", err)
			}
			expect(t, jsonRequest(a, admin, "POST", "/api/files/unzip", map[string]string{"path": archive, "destination": "unpacked"}, a.Unzip), 409)
		})
	}
}

func TestSingleFileCompressionRejectsFolders(t *testing.T) {
	if _, err := exec.LookPath("7zz"); err != nil {
		t.Skip("7zz is not installed")
	}
	a, admin, _ := setup(t)
	write(t, a, "folder/file.txt", "hello")
	for _, extension := range []string{"xz", "bz2", "gz"} {
		result := jsonRequest(a, admin, "POST", "/api/files/zip", map[string]any{"paths": []string{"folder"}, "destination": "packed." + extension}, a.ZipV5)
		expect(t, result, 400)
		if !strings.Contains(result.Body.String(), "single file") {
			t.Fatal(result.Body.String())
		}
	}
}

func TestArchiveRejectsUnsafeTar(t *testing.T) {
	if _, err := exec.LookPath("7zz"); err != nil {
		t.Skip("7zz is not installed")
	}
	for _, entry := range []struct {
		name string
		kind byte
		link string
	}{
		{"../outside.txt", tar.TypeReg, ""},
		{"link", tar.TypeSymlink, "/tmp/outside"},
	} {
		t.Run(entry.name, func(t *testing.T) {
			a, admin, _ := setup(t)
			f, err := os.Create(filepath.Join(a.Root, "unsafe.tar"))
			if err != nil {
				t.Fatal(err)
			}
			tw := tar.NewWriter(f)
			if err = tw.WriteHeader(&tar.Header{Name: entry.name, Typeflag: entry.kind, Linkname: entry.link, Mode: 0644}); err != nil {
				t.Fatal(err)
			}
			if err = tw.Close(); err != nil {
				t.Fatal(err)
			}
			f.Close()
			expect(t, jsonRequest(a, admin, "POST", "/api/files/unzip", map[string]string{"path": "unsafe.tar", "destination": "extracted"}, a.Unzip), 400)
			if _, err = os.Stat(filepath.Join(a.Root, "extracted")); !os.IsNotExist(err) {
				t.Fatal("unsafe archive created destination")
			}
		})
	}
}

func TestArchiveFoldersRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("7zz"); err != nil {
		t.Skip("7zz is not installed")
	}
	for _, extension := range []string{"7z", "tar", "wim"} {
		t.Run(extension, func(t *testing.T) {
			a, admin, _ := setup(t)
			write(t, a, "folder/nested/@notes.txt", "nested contents")
			write(t, a, "folder/star*.txt", "literal wildcard")
			archive := "packed." + extension
			expect(t, jsonRequest(a, admin, "POST", "/api/files/zip", map[string]any{"paths": []string{"folder"}, "destination": archive}, a.ZipV5), 200)
			expect(t, jsonRequest(a, admin, "POST", "/api/files/unzip", map[string]string{"path": archive, "destination": "unpacked"}, a.Unzip), 200)
			for name, expected := range map[string]string{"nested/@notes.txt": "nested contents", "star*.txt": "literal wildcard"} {
				data, err := os.ReadFile(filepath.Join(a.Root, "unpacked", "folder", name))
				if err != nil || string(data) != expected {
					t.Fatalf("%s: %q, %v", name, data, err)
				}
			}
		})
	}
}

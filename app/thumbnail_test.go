package app

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileThumbnails(t *testing.T) {
	a, admin, user := setup(t)
	var source bytes.Buffer
	_ = png.Encode(&source, image.NewRGBA(image.Rect(0, 0, 800, 400)))
	if err := os.WriteFile(filepath.Join(a.Root, "photo.png"), source.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	response := request(a, admin, "GET", "/api/files/thumbnail?path=photo.png", nil, a.Thumbnail)
	expect(t, response, 200)
	config, _, err := image.DecodeConfig(response.Body)
	if err != nil || config.Width != 160 || config.Height != 80 {
		t.Fatalf("thumbnail not resized: %+v %v", config, err)
	}
	if err := os.WriteFile(filepath.Join(a.Root, "notes.txt"), []byte("<script>alert('test')</script>\nSecond line"), 0600); err != nil {
		t.Fatal(err)
	}
	response = request(a, admin, "GET", "/api/files/thumbnail?path=notes.txt", nil, a.Thumbnail)
	expect(t, response, 200)
	if response.Header().Get("Content-Type") != "image/svg+xml" || strings.Contains(response.Body.String(), "<script>") || !strings.Contains(response.Body.String(), "&lt;script&gt;") || !strings.Contains(response.Body.String(), "Second line") {
		t.Fatal("unsafe or missing text thumbnail", response.Body.String())
	}
	expect(t, request(a, user, "GET", "/api/files/thumbnail?path=notes.txt", nil, a.Thumbnail), 403)
	expect(t, request(a, admin, "GET", "/api/files/thumbnail?path=../outside", nil, a.Thumbnail), 400)
	if err := os.WriteFile(filepath.Join(a.Root, "unknown.bin"), []byte{0, 1, 2}, 0600); err != nil {
		t.Fatal(err)
	}
	response = request(a, admin, "GET", "/api/files/thumbnail?path=unknown.bin", nil, a.Thumbnail)
	expect(t, response, 200)
	if !strings.Contains(response.Body.String(), "BIN") {
		t.Fatal("missing generic file thumbnail")
	}
}

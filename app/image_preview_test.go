package app

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestImagePreviewTypes(t *testing.T) {
	a, admin, _ := setup(t)
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><circle cx="10" cy="10" r="5"/></svg>`)
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, _ = writer.Write(svg)
	_ = writer.Close()
	for _, test := range []struct {
		name, mime, encoding string
		data                 []byte
	}{
		{"drawing.svg", "image/svg+xml", "", svg},
		{"drawing.svgz", "image/svg+xml", "gzip", compressed.Bytes()},
		{"icon.ico", "image/x-icon", "", []byte{0, 0, 1, 0}},
		{"photo.jfif", "image/jpeg", "", []byte{255, 216, 255}},
	} {
		if err := os.WriteFile(filepath.Join(a.Root, test.name), test.data, 0600); err != nil {
			t.Fatal(err)
		}
		response := request(a, admin, "GET", "/api/files/preview?path="+test.name, nil, a.Preview)
		expect(t, response, 200)
		if response.Header().Get("Content-Type") != test.mime || response.Header().Get("Content-Encoding") != test.encoding || !bytes.Equal(response.Body.Bytes(), test.data) {
			t.Fatalf("wrong response for %s: %v", test.name, response.Header())
		}
		if response.Header().Get("Content-Security-Policy") != "sandbox" {
			t.Fatal("missing preview sandbox")
		}
	}
}

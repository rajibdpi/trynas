package main

import (
	"bytes"
	"encoding/json"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"
)

func TestSharePageAndQRDownloads(t *testing.T) {
	a, _, user := setup(t)
	write(t, a, "shared/file.txt", "shared contents")
	grant(t, a, user, "shared", "read")
	created := jsonRequest(a, user, "POST", "/api/shares", map[string]any{"path": "shared/file.txt", "days": 7}, a.createShare)
	expect(t, created, 200)
	var share struct{ Token string }
	if err := json.Unmarshal(created.Body.Bytes(), &share); err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	router.Get("/s/{token}", a.publicShare)
	get := func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "http://example.com/s/"+share.Token+query, nil))
		return w
	}
	page := get("")
	expect(t, page, 200)
	if !strings.Contains(page.Body.String(), "Download QR code") || !strings.Contains(page.Body.String(), "file.txt") || !strings.Contains(page.Header().Get("Content-Type"), "text/html") {
		t.Fatal("missing share page")
	}
	file := get("?download=1")
	expect(t, file, 200)
	if file.Body.String() != "shared contents" || !strings.Contains(file.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("file download changed")
	}
	qr := get("?qr=1&download=1&origin=https%3A%2F%2Fnas.example.com")
	expect(t, qr, 200)
	img, err := png.Decode(bytes.NewReader(qr.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 512 || img.Bounds().Dy() != 512 || qr.Header().Get("Content-Type") != "image/png" || !strings.Contains(qr.Header().Get("Content-Disposition"), "trynas-share-qr.png") {
		t.Fatal("invalid QR download")
	}
	want, err := qrcode.Encode("https://nas.example.com/s/"+share.Token, qrcode.Medium, 512)
	if err != nil || !bytes.Equal(qr.Body.Bytes(), want) {
		t.Fatal("QR did not encode the canonical browser URL")
	}
	expect(t, get("?qr=1&origin=javascript%3Aalert(1)"), 400)
	grant(t, a, user, "shared", "none")
	for _, query := range []string{"", "?download=1", "?qr=1&download=1"} {
		expect(t, get(query), 404)
	}
	grant(t, a, user, "shared", "read")
	if _, err := a.db.Exec("UPDATE shares SET expires_at='2000-01-01' WHERE token=?", share.Token); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"", "?download=1", "?qr=1&download=1"} {
		expect(t, get(query), 404)
	}
}

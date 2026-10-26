package app

import (
	"bytes"
	"encoding/json"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"

	chi "github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"
)

func TestSharePageAndQRDownloads(t *testing.T) {
	a, _, user := setup(t)
	write(t, a, "shared/file.txt", "shared contents")
	grant(t, a, user, "shared", "read")
	created := jsonRequest(a, user, "POST", "/api/shares", map[string]any{"path": "shared/file.txt", "days": 7}, a.CreateShare)
	expect(t, created, 200)
	var share struct{ Token string }
	if err := json.Unmarshal(created.Body.Bytes(), &share); err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	router.Get("/s/{token}", a.PublicShare)
	get := func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "http://example.com/s/"+share.Token+query, nil))
		return w
	}
	if _, err := a.DB.Exec(`INSERT OR REPLACE INTO app_settings(key,value) VALUES('basic',?)`, `{"appName":"Home & NAS","logoUrl":"/api/branding/logo?v=custom"}`); err != nil {
		t.Fatal(err)
	}
	page := get("")
	expect(t, page, 200)
	if !strings.Contains(page.Body.String(), "Download QR code") || !strings.Contains(page.Body.String(), "file.txt") || !strings.Contains(page.Header().Get("Content-Type"), "text/html") {
		t.Fatal("missing share page")
	}
	if !strings.Contains(page.Body.String(), "Shared file · Home &amp; NAS") || !strings.Contains(page.Body.String(), `/api/branding/logo?v=custom`) {
		t.Fatal("share page did not use saved branding")
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
	if _, err := a.DB.Exec("UPDATE shares SET expires_at='2000-01-01' WHERE token=?", share.Token); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"", "?download=1", "?qr=1&download=1"} {
		expect(t, get(query), 404)
	}
}

func TestPasswordShareUnlockThroughTrustedProxy(t *testing.T) {
	a, _, user := setup(t)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1")
	t.Setenv("CORS_ORIGIN", "http://localhost:3000")
	write(t, a, "shared/file.txt", "shared contents")
	grant(t, a, user, "shared", "read")
	made := jsonRequest(a, user, "POST", "/api/shares", map[string]any{"path": "shared/file.txt", "never": true, "password": "share-password"}, a.CreateShare)
	expect(t, made, 200)
	var share struct{ Token string }
	if err := json.Unmarshal(made.Body.Bytes(), &share); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(`INSERT OR REPLACE INTO app_settings(key,value) VALUES('basic',?)`, `{"appName":"Private NAS","logoUrl":"https://example.com/logo.png"}`); err != nil {
		t.Fatal(err)
	}
	router := a.Router()
	page := httptest.NewRecorder()
	router.ServeHTTP(page, httptest.NewRequest("GET", "http://127.0.0.1:8080/s/"+share.Token, nil))
	expect(t, page, 200)
	if !strings.Contains(page.Body.String(), "Protected share · Private NAS") || !strings.Contains(page.Body.String(), `src="https://example.com/logo.png"`) {
		t.Fatal("password prompt did not use saved branding")
	}
	if page.Header().Get("Referrer-Policy") != "same-origin" {
		t.Fatal("password form must preserve its same-origin POST Origin header")
	}
	for _, test := range []struct {
		origin, password string
		status           int
	}{
		{"https://nas.example.com", "share-password", 200},
		{"https://nas.example.com", "wrong-password", 401},
		{"https://evil.example", "share-password", 403},
	} {
		req := httptest.NewRequest("POST", "http://127.0.0.1:8080/s/"+share.Token, strings.NewReader("password="+test.password))
		req.RemoteAddr = "127.0.0.1:1234"
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", test.origin)
		req.Header.Set("X-Forwarded-Host", "nas.example.com")
		req.Header.Set("X-Forwarded-Proto", "https")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		expect(t, w, test.status)
		if test.status == 200 && len(w.Result().Cookies()) != 1 {
			t.Fatal("missing share session")
		}
	}
}

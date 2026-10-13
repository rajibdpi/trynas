package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProtectedShareRequiresPasswordOnEveryVisit(t *testing.T) {
	a, _, user := setup(t)
	write(t, a, "shared/file.txt", "protected content")
	grant(t, a, user, "shared", "read")
	made := jsonRequest(a, user, "POST", "/api/shares", map[string]any{"path": "shared/file.txt", "never": true, "password": "share-password"}, a.CreateShare)
	expect(t, made, 200)
	var shared struct {
		Token string
		ID    int64
	}
	if err := json.Unmarshal(made.Body.Bytes(), &shared); err != nil {
		t.Fatal(err)
	}
	router := a.Router()
	serve := func(method, query, password string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/s/"+shared.Token+query, strings.NewReader("password="+password))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	unlock := func() *http.Cookie {
		w := serve("POST", "", "share-password", nil)
		expect(t, w, 200)
		if !strings.Contains(w.Body.String(), "event.persisted") {
			t.Fatal("unlocked page lacks browser-history relock protection")
		}
		if !strings.Contains(w.Body.String(), "Download file") {
			t.Fatal("unlocked share page missing")
		}
		cookies := w.Result().Cookies()
		if len(cookies) != 1 || !cookies[0].HttpOnly {
			t.Fatal("missing download grant")
		}
		if strings.Contains(w.Body.String(), cookies[0].Value) {
			t.Fatal("grant exposed in page or links")
		}
		return cookies[0]
	}
	first := unlock()
	// Another visitor's unlock must survive this browser reopening the link.
	other := unlock()
	expect(t, serve("GET", "?download=1", "", first), 200)
	expect(t, serve("GET", "?download=1", "", nil), 401)
	reopened := serve("GET", "", "", first)
	expect(t, reopened, 200)
	if !strings.Contains(reopened.Body.String(), "Unlock share") || strings.Contains(reopened.Body.String(), "Download file") {
		t.Fatal("reopen bypasses password")
	}
	if reopened.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("protected page can be cached")
	}
	cookies := reopened.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge != -1 {
		t.Fatal("old cookie not expired")
	}
	expect(t, serve("GET", "?download=1", "", first), 401)
	expect(t, serve("GET", "?qr=1", "", first), 401)
	expect(t, serve("GET", "?download=1", "", other), 200)
	expect(t, serve("POST", "", "wrong-password", first), 401)
	second := unlock()
	expect(t, serve("GET", "?download=1", "", second), 200)
	expect(t, serve("GET", "?qr=1", "", second), 200)
	// An already-unlocked browser cannot skip verification on another password POST.
	expect(t, serve("POST", "", "wrong-password", second), 401)
	expect(t, serve("GET", "?download=1", "", second), 401)
	var visits int
	if err := a.DB.QueryRow("SELECT visits FROM share_stats WHERE share_id=?", shared.ID).Scan(&visits); err != nil || visits != 3 {
		t.Fatalf("authorized visits not counted correctly: %d %v", visits, err)
	}
}

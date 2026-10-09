package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouterModuleWiring(t *testing.T) {
	a, _, _ := setup(t)
	router := a.Router()
	for _, route := range []struct {
		path   string
		status int
	}{
		{"/api/health", http.StatusOK},
		{"/api/me", http.StatusUnauthorized},
		{"/api/files", http.StatusUnauthorized},
		{"/api/users", http.StatusUnauthorized},
		{"/api/storage/providers", http.StatusUnauthorized},
		{"/api/missing", http.StatusNotFound},
		{"/s/missing", http.StatusNotFound},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, route.path, nil))
		if w.Code != route.status {
			t.Fatalf("%s: got %d, want %d: %s", route.path, w.Code, route.status, w.Body.String())
		}
	}

	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"test-admin","password":"test-password-long"}`))
	login.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, login)
	expect(t, w, http.StatusOK)
	var session *http.Cookie
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "trynas_session" {
			session = cookie
		}
	}
	if session == nil {
		t.Fatal("login did not return a session cookie")
	}
	for _, path := range []string{"/api/me", "/api/files", "/api/users", "/api/settings", "/api/storage/providers"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.AddCookie(session)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("authenticated %s: got %d: %s", path, w.Code, w.Body.String())
		}
	}
}

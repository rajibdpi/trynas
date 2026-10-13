package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecureCookieOnlyTrustsConfiguredProxy(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1")
	for _, test := range []struct {
		peer   string
		secure bool
	}{{"127.0.0.1:1234", true}, {"192.0.2.10:1234", false}} {
		r := httptest.NewRequest("GET", "http://nas.example/api/health", nil)
		r.RemoteAddr = test.peer
		r.Header.Set("X-Forwarded-Proto", "https")
		if SecureCookie(r) != test.secure {
			t.Fatal("incorrect forwarded HTTPS trust")
		}
		w := httptest.NewRecorder()
		SecurityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(w, r)
		if (w.Header().Get("Strict-Transport-Security") != "") != test.secure {
			t.Fatal("incorrect HSTS trust")
		}
	}
}

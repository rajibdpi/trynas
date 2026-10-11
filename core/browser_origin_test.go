package core

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrowserMutationForwardedOrigin(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "http://localhost:3000")
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1,::1")
	t.Setenv("COOKIE_SECURE", "false")
	for _, test := range []struct {
		name, peer, origin, host, proto string
		allowed                         bool
	}{
		{"LAN Next proxy", "127.0.0.1:1234", "http://192.168.0.16:3000", "192.168.0.16:3000", "http", true},
		{"HTTPS proxy", "127.0.0.1:1234", "https://nas.example.com", "nas.example.com", "https", true},
		{"untrusted peer", "192.168.0.20:1234", "https://nas.example.com", "nas.example.com", "https", false},
		{"cross origin", "127.0.0.1:1234", "https://evil.example", "nas.example.com", "https", false},
		{"ambiguous host", "127.0.0.1:1234", "https://nas.example.com", "evil.example,nas.example.com", "https", false},
		{"missing protocol", "127.0.0.1:1234", "https://nas.example.com", "nas.example.com", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://127.0.0.1:8080/s/token", strings.NewReader("password=secret"))
			r.RemoteAddr = test.peer
			r.Header.Set("Origin", test.origin)
			r.Header.Set("X-Forwarded-Host", test.host)
			r.Header.Set("X-Forwarded-Proto", test.proto)
			w := httptest.NewRecorder()
			if allowed := CheckBrowserMutation(w, r); allowed != test.allowed {
				t.Fatalf("allowed=%v, want %v: %s", allowed, test.allowed, w.Body.String())
			}
		})
	}
}

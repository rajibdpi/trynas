package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMultipleFrontendOrigins(t *testing.T) {
	a, _, _ := setup(t)
	t.Setenv("CORS_ORIGIN", "http://localhost:3000, https://nas.example.com\nhttps://other.example.com")
	for _, origin := range []string{"http://localhost:3000", "https://nas.example.com", "https://other.example.com", "https://evil.example.com"} {
		r := httptest.NewRequest("POST", "/api/test", strings.NewReader("{}"))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		a.Cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })).ServeHTTP(w, r)
		if origin == "https://evil.example.com" {
			expect(t, w, 403)
		} else {
			expect(t, w, 200)
			if w.Header().Get("Access-Control-Allow-Origin") != origin {
				t.Fatal("CORS must return the matching origin")
			}
		}
	}
}

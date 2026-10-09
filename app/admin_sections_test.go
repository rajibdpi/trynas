package app

import (
	"net/http"
	"testing"
)

func TestAdminSectionsGuard(t *testing.T) {
	a, admin, user := setup(t)
	for _, path := range []string{"/api/system", "/api/network", "/api/activity", "/api/users", "/api/permissions", "/api/settings", "/api/hardware", "/api/storage/providers"} {
		called := false
		handler := func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(200) }
		expect(t, request(a, user, "GET", path, nil, handler), 403)
		if called {
			t.Fatalf("user reached %s", path)
		}
		expect(t, request(a, admin, "GET", path, nil, handler), 200)
		if !called {
			t.Fatalf("admin blocked from %s", path)
		}
	}
}

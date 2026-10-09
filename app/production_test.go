package app

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProductionHeadersAndCookies(t *testing.T) {
	a, _, _ := setup(t)
	router := a.Router()
	for _, path := range []string{"/api/me", "/api/health", "/s/missing"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
			t.Fatalf("missing security headers for %s: %v", path, w.Header())
		}
	}
	for _, secure := range []bool{false, true} {
		t.Setenv("COOKIE_SECURE", "false")
		r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"test-admin","password":"test-password-long"}`))
		r.Header.Set("Content-Type", "application/json")
		if secure {
			r.TLS = &tls.ConnectionState{}
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		expect(t, w, 200)
		cookies := w.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Secure != secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
			t.Fatalf("incorrect login cookie: %v", cookies)
		}
		logout := httptest.NewRequest("POST", "/api/auth/logout", nil)
		logout.AddCookie(cookies[0])
		if secure {
			logout.TLS = &tls.ConnectionState{}
		}
		w = httptest.NewRecorder()
		router.ServeHTTP(w, logout)
		expect(t, w, 200)
		expired := w.Result().Cookies()[0]
		if expired.MaxAge != -1 || expired.Secure != secure || !expired.HttpOnly {
			t.Fatalf("incorrect logout cookie: %v", expired)
		}
	}
}

func TestQuotaUsesCoveringIndex(t *testing.T) {
	a, _, user := setup(t)
	if a.DB.Stats().MaxOpenConnections != 8 {
		t.Fatal("database pool is not bounded")
	}
	rows, err := a.DB.Query("EXPLAIN QUERY PLAN SELECT COALESCE(SUM(bytes),0) FROM file_owners WHERE user_id=?", user.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	indexed := false
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		indexed = indexed || strings.Contains(detail, "COVERING INDEX idx_file_owners_user_bytes")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !indexed {
		t.Fatal("quota totals are not using the covering index")
	}
}

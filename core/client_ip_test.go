package core

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPTrustedProxy(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1,::1")
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.25")
	if ClientIP(r) != "203.0.113.25" {
		t.Fatal("did not recover original client")
	}
	r.Header.Set("X-Forwarded-For", "198.51.100.1, 203.0.113.25")
	if ClientIP(r) != "203.0.113.25" {
		t.Fatal("accepted spoofed leftmost address")
	}
	r.RemoteAddr = "192.168.0.20:1234"
	if ClientIP(r) != "192.168.0.20" {
		t.Fatal("trusted header from untrusted client")
	}
}

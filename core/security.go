package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type AttemptBucket struct {
	count int
	until time.Time
}

type AttemptLimiter struct {
	Mu      sync.Mutex
	buckets map[string]AttemptBucket
}

func (l *AttemptLimiter) Allow(key string, limit int) bool {
	l.Mu.Lock()
	defer l.Mu.Unlock()
	now := time.Now()
	if l.buckets == nil {
		l.buckets = map[string]AttemptBucket{}
	}
	bucket, exists := l.buckets[key]
	if !exists || !now.Before(bucket.until) {
		for k, b := range l.buckets {
			if !now.Before(b.until) {
				delete(l.buckets, k)
			}
		}
		if len(l.buckets) >= 4096 {
			return false
		}
		bucket = AttemptBucket{until: now.Add(5 * time.Minute)}
	}
	if bucket.count >= limit {
		return false
	}
	bucket.count++
	l.buckets[key] = bucket
	return true
}

func PeerIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ConfiguredBrowserOrigin matches individual origins; credentialed CORS never uses a wildcard.
func ConfiguredBrowserOrigin(origin string) bool {
	for _, allowed := range strings.FieldsFunc(Env("CORS_ORIGIN", "http://localhost:3000"), func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
		if origin == strings.TrimSuffix(strings.TrimSpace(allowed), "/") {
			return true
		}
	}
	return false
}

// TrustedProxyIP identifies explicitly configured proxy peers.
func TrustedProxyIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, value := range strings.Split(Env("TRUSTED_PROXIES", ""), ",") {
		value = strings.TrimSpace(value)
		if configured := net.ParseIP(value); configured != nil && configured.Equal(ip) {
			return true
		}
		if _, network, err := net.ParseCIDR(value); err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

// ClientIP only accepts forwarding headers from explicitly trusted proxy peers.
func ClientIP(r *http.Request) string {
	peer := PeerIP(r)
	if !TrustedProxyIP(net.ParseIP(peer)) {
		return peer
	}
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(chain) - 1; i >= 0; i-- {
		candidate := net.ParseIP(strings.TrimSpace(chain[i]))
		if candidate == nil {
			return peer
		}
		if !TrustedProxyIP(candidate) {
			return candidate.String()
		}
	}
	return peer
}

func AllowedBrowserOrigin(r *http.Request, origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return false
	}
	normalized := parsed.Scheme + "://" + parsed.Host
	scheme := "http"
	if r.TLS != nil || Env("COOKIE_SECURE", "false") == "true" {
		scheme = "https"
	}
	if ConfiguredBrowserOrigin(normalized) || normalized == scheme+"://"+r.Host {
		return true
	}
	// Next.js and HTTPS reverse proxies forward the browser-facing origin.
	// Only explicitly trusted peers may supply these headers.
	if TrustedProxyIP(net.ParseIP(PeerIP(r))) {
		host := r.Header.Get("X-Forwarded-Host")
		proto := r.Header.Get("X-Forwarded-Proto")
		if host != "" && !strings.ContainsAny(host, ",/\\\r\n") && (proto == "http" || proto == "https") {
			return normalized == proto+"://"+host
		}
	}
	return false
}

func CheckBrowserMutation(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin != "" && !AllowedBrowserOrigin(r, origin) {
		Bad(w, "untrusted request origin", 403)
		return false
	}
	if origin == "" {
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			Bad(w, "cross-site request rejected", 403)
			return false
		}
		if referer := r.Header.Get("Referer"); referer != "" && !AllowedBrowserOrigin(r, referer) {
			Bad(w, "untrusted request origin", 403)
			return false
		}
	}
	if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/files/upload" && r.URL.Path != "/api/storage/upload" && r.URL.Path != "/api/settings/logo" && r.URL.Path != "/api/auth/logout" {
		kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || kind != "application/json" {
			Bad(w, "application/json required", 415)
			return false
		}
	}
	return true
}

var ErrArchiveLimit = errors.New("shared archive limit exceeded")

const MaxSharedArchiveBytes int64 = 1 << 30

const MaxSharedArchiveEntries = 10000

type ArchiveWriter struct {
	Target  io.Writer
	Ctx     context.Context
	Written int64
}

func (w *ArchiveWriter) Write(p []byte) (int, error) {
	if err := w.Ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > MaxSharedArchiveBytes-w.Written {
		return 0, fmt.Errorf("%w: archive exceeds 1 GiB", ErrArchiveLimit)
	}
	n, err := w.Target.Write(p)
	w.Written += int64(n)
	return n, err
}

// SecureCookie supports direct TLS and explicit HTTPS reverse-proxy deployments.
// Forwarded headers are intentionally not trusted without proxy configuration.
func SecureCookie(r *http.Request) bool {
	return r.TLS != nil || Env("COOKIE_SECURE", "false") == "true"
}

// SecurityHeaders prevents sensitive responses from being stored by browsers
// and intermediaries, without changing immutable frontend asset caching.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "same-origin")
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/s/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

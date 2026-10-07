package main

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

type attemptBucket struct {
	count int
	until time.Time
}
type attemptLimiter struct {
	mu      sync.Mutex
	buckets map[string]attemptBucket
}

func (l *attemptLimiter) allow(key string, limit int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if l.buckets == nil {
		l.buckets = map[string]attemptBucket{}
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
		bucket = attemptBucket{until: now.Add(5 * time.Minute)}
	}
	if bucket.count >= limit {
		return false
	}
	bucket.count++
	l.buckets[key] = bucket
	return true
}
func peerIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
func (a *App) allowAttempt(w http.ResponseWriter, r *http.Request, key string) bool {
	if !a.attempts.allow("ip:"+peerIP(r), 30) || !a.attempts.allow(key, 10) {
		w.Header().Set("Retry-After", "300")
		bad(w, "too many attempts; try again later", 429)
		return false
	}
	return true
}
func allowedBrowserOrigin(r *http.Request, origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return false
	}
	normalized := parsed.Scheme + "://" + parsed.Host
	scheme := "http"
	if r.TLS != nil || env("COOKIE_SECURE", "false") == "true" {
		scheme = "https"
	}
	return normalized == env("CORS_ORIGIN", "http://localhost:3000") || normalized == scheme+"://"+r.Host
}
func checkBrowserMutation(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin != "" && !allowedBrowserOrigin(r, origin) {
		bad(w, "untrusted request origin", 403)
		return false
	}
	if origin == "" {
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			bad(w, "cross-site request rejected", 403)
			return false
		}
		if referer := r.Header.Get("Referer"); referer != "" && !allowedBrowserOrigin(r, referer) {
			bad(w, "untrusted request origin", 403)
			return false
		}
	}
	if r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/files/upload" && r.URL.Path != "/api/storage/upload" && r.URL.Path != "/api/auth/logout" {
		kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || kind != "application/json" {
			bad(w, "application/json required", 415)
			return false
		}
	}
	return true
}

var errArchiveLimit = errors.New("shared archive limit exceeded")

const maxSharedArchiveBytes int64 = 1 << 30
const maxSharedArchiveEntries = 10000

type archiveWriter struct {
	target  io.Writer
	ctx     context.Context
	written int64
}

func (w *archiveWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > maxSharedArchiveBytes-w.written {
		return 0, fmt.Errorf("%w: archive exceeds 1 GiB", errArchiveLimit)
	}
	n, err := w.target.Write(p)
	w.written += int64(n)
	return n, err
}

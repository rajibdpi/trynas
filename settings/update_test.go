package settings

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rajibdpi/trynas/core"
)

type releaseTransport func(*http.Request) (*http.Response, error)

func (f releaseTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type updateLog struct{ calls int }

func (l *updateLog) Log(int, string, string, ...string) { l.calls++ }

func TestUpdatesInstallAndIntegrity(t *testing.T) {
	for _, test := range []struct {
		name, manifest, payload string
		fail                    bool
	}{
		{name: "verified", payload: "new binary"},
		{name: "tampered", payload: "tampered", fail: true},
		{name: "duplicate", manifest: "duplicate", fail: true},
		{name: "missing checksum", manifest: "missing", fail: true},
		{name: "download failure", payload: "http-error", fail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "trynas")
			if err := os.WriteFile(path, []byte("old binary"), 0750); err != nil {
				t.Fatal(err)
			}
			running, _ := hashExecutable(path)
			checksum := fmt.Sprintf("%x  trynas-linux-amd64\n", sha256.Sum256([]byte("new binary")))
			switch test.manifest {
			case "duplicate":
				checksum += checksum
			case "missing":
				checksum = ""
			}
			calls := []string{}
			client := &http.Client{Transport: releaseTransport(func(r *http.Request) (*http.Response, error) {
				calls = append(calls, r.URL.String())
				body, status := "", 200
				switch r.URL.String() {
				case "https://api.github.com/repos/rajibdpi/trynas/releases/latest":
					body = `{"tag_name":"v1.2.3"}`
				case releaseRepository + "/download/v1.2.3/SHA256SUMS":
					body = checksum
				case releaseRepository + "/download/v1.2.3/trynas-linux-amd64":
					body = test.payload
					if test.payload == "http-error" {
						status = 503
					}
				default:
					t.Fatalf("unexpected URL %s", r.URL)
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			log := &updateLog{}
			u := &releaseUpdater{client: client, path: path, asset: "trynas-linux-amd64", runningHash: running}
			a := &Module{Dependencies: log, updater: u}
			a.updateOnce.Do(func() {})
			request := func(method string) *httptest.ResponseRecorder {
				r := httptest.NewRequest(method, "/api/settings/updates", nil)
				r = r.WithContext(context.WithValue(r.Context(), core.UserKey, core.User{ID: 1, Role: "admin"}))
				w := httptest.NewRecorder()
				a.Updates(w, r)
				return w
			}
			if test.manifest == "" {
				w := request("GET")
				if w.Code != 200 || !strings.Contains(w.Body.String(), `"updateAvailable":true`) {
					t.Fatalf("check: %d %s", w.Code, w.Body)
				}
				if len(calls) != 2 {
					t.Fatal("check downloaded binary")
				}
				data, _ := os.ReadFile(path)
				if string(data) != "old binary" {
					t.Fatal("check changed executable")
				}
			}
			w := request("POST")
			if test.fail {
				if w.Code == 200 {
					t.Fatal("accepted invalid update")
				}
				data, _ := os.ReadFile(path)
				if string(data) != "old binary" {
					t.Fatal("failure replaced executable")
				}
				if log.calls != 0 {
					t.Fatal("failed update was logged as installed")
				}
			} else {
				if w.Code != 200 || !strings.Contains(w.Body.String(), `"restartRequired":true`) {
					t.Fatalf("install: %d %s", w.Code, w.Body)
				}
				data, _ := os.ReadFile(path)
				if string(data) != "new binary" {
					t.Fatal("binary not replaced")
				}
				info, _ := os.Stat(path)
				if info.Mode().Perm() != 0750 {
					t.Fatal("permissions changed")
				}
				if log.calls != 1 {
					t.Fatal("update not audited")
				}
				calls = nil
				w = request("POST")
				if w.Code != 200 || len(calls) != 2 || log.calls != 1 {
					t.Fatal("current binary was downloaded or installed again")
				}
			}
			staged, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".trynas-update-*"))
			if len(staged) != 0 {
				t.Fatal("staged files left behind")
			}
		})
	}
}

func TestUpdatesRequireAdministrator(t *testing.T) {
	for _, role := range []string{"", "user"} {
		for _, method := range []string{"GET", "POST"} {
			a := &Module{}
			r := httptest.NewRequest(method, "/api/settings/updates", nil)
			if role != "" {
				r = r.WithContext(context.WithValue(r.Context(), core.UserKey, core.User{Role: role}))
			}
			w := httptest.NewRecorder()
			a.Updates(w, r)
			if w.Code != 403 || a.updater != nil {
				t.Fatal("non-admin reached updater")
			}
		}
	}
}
func TestUpdatesRejectConcurrentOperation(t *testing.T) {
	u := &releaseUpdater{mu: sync.Mutex{}}
	a := &Module{updater: u}
	a.updateOnce.Do(func() {})
	u.mu.Lock()
	defer u.mu.Unlock()
	r := httptest.NewRequest("POST", "/api/settings/updates", nil)
	r = r.WithContext(context.WithValue(r.Context(), core.UserKey, core.User{Role: "admin"}))
	w := httptest.NewRecorder()
	a.Updates(w, r)
	if w.Code != 409 {
		t.Fatalf("concurrent update: %d", w.Code)
	}
}

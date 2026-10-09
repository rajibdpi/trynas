package app

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rajib/trynas/core"
	"github.com/rajib/trynas/database"
)

func TestAllowedLoginAndThrottle(t *testing.T) {
	a, _, _ := setup(t)
	login := func(password string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"Username":"test-admin","Password":"`+password+`"}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()
		a.Cors(http.HandlerFunc(a.Login)).ServeHTTP(w, r)
		return w
	}
	good := login("test-password-long")
	expect(t, good, 200)
	cookies := good.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatal("missing secure session attributes")
	}
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.AddCookie(cookies[0])
	w := httptest.NewRecorder()
	a.Auth(http.HandlerFunc(a.Me)).ServeHTTP(w, r)
	expect(t, w, 200)
	for i := 0; i < 9; i++ {
		expect(t, login("incorrect"), 401)
	}
	expect(t, login("incorrect"), 429)
}

func TestBrowserMutationOrigins(t *testing.T) {
	a, _, _ := setup(t)
	for _, test := range []struct {
		origin, kind string
		want         int
	}{{"http://localhost:3000", "application/json", 200}, {"http://example.com", "application/json", 200}, {"https://evil.example", "application/json", 403}, {"null", "application/json", 403}, {"http://localhost:3000", "text/plain", 415}} {
		r := httptest.NewRequest("POST", "http://example.com/api/settings", bytes.NewBufferString("{}"))
		r.Header.Set("Origin", test.origin)
		r.Header.Set("Content-Type", test.kind)
		w := httptest.NewRecorder()
		a.Cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })).ServeHTTP(w, r)
		expect(t, w, test.want)
	}
}

func TestLegacySessionMigrationAndExpiry(t *testing.T) {
	a, _, user := setup(t)
	for _, test := range []struct {
		token  string
		offset time.Duration
		status int
	}{{"future", time.Hour, 200}, {"past", -time.Hour, 401}} {
		when := time.Now().Add(test.offset).In(time.FixedZone("Dhaka", 6*3600))
		if _, err := a.DB.Exec("INSERT INTO sessions(token,user_id,expires_at) VALUES(?,?,?)", test.token, user.ID, when.String()+" m=+1"); err != nil {
			t.Fatal(err)
		}
		if err := a.NormalizeShareExpiries(); err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("GET", "/api/me", nil)
		r.AddCookie(&http.Cookie{Name: "trynas_session", Value: test.token})
		w := httptest.NewRecorder()
		a.Auth(http.HandlerFunc(a.Me)).ServeHTTP(w, r)
		expect(t, w, test.status)
	}
}

func TestPrivateDatabasePermissions(t *testing.T) {
	name := filepath.Join(t.TempDir(), "db.sqlite")
	if err := os.WriteFile(name, nil, 0644); err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, suffix := range []string{"", "-wal", "-shm"} {
		st, err := os.Stat(name + suffix)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm()&0077 != 0 {
			t.Fatalf("database readable by others: %s", suffix)
		}
	}
}

func TestArchiveBudgetAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	writer := &core.ArchiveWriter{Target: io.Discard, Ctx: ctx, Written: core.MaxSharedArchiveBytes - 1}
	if _, err := writer.Write([]byte("xx")); err == nil {
		t.Fatal("archive size limit bypassed")
	}
	cancel()
	if _, err := writer.Write([]byte("x")); err == nil {
		t.Fatal("archive cancellation ignored")
	}
	a, _, user := setup(t)
	a.ActiveArchives = 2
	w := httptest.NewRecorder()
	a.DownloadSharedItems(w, httptest.NewRequest("GET", "/s/test?download=1", nil), user, nil, "test.zip")
	expect(t, w, 429)
}

type unlockedArchiveRecorder struct {
	*httptest.ResponseRecorder
	app      *App
	unlocked bool
}

func (w *unlockedArchiveRecorder) Write(p []byte) (int, error) {
	if w.app.Mutation.TryLock() {
		w.unlocked = true
		w.app.Mutation.Unlock()
	}
	return w.ResponseRecorder.Write(p)
}

func TestArchiveDeliveryReleasesMutationLock(t *testing.T) {
	a, _, user := setup(t)
	write(t, a, "shared/file.txt", "contents")
	grant(t, a, user, "shared", "read")
	w := &unlockedArchiveRecorder{ResponseRecorder: httptest.NewRecorder(), app: a}
	a.DownloadSharedItems(w, httptest.NewRequest("GET", "/s/test?download=1", nil), user, []string{"shared"}, "shared.zip")
	if w.Code != 200 || !w.unlocked {
		t.Fatal("ZIP delivery held mutation lock")
	}
	file, err := os.Create(filepath.Join(a.Root, "shared/large"))
	if err != nil {
		t.Fatal(err)
	}
	if err = file.Truncate(core.MaxSharedArchiveBytes + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	file.Close()
	limited := httptest.NewRecorder()
	a.DownloadSharedItems(limited, httptest.NewRequest("GET", "/s/test?download=1", nil), user, []string{"shared/large"}, "large.zip")
	expect(t, limited, 413)
}

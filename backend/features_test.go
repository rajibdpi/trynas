package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func setup(t *testing.T) (*App, User, User) {
	t.Helper()
	t.Setenv("ADMIN_USER", "test-admin")
	t.Setenv("ADMIN_PASSWORD", "test-password")
	root := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "test.db")+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	a := &App{db: db, root: root, trash: filepath.Join(root, ".trynas-trash")}
	if err = os.MkdirAll(a.trash, 0700); err != nil {
		t.Fatal(err)
	}
	if err = a.migrate(); err != nil {
		t.Fatal(err)
	}
	if err = a.migrateV5(); err != nil {
		t.Fatal(err)
	}
	if err = a.migrateV5(); err != nil {
		t.Fatal("migration is not idempotent", err)
	}
	admin := User{ID: 1, Username: "test-admin", Role: "admin"}
	res, err := db.Exec("INSERT INTO users(username,password_hash,role) VALUES('member','unused','user')")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return a, admin, User{ID: int(id), Username: "member", Role: "user"}
}
func request(a *App, u User, method, url string, body io.Reader, h http.HandlerFunc) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, url, body)
	r = r.WithContext(context.WithValue(r.Context(), userKey, u))
	w := httptest.NewRecorder()
	a.guard(h).ServeHTTP(w, r)
	return w
}
func jsonRequest(a *App, u User, method, url string, body any, h http.HandlerFunc) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	return request(a, u, method, url, bytes.NewReader(data), h)
}
func grant(t *testing.T, a *App, u User, p, access string) {
	t.Helper()
	if _, err := a.db.Exec("INSERT INTO permissions(user_id,path,access) VALUES(?,?,?) ON CONFLICT(user_id,path) DO UPDATE SET access=excluded.access", u.ID, p, access); err != nil {
		t.Fatal(err)
	}
}
func write(t *testing.T, a *App, p, text string) {
	t.Helper()
	f := filepath.Join(a.root, p)
	if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func expect(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("want status %d, got %d: %s", status, w.Code, w.Body.String())
	}
}
func uploadRequest(a *App, u User, files map[string]string) *httptest.ResponseRecorder {
	var b bytes.Buffer
	m := multipart.NewWriter(&b)
	_ = m.WriteField("path", "shared")
	for name, data := range files {
		p, _ := m.CreateFormFile("files", name)
		_, _ = p.Write([]byte(data))
	}
	_ = m.Close()
	r := httptest.NewRequest("POST", "/api/files/upload", &b)
	r.Header.Set("Content-Type", m.FormDataContentType())
	r = r.WithContext(context.WithValue(r.Context(), userKey, u))
	w := httptest.NewRecorder()
	a.guard(http.HandlerFunc(a.uploadV5)).ServeHTTP(w, r)
	return w
}
func TestPermissionsAndRecursiveOperations(t *testing.T) {
	a, admin, u := setup(t)
	write(t, a, "shared/public.txt", "public")
	write(t, a, "shared/private/secret.txt", "secret")
	write(t, a, "outside.txt", "hidden")
	_ = os.Mkdir(filepath.Join(a.root, "dest"), 0755)
	expect(t, request(a, u, "GET", "/api/files", nil, a.listFiles), 403)
	grant(t, a, u, "shared", "read")
	grant(t, a, u, "shared/private", "none")
	if a.canBrowse(u, "") || a.can(u, "shared", true) || a.can(u, "shared/private/secret.txt", false) {
		t.Fatal("invalid ACL inheritance")
	}
	w := request(a, u, "GET", "/api/files?path=shared", nil, a.listFiles)
	expect(t, w, 200)
	if strings.Contains(w.Body.String(), "private") {
		t.Fatal("denied folder leaked")
	}
	w = request(a, u, "GET", "/api/files/search?q=txt&path=shared", nil, a.searchFiles)
	expect(t, w, 200)
	if strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "outside") {
		t.Fatal("search leaked denied files")
	}
	expect(t, request(a, u, "GET", "/api/files/download?path=shared/private/secret.txt", nil, a.download), 403)
	expect(t, request(a, u, "GET", "/api/files/download?path=shared", nil, a.download), 400)
	expect(t, jsonRequest(a, u, "POST", "/api/files/folder", map[string]any{"path": "shared/new"}, a.mkdir), 403)
	grant(t, a, u, "", "write")
	expect(t, jsonRequest(a, u, "POST", "/api/files/copy", map[string]any{"from": "shared", "to": "dest/copy"}, a.copyMoveV5(false)), 403)
	expect(t, jsonRequest(a, u, "POST", "/api/files/zip", map[string]any{"path": "shared"}, a.zipV5), 403)
	expect(t, request(a, u, "DELETE", "/api/files?path=shared", nil, a.trashV5), 403)
	// A destination subtree denial must also block recursive copies.
	grant(t, a, u, "shared/private", "read")
	grant(t, a, u, "dest/copy/private", "none")
	expect(t, jsonRequest(a, u, "POST", "/api/files/copy", map[string]any{"from": "shared", "to": "dest/copy"}, a.copyMoveV5(false)), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/files/copy", map[string]any{"from": "shared", "to": "dest/copy"}, a.copyMoveV5(false)), 200)
}
func TestQuotaAtomicUploadAndOwnership(t *testing.T) {
	a, _, u := setup(t)
	_ = os.Mkdir(filepath.Join(a.root, "shared"), 0755)
	grant(t, a, u, "", "write")
	_, _ = a.db.Exec("INSERT INTO user_limits(user_id,quota_bytes) VALUES(?,5)", u.ID)
	expect(t, uploadRequest(a, u, map[string]string{"a.txt": "1234", "b.txt": "56"}), 413)
	if _, err := os.Stat(filepath.Join(a.root, "shared/a.txt")); !os.IsNotExist(err) {
		t.Fatal("failed upload installed a file")
	}
	used, _, err := a.usage(u.ID)
	if err != nil || used != 0 {
		t.Fatal("failed upload changed quota")
	}
	expect(t, uploadRequest(a, u, map[string]string{"a.txt": "1234"}), 200)
	expect(t, uploadRequest(a, u, map[string]string{"a.txt": "x"}), 409)
	expect(t, jsonRequest(a, u, "POST", "/api/files/copy", map[string]any{"from": "shared/a.txt", "to": "shared/copy.txt"}, a.copyMoveV5(false)), 413)
	expect(t, jsonRequest(a, u, "POST", "/api/files/rename", map[string]any{"from": "shared/a.txt", "to": "shared/renamed.txt"}, a.renameV5), 200)
	expect(t, request(a, u, "DELETE", "/api/files?path=shared/renamed.txt", nil, a.trashV5), 200)
	used, _, _ = a.usage(u.ID)
	if used != 4 {
		t.Fatalf("trash must retain quota, got %d", used)
	}
	var id int
	_ = a.db.QueryRow("SELECT id FROM trash WHERE user_id=?", u.ID).Scan(&id)
	expect(t, jsonRequest(a, u, "POST", "/api/trash/restore", map[string]any{"id": id}, a.restoreV5), 200)
	used, _, _ = a.usage(u.ID)
	if used != 4 {
		t.Fatal("restore changed quota")
	}
	expect(t, request(a, u, "DELETE", "/api/files?path=shared/renamed.txt", nil, a.trashV5), 200)
	var idText string
	_ = a.db.QueryRow("SELECT id FROM trash WHERE user_id=?", u.ID).Scan(&idText)
	expect(t, request(a, u, "DELETE", "/api/trash?id="+idText, nil, a.purgeV5), 200)
	used, _, _ = a.usage(u.ID)
	if used != 0 {
		t.Fatal("purge did not free quota")
	}
}
func TestSafePathsSymlinksAndBulkResults(t *testing.T) {
	a, _, u := setup(t)
	grant(t, a, u, "", "write")
	write(t, a, "a.txt", "hello")
	for _, p := range []string{"../secret", "/etc/passwd", ".trynas-trash/x", "x/../../secret", "x\\y"} {
		if _, err := safe(a.root, p); err == nil {
			t.Errorf("accepted invalid path %q", p)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(a.root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := safe(a.root, "link/file"); err == nil {
		t.Fatal("followed symlink")
	}
	expect(t, jsonRequest(a, u, "POST", "/api/files/copy", map[string]any{"from": "link", "to": "copy"}, a.copyMoveV5(false)), 403)
	w := jsonRequest(a, u, "POST", "/api/files/bulk", map[string]any{"action": "trash", "paths": []string{"a.txt", "missing.txt"}}, a.bulk)
	expect(t, w, 200)
	var result struct{ Results []struct{ OK bool } }
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if len(result.Results) != 2 || !result.Results[0].OK || result.Results[1].OK {
		t.Fatal("bulk must report per-item partial failure", w.Body.String())
	}
}
func TestAdminUserValidationSessionRevocationAndShares(t *testing.T) {
	a, admin, u := setup(t)
	expect(t, request(a, u, "GET", "/api/users", nil, a.users), 403)
	expect(t, jsonRequest(a, u, "POST", "/api/permissions", map[string]any{"userId": u.ID, "path": "", "access": "write"}, a.savePermission), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/users", map[string]any{"id": admin.ID, "username": admin.Username, "role": "user", "quota": 0}, a.saveUser), 409)
	_, _ = a.db.Exec("INSERT INTO sessions(token,user_id,expires_at) VALUES('old',?,?)", u.ID, time.Now().Add(time.Hour))
	expect(t, jsonRequest(a, admin, "POST", "/api/users", map[string]any{"id": u.ID, "username": u.Username, "role": "user", "password": "new-password", "quota": 100}, a.saveUser), 200)
	var n int
	_ = a.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id=?", u.ID).Scan(&n)
	if n != 0 {
		t.Fatal("password reset did not revoke sessions")
	}
	write(t, a, "shared/public.txt", "public")
	grant(t, a, u, "shared", "read")
	w := jsonRequest(a, u, "POST", "/api/shares", map[string]any{"path": "shared/public.txt", "days": 7}, a.createShare)
	expect(t, w, 200)
	var share struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &share)
	router := chi.NewRouter()
	router.Get("/s/{token}", a.publicShare)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/s/"+share.Token, nil))
	expect(t, w, 200)
	grant(t, a, u, "shared", "none")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/s/"+share.Token, nil))
	expect(t, w, 404)
}

func TestConcurrentUploadsCannotExceedQuota(t *testing.T) {
	a, _, u := setup(t)
	_ = os.Mkdir(filepath.Join(a.root, "shared"), 0755)
	grant(t, a, u, "shared", "write")
	_, _ = a.db.Exec("INSERT INTO user_limits(user_id,quota_bytes) VALUES(?,5)", u.ID)
	results := make(chan int, 2)
	for _, name := range []string{"one.txt", "two.txt"} {
		go func(name string) { results <- uploadRequest(a, u, map[string]string{name: "1234"}).Code }(name)
	}
	first, second := <-results, <-results
	if !((first == 200 && second == 413) || (first == 413 && second == 200)) {
		t.Fatalf("quota race: %d %d", first, second)
	}
	used, _, err := a.usage(u.ID)
	if err != nil || used != 4 {
		t.Fatalf("quota race usage=%d err=%v", used, err)
	}
}
func TestSmartNonzeroExitStillReportsFailedHealth(t *testing.T) {
	a, admin, _ := setup(t)
	bin := t.TempDir()
	script := `#!/bin/sh
case "$1" in
 --scan-open) echo '{"devices":[{"name":"/dev/test","type":"sat"}]}';;
 *) echo '{"model_name":"Fixture Disk","smart_status":{"passed":false},"temperature":{"current":47},"power_on_time":{"hours":123}}';exit 8;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "smartctl"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	w := request(a, admin, "GET", "/api/hardware", nil, a.hardware)
	expect(t, w, 200)
	var result struct {
		Available bool
		Disks     []struct {
			Health      string
			Temperature int
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Available || len(result.Disks) != 1 || result.Disks[0].Health != "failed" || result.Disks[0].Temperature != 47 {
		t.Fatal("invalid SMART error-bit interpretation", w.Body.String())
	}
	expect(t, request(a, User{ID: 999, Role: "user"}, "GET", "/api/hardware", nil, a.hardware), 403)
}

func TestSmartPermissionDiagnosticsAndRefresh(t *testing.T) {
	a, admin, _ := setup(t)
	bin := t.TempDir()
	script := `#!/bin/sh
case "$1" in
 --scan-open) echo '{"devices":[{"name":"/dev/test","type":"scsi"}]}';;
 *) echo '{"smartctl":{"messages":[{"string":"Smartctl open device: /dev/test failed: Permission denied","severity":"error"}],"exit_status":2}}';exit 2;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "smartctl"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SMART_USE_SUDO", "false")
	a.smartCached = map[string]any{"message": "stale"}
	a.smartAt = time.Now()
	w := request(a, admin, "GET", "/api/hardware", nil, a.hardware)
	expect(t, w, 200)
	if !strings.Contains(w.Body.String(), "stale") {
		t.Fatal("cache missed")
	}
	w = request(a, admin, "GET", "/api/hardware?refresh=true", nil, a.hardware)
	expect(t, w, 200)
	var result struct {
		Available, AccessDenied bool
		Disks                   []struct {
			Message      string
			AccessDenied bool
			ExitStatus   int
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Available || !result.AccessDenied || len(result.Disks) != 1 || !result.Disks[0].AccessDenied || !strings.Contains(result.Disks[0].Message, "Permission denied") || result.Disks[0].ExitStatus != 2 {
		t.Fatal("missing SMART diagnostics", w.Body.String())
	}
}

func TestNewUserHomeFolder(t *testing.T) {
	a, admin, _ := setup(t)
	w := jsonRequest(a, admin, "POST", "/api/users", map[string]any{"username": "alice", "password": "test-password", "role": "user", "quota": 1024}, a.saveUser)
	expect(t, w, 200)
	var result struct{ ID int }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(a.root, "alice"))
	if err != nil || !st.IsDir() {
		t.Fatal("home folder was not created", err)
	}
	u := User{ID: result.ID, Username: "alice", Role: "user"}
	if !a.can(u, "alice/file.txt", true) || a.can(u, "other/file.txt", false) {
		t.Fatal("incorrect automatic home permission")
	}
	expect(t, request(a, u, "GET", "/api/files?path=alice", nil, a.listFiles), 200)
	// Username edits preserve the existing folder and its access.
	expect(t, jsonRequest(a, admin, "POST", "/api/users", map[string]any{"id": u.ID, "username": "alice-renamed", "role": "user", "quota": 1024}, a.saveUser), 200)
	if !a.can(u, "alice/file.txt", true) {
		t.Fatal("editing user removed home access")
	}
	if _, err = os.Stat(filepath.Join(a.root, "alice-renamed")); !os.IsNotExist(err) {
		t.Fatal("editing unexpectedly created a second home")
	}
}
func TestNewUserHomeConflictsAndInvalidNames(t *testing.T) {
	a, admin, _ := setup(t)
	write(t, a, "existing/private.txt", "keep")
	for _, name := range []string{"existing", "../escape", "nested/user", ".trynas-private", "."} {
		w := jsonRequest(a, admin, "POST", "/api/users", map[string]any{"username": name, "password": "test-password", "role": "user", "quota": 0}, a.saveUser)
		if w.Code != 400 && w.Code != 409 {
			t.Fatalf("accepted home name %q: %d", name, w.Code)
		}
		var n int
		_ = a.db.QueryRow("SELECT COUNT(*) FROM users WHERE username=?", name).Scan(&n)
		if n != 0 {
			t.Fatal("failed creation left an account")
		}
	}
	data, err := os.ReadFile(filepath.Join(a.root, "existing/private.txt"))
	if err != nil || string(data) != "keep" {
		t.Fatal("existing folder changed")
	}
}

func TestRootRequiresExplicitGrant(t *testing.T) {
	a, admin, _ := setup(t)
	w := jsonRequest(a, admin, "POST", "/api/users", map[string]any{"username": "private-user", "password": "test-password", "role": "user", "quota": 0}, a.saveUser)
	expect(t, w, 200)
	var result struct{ ID int }
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	u := User{ID: result.ID, Username: "private-user", Role: "user"}
	expect(t, request(a, u, "GET", "/api/files", nil, a.listFiles), 403)
	expect(t, request(a, u, "GET", "/api/files/search?q=txt", nil, a.searchFiles), 403)
	expect(t, request(a, u, "GET", "/api/files?path=private-user", nil, a.listFiles), 200)
	if a.profile(u)["home"] != "private-user" {
		t.Fatal("incorrect home")
	}
	grant(t, a, u, "", "read")
	expect(t, request(a, u, "GET", "/api/files", nil, a.listFiles), 200)
	if a.can(u, "", true) || !a.can(u, "private-user", true) {
		t.Fatal("root grant changed write permissions")
	}
}

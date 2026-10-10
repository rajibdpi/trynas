package app

import (
	"bytes"
	"context"
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

	chi "github.com/go-chi/chi/v5"
	"github.com/rajibdpi/trynas/core"
	"github.com/rajibdpi/trynas/database"
)

func setup(t *testing.T) (*App, core.User, core.User) {
	t.Helper()
	t.Setenv("ADMIN_USER", "test-admin")
	t.Setenv("ADMIN_PASSWORD", "test-password-long")
	root := t.TempDir()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	a := New(db, root, filepath.Join(root, ".trynas-trash"))
	if err = os.MkdirAll(a.Trash, 0700); err != nil {
		t.Fatal(err)
	}
	if err = a.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err = a.MigrateV5(); err != nil {
		t.Fatal(err)
	}
	if err = a.MigrateV5(); err != nil {
		t.Fatal("migration is not idempotent", err)
	}
	admin := core.User{ID: 1, Username: "test-admin", Role: "admin"}
	res, err := db.Exec("INSERT INTO users(username,password_hash,role) VALUES('member','unused','user')")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return a, admin, core.User{ID: int(id), Username: "member", Role: "user"}
}

func request(a *App, u core.User, method, url string, body io.Reader, h http.HandlerFunc) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, url, body)
	r = r.WithContext(context.WithValue(r.Context(), core.UserKey, u))
	w := httptest.NewRecorder()
	a.Guard(h).ServeHTTP(w, r)
	return w
}

func jsonRequest(a *App, u core.User, method, url string, body any, h http.HandlerFunc) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	return request(a, u, method, url, bytes.NewReader(data), h)
}

func grant(t *testing.T, a *App, u core.User, p, access string) {
	t.Helper()
	if _, err := a.DB.Exec("INSERT INTO permissions(user_id,path,access) VALUES(?,?,?) ON CONFLICT(user_id,path) DO UPDATE SET access=excluded.access", u.ID, p, access); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, a *App, p, text string) {
	t.Helper()
	f := filepath.Join(a.Root, p)
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

func uploadRequest(a *App, u core.User, files map[string]string) *httptest.ResponseRecorder {
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
	r = r.WithContext(context.WithValue(r.Context(), core.UserKey, u))
	w := httptest.NewRecorder()
	a.Guard(http.HandlerFunc(a.UploadV5)).ServeHTTP(w, r)
	return w
}

func TestPermissionsAndRecursiveOperations(t *testing.T) {
	a, admin, u := setup(t)
	write(t, a, "shared/public.txt", "public")
	write(t, a, "shared/private/secret.txt", "secret")
	write(t, a, "outside.txt", "hidden")
	_ = os.Mkdir(filepath.Join(a.Root, "dest"), 0755)
	expect(t, request(a, u, "GET", "/api/files", nil, a.ListFiles), 403)
	grant(t, a, u, "shared", "read")
	grant(t, a, u, "shared/private", "none")
	if a.CanBrowse(u, "") || a.Can(u, "shared", true) || a.Can(u, "shared/private/secret.txt", false) {
		t.Fatal("invalid ACL inheritance")
	}
	w := request(a, u, "GET", "/api/files?path=shared", nil, a.ListFiles)
	expect(t, w, 200)
	if strings.Contains(w.Body.String(), "private") {
		t.Fatal("denied folder leaked")
	}
	w = request(a, u, "GET", "/api/files/search?q=txt&path=shared", nil, a.SearchFiles)
	expect(t, w, 200)
	if strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "outside") {
		t.Fatal("search leaked denied files")
	}
	expect(t, request(a, u, "GET", "/api/files/download?path=shared/private/secret.txt", nil, a.Download), 403)
	expect(t, request(a, u, "GET", "/api/files/download?path=shared", nil, a.Download), 400)
	expect(t, jsonRequest(a, u, "POST", "/api/files/folder", map[string]any{"path": "shared/new"}, a.Mkdir), 403)
	grant(t, a, u, "", "write")
	expect(t, jsonRequest(a, u, "POST", "/api/files/copy", map[string]any{"from": "shared", "to": "dest/copy"}, a.CopyMoveV5(false)), 403)
	expect(t, jsonRequest(a, u, "POST", "/api/files/zip", map[string]any{"path": "shared"}, a.ZipV5), 403)
	expect(t, request(a, u, "DELETE", "/api/files?path=shared", nil, a.TrashV5), 403)
	// A destination subtree denial must also block recursive copies.
	grant(t, a, u, "shared/private", "read")
	grant(t, a, u, "dest/copy/private", "none")
	expect(t, jsonRequest(a, u, "POST", "/api/files/copy", map[string]any{"from": "shared", "to": "dest/copy"}, a.CopyMoveV5(false)), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/files/copy", map[string]any{"from": "shared", "to": "dest/copy"}, a.CopyMoveV5(false)), 200)
}

func TestQuotaAtomicUploadAndOwnership(t *testing.T) {
	a, _, u := setup(t)
	_ = os.Mkdir(filepath.Join(a.Root, "shared"), 0755)
	grant(t, a, u, "", "write")
	_, _ = a.DB.Exec("INSERT INTO user_limits(user_id,quota_bytes) VALUES(?,5)", u.ID)
	expect(t, uploadRequest(a, u, map[string]string{"a.txt": "1234", "b.txt": "56"}), 413)
	if _, err := os.Stat(filepath.Join(a.Root, "shared/a.txt")); !os.IsNotExist(err) {
		t.Fatal("failed upload installed a file")
	}
	used, _, err := a.Usage(u.ID)
	if err != nil || used != 0 {
		t.Fatal("failed upload changed quota")
	}
	expect(t, uploadRequest(a, u, map[string]string{"a.txt": "1234"}), 200)
	expect(t, uploadRequest(a, u, map[string]string{"a.txt": "x"}), 409)
	expect(t, jsonRequest(a, u, "POST", "/api/files/copy", map[string]any{"from": "shared/a.txt", "to": "shared/copy.txt"}, a.CopyMoveV5(false)), 413)
	expect(t, jsonRequest(a, u, "POST", "/api/files/rename", map[string]any{"from": "shared/a.txt", "to": "shared/renamed.txt"}, a.RenameV5), 200)
	expect(t, request(a, u, "DELETE", "/api/files?path=shared/renamed.txt", nil, a.TrashV5), 200)
	used, _, _ = a.Usage(u.ID)
	if used != 4 {
		t.Fatalf("trash must retain quota, got %d", used)
	}
	var id int
	_ = a.DB.QueryRow("SELECT id FROM trash WHERE user_id=?", u.ID).Scan(&id)
	expect(t, jsonRequest(a, u, "POST", "/api/trash/restore", map[string]any{"id": id}, a.RestoreV5), 200)
	used, _, _ = a.Usage(u.ID)
	if used != 4 {
		t.Fatal("restore changed quota")
	}
	expect(t, request(a, u, "DELETE", "/api/files?path=shared/renamed.txt", nil, a.TrashV5), 200)
	var idText string
	_ = a.DB.QueryRow("SELECT id FROM trash WHERE user_id=?", u.ID).Scan(&idText)
	expect(t, request(a, u, "DELETE", "/api/trash?id="+idText, nil, a.PurgeV5), 200)
	used, _, _ = a.Usage(u.ID)
	if used != 0 {
		t.Fatal("purge did not free quota")
	}
}

func TestSafePathsSymlinksAndBulkResults(t *testing.T) {
	a, _, u := setup(t)
	grant(t, a, u, "", "write")
	write(t, a, "a.txt", "hello")
	for _, p := range []string{"../secret", "/etc/passwd", ".trynas-trash/x", "x/../../secret", "x\\y"} {
		if _, err := core.Safe(a.Root, p); err == nil {
			t.Errorf("accepted invalid path %q", p)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(a.Root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := core.Safe(a.Root, "link/file"); err == nil {
		t.Fatal("followed symlink")
	}
	expect(t, jsonRequest(a, u, "POST", "/api/files/copy", map[string]any{"from": "link", "to": "copy"}, a.CopyMoveV5(false)), 403)
	w := jsonRequest(a, u, "POST", "/api/files/bulk", map[string]any{"action": "trash", "paths": []string{"a.txt", "missing.txt"}}, a.Bulk)
	expect(t, w, 200)
	var result struct{ Results []struct{ OK bool } }
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if len(result.Results) != 2 || !result.Results[0].OK || result.Results[1].OK {
		t.Fatal("bulk must report per-item partial failure", w.Body.String())
	}
}

func TestAdminUserValidationSessionRevocationAndShares(t *testing.T) {
	a, admin, u := setup(t)
	expect(t, request(a, u, "GET", "/api/users", nil, a.Users), 403)
	expect(t, jsonRequest(a, u, "POST", "/api/permissions", map[string]any{"userId": u.ID, "path": "", "access": "write"}, a.SavePermission), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/users", map[string]any{"id": admin.ID, "username": admin.Username, "role": "user", "quota": 0}, a.SaveUser), 409)
	_, _ = a.DB.Exec("INSERT INTO sessions(token,user_id,expires_at) VALUES('old',?,?)", u.ID, time.Now().Add(time.Hour))
	expect(t, jsonRequest(a, admin, "POST", "/api/users", map[string]any{"id": u.ID, "username": u.Username, "role": "user", "password": "new-password", "quota": 100}, a.SaveUser), 200)
	var n int
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id=?", u.ID).Scan(&n)
	if n != 0 {
		t.Fatal("password reset did not revoke sessions")
	}
	write(t, a, "shared/public.txt", "public")
	grant(t, a, u, "shared", "read")
	w := jsonRequest(a, u, "POST", "/api/shares", map[string]any{"path": "shared/public.txt", "days": 7}, a.CreateShare)
	expect(t, w, 200)
	var share struct{ Token string }
	_ = json.Unmarshal(w.Body.Bytes(), &share)
	router := chi.NewRouter()
	router.Get("/s/{token}", a.PublicShare)
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
	_ = os.Mkdir(filepath.Join(a.Root, "shared"), 0755)
	grant(t, a, u, "shared", "write")
	_, _ = a.DB.Exec("INSERT INTO user_limits(user_id,quota_bytes) VALUES(?,5)", u.ID)
	results := make(chan int, 2)
	for _, name := range []string{"one.txt", "two.txt"} {
		go func(name string) { results <- uploadRequest(a, u, map[string]string{name: "1234"}).Code }(name)
	}
	first, second := <-results, <-results
	if !((first == 200 && second == 413) || (first == 413 && second == 200)) {
		t.Fatalf("quota race: %d %d", first, second)
	}
	used, _, err := a.Usage(u.ID)
	if err != nil || used != 4 {
		t.Fatalf("quota race usage=%d err=%v", used, err)
	}
}

func TestNewUserHomeFolder(t *testing.T) {
	a, admin, _ := setup(t)
	w := jsonRequest(a, admin, "POST", "/api/users", map[string]any{"username": "alice", "password": "test-password", "role": "user", "quota": 1024}, a.SaveUser)
	expect(t, w, 200)
	var result struct{ ID int }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(a.Root, "alice"))
	if err != nil || !st.IsDir() {
		t.Fatal("home folder was not created", err)
	}
	u := core.User{ID: result.ID, Username: "alice", Role: "user"}
	if !a.Can(u, "alice/file.txt", true) || a.Can(u, "other/file.txt", false) {
		t.Fatal("incorrect automatic home permission")
	}
	expect(t, request(a, u, "GET", "/api/files?path=alice", nil, a.ListFiles), 200)
	// Username edits preserve the existing folder and its access.
	expect(t, jsonRequest(a, admin, "POST", "/api/users", map[string]any{"id": u.ID, "username": "alice-renamed", "role": "user", "quota": 1024}, a.SaveUser), 200)
	if !a.Can(u, "alice/file.txt", true) {
		t.Fatal("editing user removed home access")
	}
	if _, err = os.Stat(filepath.Join(a.Root, "alice-renamed")); !os.IsNotExist(err) {
		t.Fatal("editing unexpectedly created a second home")
	}
}

func TestNewUserHomeConflictsAndInvalidNames(t *testing.T) {
	a, admin, _ := setup(t)
	write(t, a, "existing/private.txt", "keep")
	for _, name := range []string{"existing", "../escape", "nested/user", ".trynas-private", "."} {
		w := jsonRequest(a, admin, "POST", "/api/users", map[string]any{"username": name, "password": "test-password", "role": "user", "quota": 0}, a.SaveUser)
		if w.Code != 400 && w.Code != 409 {
			t.Fatalf("accepted home name %q: %d", name, w.Code)
		}
		var n int
		_ = a.DB.QueryRow("SELECT COUNT(*) FROM users WHERE username=?", name).Scan(&n)
		if n != 0 {
			t.Fatal("failed creation left an account")
		}
	}
	data, err := os.ReadFile(filepath.Join(a.Root, "existing/private.txt"))
	if err != nil || string(data) != "keep" {
		t.Fatal("existing folder changed")
	}
}

func TestRootRequiresExplicitGrant(t *testing.T) {
	a, admin, _ := setup(t)
	w := jsonRequest(a, admin, "POST", "/api/users", map[string]any{"username": "private-user", "password": "test-password", "role": "user", "quota": 0}, a.SaveUser)
	expect(t, w, 200)
	var result struct{ ID int }
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	u := core.User{ID: result.ID, Username: "private-user", Role: "user"}
	expect(t, request(a, u, "GET", "/api/files", nil, a.ListFiles), 403)
	expect(t, request(a, u, "GET", "/api/files/search?q=txt", nil, a.SearchFiles), 403)
	expect(t, request(a, u, "GET", "/api/files?path=private-user", nil, a.ListFiles), 200)
	if a.Profile(u)["home"] != "private-user" {
		t.Fatal("incorrect home")
	}
	grant(t, a, u, "", "read")
	expect(t, request(a, u, "GET", "/api/files", nil, a.ListFiles), 200)
	if a.Can(u, "", true) || !a.Can(u, "private-user", true) {
		t.Fatal("root grant changed write permissions")
	}
}

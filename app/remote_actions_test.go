package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rajibdpi/trynas/core"
)

func TestRemoteFileActions(t *testing.T) {
	a, admin, user := setup(t)
	remote := t.TempDir()
	other := t.TempDir()
	if err := a.SaveConnections([]core.StorageConnection{{ID: "remote", Name: "Remote", Provider: "local", Root: remote, Attached: true, Options: map[string]string{}}, {ID: "other", Name: "Other", Provider: "local", Root: other, Attached: true, Options: map[string]string{}}}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(remote, "folder", "empty"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remote, "folder", "a.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	action := func(payload map[string]any, status int) {
		t.Helper()
		expect(t, jsonRequest(a, admin, "POST", "/api/storage/actions", payload, a.FileActions), status)
	}
	expect(t, jsonRequest(a, user, "POST", "/api/storage/actions", map[string]any{"action": "copy", "storage": "remote", "paths": []string{"folder"}}, a.FileActions), 403)
	action(map[string]any{"action": "copy", "storage": "remote", "destinationStorage": "", "paths": []string{"folder"}}, 200)
	if b, e := os.ReadFile(filepath.Join(a.Root, "folder/a.txt")); e != nil || string(b) != "hello" {
		t.Fatal("remote to local copy failed", e)
	}
	var owned int64
	if e := a.DB.QueryRow("SELECT bytes FROM file_owners WHERE path='folder/a.txt' AND user_id=?", admin.ID).Scan(&owned); e != nil || owned != 5 {
		t.Fatal("local ownership missing", e)
	}
	action(map[string]any{"action": "copy", "storage": "remote", "paths": []string{"folder"}}, 409)
	action(map[string]any{"action": "copy", "storage": "", "destinationStorage": "other", "paths": []string{"folder"}}, 200)
	if b, e := os.ReadFile(filepath.Join(other, "folder/a.txt")); e != nil || string(b) != "hello" {
		t.Fatal("local to remote failed", e)
	}
	action(map[string]any{"action": "move", "storage": "other", "destinationStorage": "remote", "destination": "folder", "paths": []string{"folder"}}, 200)
	if _, e := os.Stat(filepath.Join(other, "folder")); !os.IsNotExist(e) {
		t.Fatal("move retained source")
	}
	action(map[string]any{"action": "rename", "storage": "remote", "paths": []string{"folder/folder"}, "name": "renamed"}, 200)
	action(map[string]any{"action": "zip", "storage": "remote", "paths": []string{"folder/renamed"}, "destination": "folder", "name": "backup.zip"}, 200)
	z, e := zip.OpenReader(filepath.Join(remote, "folder/backup.zip"))
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	found := false
	for _, f := range z.File {
		if f.Name == "renamed/a.txt" {
			found = true
		}
	}
	if !found {
		t.Fatal("zip content missing")
	}
	action(map[string]any{"action": "trash", "storage": "remote", "paths": []string{"folder/renamed"}}, 200)
	if _, e := os.Stat(filepath.Join(remote, "folder/renamed")); !os.IsNotExist(e) {
		t.Fatal("trash retained source")
	}
	var id int64
	if e := a.DB.QueryRow("SELECT id FROM remote_trash WHERE original_path='folder/renamed'").Scan(&id); e != nil {
		t.Fatal(e)
	}
	action(map[string]any{"action": "restore", "storage": "remote", "trashID": id}, 200)
	if _, e := os.Stat(filepath.Join(remote, "folder/renamed/a.txt")); e != nil {
		t.Fatal("restore failed", e)
	}
	action(map[string]any{"action": "trash", "storage": "remote", "paths": []string{"folder/renamed"}}, 200)
	if e := a.DB.QueryRow("SELECT id FROM remote_trash WHERE original_path='folder/renamed'").Scan(&id); e != nil {
		t.Fatal(e)
	}
	action(map[string]any{"action": "purge", "storage": "remote", "trashID": id}, 400)
	action(map[string]any{"action": "purge", "storage": "remote", "trashID": id, "confirm": true}, 200)
	for _, p := range []string{"../outside", ".trynas-trash", ""} {
		action(map[string]any{"action": "copy", "storage": "remote", "destinationStorage": "other", "paths": []string{p}}, 400)
	}
	action(map[string]any{"action": "copy", "storage": "remote", "destinationStorage": "remote", "destination": "folder", "paths": []string{"folder"}}, 400)
}

func TestRemoteShareLifecycle(t *testing.T) {
	a, admin, user := setup(t)
	remote := t.TempDir()
	if e := os.WriteFile(filepath.Join(remote, "hello.txt"), []byte("remote shared"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := a.SaveConnections([]core.StorageConnection{{ID: "remote", Provider: "local", Root: remote, Attached: true, Options: map[string]string{}}}); e != nil {
		t.Fatal(e)
	}
	payload := map[string]any{"storage": "remote", "paths": []string{"hello.txt"}, "days": 7}
	expect(t, jsonRequest(a, user, "POST", "/api/shares", payload, a.CreateShare), 403)
	w := jsonRequest(a, admin, "POST", "/api/shares", payload, a.CreateShare)
	expect(t, w, 200)
	var shared struct {
		Token string
		ID    int
	}
	if e := json.Unmarshal(w.Body.Bytes(), &shared); e != nil {
		t.Fatal(e)
	}
	get := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		a.Router().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/s/"+shared.Token+"?download=1", nil))
		return w
	}
	w = get()
	expect(t, w, 200)
	if w.Body.String() != "remote shared" {
		t.Fatal("remote share download failed", w.Body.String())
	}
	listed := request(a, admin, "GET", "/api/shares", nil, a.Shares)
	expect(t, listed, 200)
	if !strings.Contains(listed.Body.String(), `"storage":"remote"`) {
		t.Fatal("share storage not listed")
	}
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/providers", map[string]any{"action": "detach", "id": "remote"}, a.StorageProviders), 200)
	expect(t, get(), 503)
	if _, e := a.DB.Exec("DELETE FROM shares WHERE id=?", shared.ID); e != nil {
		t.Fatal(e)
	}
	expect(t, get(), 404)
}

func TestRemoteActionsPreserveSourcesOnFailure(t *testing.T) {
	a, admin, _ := setup(t)
	remote := t.TempDir()
	if err := os.WriteFile(filepath.Join(remote, "file.txt"), []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveConnections([]core.StorageConnection{{ID: "remote", Provider: "local", Root: remote, Attached: true, Options: map[string]string{}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec("INSERT INTO user_limits(user_id,quota_bytes) VALUES(?,1)", admin.ID); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"action": "move", "storage": "remote", "paths": []string{"file.txt"}}
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/actions", payload, a.FileActions), 413)
	if _, err := os.Stat(filepath.Join(remote, "file.txt")); err != nil {
		t.Fatal("quota failure removed source")
	}
	if _, err := os.Stat(filepath.Join(a.Root, "file.txt")); !os.IsNotExist(err) {
		t.Fatal("quota failure published destination")
	}
	if _, err := a.DB.Exec("UPDATE user_limits SET quota_bytes=0 WHERE user_id=?", admin.ID); err != nil {
		t.Fatal(err)
	}
	write(t, a, "file.txt", "existing")
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/actions", payload, a.FileActions), 409)
	if b, e := os.ReadFile(filepath.Join(a.Root, "file.txt")); e != nil || string(b) != "existing" {
		t.Fatal("collision overwrote destination")
	}
	write(t, a, "protected/data.txt", "keep")
	grant(t, a, admin, "protected", "write")
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/actions", map[string]any{"action": "move", "storage": "", "destinationStorage": "remote", "paths": []string{"protected"}}, a.FileActions), 409)
	if _, err := os.Stat(filepath.Join(a.Root, "protected/data.txt")); err != nil {
		t.Fatal("protected source removed")
	}
	if _, err := os.Stat(filepath.Join(remote, "protected")); !os.IsNotExist(err) {
		t.Fatal("protected move published copy")
	}
	if err := os.Symlink(remote, filepath.Join(a.Root, "link")); err == nil {
		expect(t, jsonRequest(a, admin, "POST", "/api/storage/actions", map[string]any{"action": "copy", "storage": "", "destinationStorage": "remote", "paths": []string{"link"}}, a.FileActions), 400)
	}
}

func TestRemoteSharesPasswordFolderAndExpiry(t *testing.T) {
	a, admin, _ := setup(t)
	remote := t.TempDir()
	if err := os.Mkdir(filepath.Join(remote, "Docs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remote, "Docs/a.txt"), []byte("protected"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveConnections([]core.StorageConnection{{ID: "remote", Provider: "local", Root: remote, Attached: true, Options: map[string]string{}}}); err != nil {
		t.Fatal(err)
	}
	w := jsonRequest(a, admin, "POST", "/api/shares", map[string]any{"storage": "remote", "paths": []string{"Docs"}, "days": 7, "password": "test-password"}, a.CreateShare)
	expect(t, w, 200)
	var shared struct {
		Token string
		ID    int
	}
	if err := json.Unmarshal(w.Body.Bytes(), &shared); err != nil {
		t.Fatal(err)
	}
	serve := func(r *http.Request) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		a.Router().ServeHTTP(w, r)
		return w
	}
	url := "/s/" + shared.Token
	expect(t, serve(httptest.NewRequest("GET", url+"?download=1", nil)), 401)
	form := httptest.NewRequest("POST", url, strings.NewReader("password=test-password"))
	form.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unlocked := serve(form)
	expect(t, unlocked, 200)
	cookies := unlocked.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("share session missing")
	}
	get := func(suffix string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", url+suffix, nil)
		r.AddCookie(cookies[0])
		return serve(r)
	}
	w = get("?download=1")
	expect(t, w, 200)
	archive, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range archive.File {
		if f.Name == "Docs/a.txt" {
			found = true
		}
	}
	if !found {
		t.Fatal("folder share ZIP missing file")
	}
	expect(t, get("?download=1&item=outside.txt"), 404)
	w = get("?download=1&item=Docs%2Fa.txt")
	expect(t, w, 200)
	if w.Body.String() != "protected" {
		t.Fatal("shared child download failed")
	}
	expect(t, get("?qr=1"), 200)
	reopened := get("")
	expect(t, reopened, 200)
	if !strings.Contains(reopened.Body.String(), "Unlock share") || strings.Contains(reopened.Body.String(), "Docs/a.txt") {
		t.Fatal("reopened remote share bypassed password prompt")
	}
	expect(t, get("?download=1"), 401)
	expect(t, get("?qr=1"), 401)
	if _, err := a.DB.Exec("UPDATE shares SET expires_at='2000-01-01' WHERE id=?", shared.ID); err != nil {
		t.Fatal(err)
	}
	expect(t, get("?download=1"), 404)
}

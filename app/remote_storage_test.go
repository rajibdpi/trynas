package app

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rajib/trynas/core"
)

func TestAttachedStorageLifecycleAndFiles(t *testing.T) {
	a, admin, user := setup(t)
	remote := t.TempDir()
	c := core.StorageConnection{ID: "remote", Name: "Attached test", Provider: "local", Root: remote, Options: map[string]string{}}
	if err := a.SaveConnections([]core.StorageConnection{c}); err != nil {
		t.Fatal(err)
	}
	expect(t, request(a, admin, "GET", "/api/storage/files?storage=remote", nil, a.RemoteStorage), 404)
	expect(t, jsonRequest(a, user, "POST", "/api/storage/providers", map[string]any{"action": "attach", "id": c.ID}, a.StorageProviders), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/providers", map[string]any{"action": "attach", "id": c.ID}, a.StorageProviders), 200)
	saved, err := a.Connections()
	if err != nil || !saved[0].Attached {
		t.Fatal("attachment not persisted", err)
	}
	expect(t, request(a, user, "GET", "/api/storage/files?storage=remote", nil, a.RemoteStorage), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/files?storage=remote", map[string]any{"action": "mkdir", "name": "Docs"}, a.RemoteStorage), 200)
	if err := os.WriteFile(filepath.Join(remote, "Docs/a.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	w := request(a, admin, "GET", "/api/storage/files?storage=remote&path=Docs", nil, a.RemoteStorage)
	expect(t, w, 200)
	var listed struct {
		Items []struct {
			Name string
			Path string
			Dir  bool
		}
	}
	if json.Unmarshal(w.Body.Bytes(), &listed) != nil || len(listed.Items) != 1 || listed.Items[0].Path != "Docs/a.txt" {
		t.Fatal("remote listing incorrect", w.Body.String())
	}
	w = request(a, admin, "GET", "/api/storage/files?storage=remote&path=Docs/a.txt&download=1", nil, a.RemoteStorage)
	expect(t, w, 200)
	if w.Body.String() != "hello" || w.Header().Get("Content-Disposition") == "" {
		t.Fatal("download failed")
	}
	expect(t, request(a, admin, "GET", "/api/storage/files?storage=remote&path=../outside", nil, a.RemoteStorage), 400)
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/files?storage=remote", map[string]any{"action": "delete", "dir": true, "confirm": true}, a.RemoteStorage), 400)
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/files?storage=remote&path=Docs", map[string]any{"action": "delete", "dir": true}, a.RemoteStorage), 400)
	upload := func(name string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, e := writer.CreateFormFile("files", name)
		if e != nil {
			t.Fatal(e)
		}
		_, _ = part.Write([]byte("uploaded"))
		writer.Close()
		r := httptest.NewRequest(http.MethodPost, "/api/storage/upload?storage=remote&path=Docs", &body)
		r.Header.Set("Content-Type", writer.FormDataContentType())
		r = r.WithContext(context.WithValue(r.Context(), core.UserKey, admin))
		w := httptest.NewRecorder()
		a.Cors(a.Guard(http.HandlerFunc(a.RemoteStorage))).ServeHTTP(w, r)
		return w
	}
	expect(t, upload("b.txt"), 200)
	expect(t, upload("b.txt"), 409)
	if data, err := os.ReadFile(filepath.Join(remote, "Docs/b.txt")); err != nil || string(data) != "uploaded" {
		t.Fatal("remote upload failed", err)
	}
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/files?storage=remote&path=Docs/b.txt", map[string]any{"action": "delete", "confirm": true}, a.RemoteStorage), 200)
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/files?storage=remote&path=Docs", map[string]any{"action": "delete", "dir": true, "confirm": true}, a.RemoteStorage), 200)
	if _, err := os.Stat(filepath.Join(remote, "Docs")); !os.IsNotExist(err) {
		t.Fatal("directory deletion failed")
	}
	expect(t, jsonRequest(a, admin, "POST", "/api/storage/providers", map[string]any{"action": "detach", "id": c.ID}, a.StorageProviders), 200)
	expect(t, request(a, admin, "GET", "/api/storage/files?storage=remote", nil, a.RemoteStorage), 404)
	if _, err := os.Stat(remote); err != nil {
		t.Fatal("detach deleted storage")
	}
}

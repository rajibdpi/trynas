package app

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rajibdpi/trynas/frontend"
)

func TestEmbeddedStaticServing(t *testing.T) {
	handler := frontend.StaticFrontend(fstest.MapFS{
		"index.html":                {Data: []byte("<!doctype html><h1>TryNAS</h1>")},
		"files/__next._tree.txt":    {Data: []byte("RSC payload")},
		"settings/__next._tree.txt": {Data: []byte("RSC payload")},
		"settings.html":             {Data: []byte("<!doctype html><h1>Settings</h1>")},
		"files.html":                {Data: []byte("<!doctype html><h1>All files</h1>")},
		"_next/static/app.js":       {Data: []byte("console.log('app')")},
		"assets/private.txt":        {Data: []byte("fixture")},
		".env":                      {Data: []byte("must never serve")},
	})
	for _, test := range []struct {
		path   string
		status int
	}{{"/", 200}, {"/files", 200}, {"/files/", 200}, {"/settings", 200}, {"/settings/", 200}, {"/_next/static/app.js", 200}, {"/api/missing", 404}, {"/s/missing", 404}, {"/.env", 404}, {"/assets/", 404}, {"/missing", 404}} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", test.path, nil))
		if w.Code != test.status {
			t.Errorf("%s status %d want %d", test.path, w.Code, test.status)
		}
		if test.path == "/" && !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
			t.Fatal("HTML served with wrong MIME type")
		}
		if strings.HasPrefix(test.path, "/files") && !strings.Contains(w.Body.String(), "All files") {
			t.Fatal("files route served wrong page")
		}
		if strings.HasPrefix(test.path, "/settings") && !strings.Contains(w.Body.String(), "Settings") {
			t.Fatal("settings route served wrong page")
		}
		if test.path == "/_next/static/app.js" && !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
			t.Fatal("hashed asset is not cached")
		}
	}
}

func TestDatabasePragmasAndForeignKeys(t *testing.T) {
	a, _, u := setup(t)
	var foreignKeys int
	if err := a.DB.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatal("foreign keys disabled", err)
	}
	if _, err := a.DB.Exec("INSERT INTO permissions(user_id,path,access) VALUES(999999,'','read')"); err == nil {
		t.Fatal("missing foreign key enforcement")
	}
	grant(t, a, u, "", "read")
	if _, err := a.DB.Exec("DELETE FROM users WHERE id=?", u.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM permissions WHERE user_id=?", u.ID).Scan(&n)
	if n != 0 {
		t.Fatal("user permissions did not cascade")
	}
}

package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProtectedFolderShare(t *testing.T) {
	a, _, user := setup(t)
	write(t, a, "shared/public.txt", "public")
	write(t, a, "shared/private/secret.txt", "secret")
	write(t, a, "shared/.trynas-hidden", "hidden")
	if err := os.Symlink(filepath.Join(a.root, "shared/private/secret.txt"), filepath.Join(a.root, "shared/link.txt")); err != nil {
		t.Fatal(err)
	}
	grant(t, a, user, "shared", "read")
	grant(t, a, user, "shared/private", "none")
	expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	made := jsonRequest(a, user, "POST", "/api/shares", map[string]any{"path": "shared", "expiresAt": expiry.Format(time.RFC3339), "password": "share-password"}, a.createShare)
	expect(t, made, 200)
	var share struct {
		Token             string
		ExpiresAt         string
		PasswordProtected bool
	}
	if err := json.Unmarshal(made.Body.Bytes(), &share); err != nil {
		t.Fatal(err)
	}
	if !share.PasswordProtected || share.ExpiresAt != expiry.Format(time.RFC3339) {
		t.Fatal("share settings not preserved")
	}
	var hash string
	if err := a.db.QueryRow("SELECT password_hash FROM share_passwords").Scan(&hash); err != nil || hash == "share-password" {
		t.Fatal("password not hashed")
	}
	router := chi.NewRouter()
	router.Get("/s/{token}", a.publicShare)
	router.Post("/s/{token}", a.publicShare)
	get := func(method, query, password string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/s/"+share.Token+query, strings.NewReader(url.Values{"password": {password}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	locked := get("GET", "", "", nil)
	expect(t, locked, 200)
	if !strings.Contains(locked.Body.String(), "Unlock share") || strings.Contains(locked.Body.String(), "public.txt") {
		t.Fatal("missing password gate")
	}
	expect(t, get("GET", "?download=1", "", nil), 401)
	expect(t, get("GET", "?qr=1", "", nil), 401)
	expect(t, get("POST", "", "wrong-password", nil), 401)
	unlock := get("POST", "", "share-password", nil)
	expect(t, unlock, 303)
	cookies := unlock.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatal("missing share session")
	}
	cookie := cookies[0]
	page := get("GET", "", "", cookie)
	expect(t, page, 200)
	if !strings.Contains(page.Body.String(), "Download folder ZIP") {
		t.Fatal("missing folder page")
	}
	if !strings.Contains(page.Body.String(), "shared/public.txt") || strings.Contains(page.Body.String(), "secret.txt") || strings.Contains(page.Body.String(), ".trynas-hidden") || strings.Contains(page.Body.String(), "link.txt") {
		t.Fatal("incorrect public folder listing")
	}
	expect(t, get("GET", "?download=1&item=shared%2Fpublic.txt", "", nil), 401)
	individual := get("GET", "?download=1&item=shared%2Fpublic.txt", "", cookie)
	expect(t, individual, 200)
	if individual.Body.String() != "public" {
		t.Fatal("incorrect individual download")
	}
	expect(t, get("GET", "?download=1&item=shared%2Fprivate%2Fsecret.txt", "", cookie), 404)
	archive := get("GET", "?download=1", "", cookie)
	expect(t, archive, 200)
	reader, err := zip.NewReader(bytes.NewReader(archive.Body.Bytes()), int64(archive.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range reader.File {
		if strings.Contains(entry.Name, "secret") || strings.Contains(entry.Name, ".trynas-") || strings.Contains(entry.Name, "link.txt") {
			t.Fatalf("forbidden ZIP entry %s", entry.Name)
		}
		if entry.Name == "shared/public.txt" {
			found = true
			f, err := entry.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(f)
			f.Close()
			if string(data) != "public" {
				t.Fatal("file contents changed")
			}
		}
	}
	if !found {
		t.Fatal("missing readable file")
	}
	expect(t, get("GET", "?qr=1", "", cookie), 200)
	if _, err := a.db.Exec("UPDATE shares SET expires_at='2000-01-01' WHERE token=?", share.Token); err != nil {
		t.Fatal(err)
	}
	expect(t, get("GET", "?download=1", "", cookie), 404)
}

func TestShareExpiryAndPasswordValidation(t *testing.T) {
	a, _, user := setup(t)
	write(t, a, "shared/file.txt", "file")
	grant(t, a, user, "shared", "read")
	for _, settings := range []map[string]any{
		{"expiresAt": time.Now().Add(-time.Minute).Format(time.RFC3339)},
		{"expiresAt": time.Now().Add(366 * 24 * time.Hour).Format(time.RFC3339)},
		{"expiresAt": "2026-10-08T12:00"},
		{"days": 7, "password": "short"},
		{"days": 7, "password": strings.Repeat("a", 73)},
	} {
		settings["path"] = "shared/file.txt"
		expect(t, jsonRequest(a, user, "POST", "/api/shares", settings, a.createShare), 400)
	}
	offset := time.Now().Add(time.Hour).In(time.FixedZone("Dhaka", 6*3600)).Format(time.RFC3339)
	expect(t, jsonRequest(a, user, "POST", "/api/shares", map[string]any{"path": "shared/file.txt", "expiresAt": offset}, a.createShare), 200)
	grant(t, a, user, "shared", "none")
	expect(t, jsonRequest(a, user, "POST", "/api/shares", map[string]any{"path": "shared/file.txt", "days": 7}, a.createShare), 403)
}

func TestLegacyShareExpiryMigration(t *testing.T) {
	a, _, user := setup(t)
	future := time.Now().Add(time.Hour).In(time.FixedZone("Dhaka", 6*3600))
	_, err := a.db.Exec("INSERT INTO shares(user_id,path,token,expires_at) VALUES(?,?,?,?)", user.ID, "file.txt", "legacy", future.String()+" m=+3600.123")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.migrateV5(); err != nil {
		t.Fatal(err)
	}
	var text string
	var valid int
	if err = a.db.QueryRow("SELECT CAST(expires_at AS TEXT),julianday(expires_at)>julianday('now') FROM shares WHERE token='legacy'").Scan(&text, &valid); err != nil {
		t.Fatal(err)
	}
	if valid != 1 || text != future.UTC().Format("2006-01-02 15:04:05.999") {
		t.Fatalf("legacy expiry changed: %s", text)
	}
	if err = a.migrateV5(); err != nil {
		t.Fatal(err)
	}
}

func TestNeverAndPresetShareExpiry(t *testing.T) {
	a, _, user := setup(t)
	write(t, a, "shared/file.txt", "file")
	grant(t, a, user, "shared", "read")
	for _, days := range []int{0, 1, 7, 15, 30} {
		settings := map[string]any{"path": "shared/file.txt"}
		if days == 0 {
			settings["never"] = true
		} else {
			settings["days"] = days
		}
		before := time.Now()
		w := jsonRequest(a, user, "POST", "/api/shares", settings, a.createShare)
		expect(t, w, 200)
		var share struct {
			Token     string
			ExpiresAt string
		}
		if err := json.Unmarshal(w.Body.Bytes(), &share); err != nil {
			t.Fatal(err)
		}
		if days == 0 {
			var count int
			if err := a.db.QueryRow("SELECT COUNT(*) FROM shares WHERE token=? AND expires_at IS NULL", share.Token).Scan(&count); err != nil || count != 1 || share.ExpiresAt != "" {
				t.Fatal("never share has expiry")
			}
			router := chi.NewRouter()
			router.Get("/s/{token}", a.publicShare)
			page := httptest.NewRecorder()
			router.ServeHTTP(page, httptest.NewRequest("GET", "/s/"+share.Token, nil))
			expect(t, page, 200)
		} else {
			expiry, err := time.Parse(time.RFC3339, share.ExpiresAt)
			expected := before.Add(time.Duration(days) * 24 * time.Hour)
			if err != nil || expiry.Sub(expected) < -time.Second || expiry.Sub(expected) > time.Second {
				t.Fatal("incorrect preset expiry")
			}
		}
	}
	expect(t, jsonRequest(a, user, "POST", "/api/shares", map[string]any{"path": "shared/file.txt", "never": true, "days": 7}, a.createShare), 400)
}

func TestMultipleItemsOneShare(t *testing.T) {
	a, _, user := setup(t)
	write(t, a, "one/file.txt", "one")
	write(t, a, "two/file.txt", "two")
	write(t, a, "folder/public.txt", "public")
	write(t, a, "folder/private/secret.txt", "secret")
	for _, p := range []string{"one", "two", "folder"} {
		grant(t, a, user, p, "read")
	}
	grant(t, a, user, "folder/private", "none")
	made := jsonRequest(a, user, "POST", "/api/shares", map[string]any{"paths": []string{"one/file.txt", "two/file.txt", "folder", "folder/public.txt", "folder"}, "never": true}, a.createShare)
	expect(t, made, 200)
	var share struct {
		Token string
		Paths []string
	}
	if err := json.Unmarshal(made.Body.Bytes(), &share); err != nil {
		t.Fatal(err)
	}
	if len(share.Paths) != 3 {
		t.Fatalf("overlaps not removed: %v", share.Paths)
	}
	listed := request(a, user, "GET", "/api/shares", nil, a.shares)
	expect(t, listed, 200)
	if !strings.Contains(listed.Body.String(), "one/file.txt") || !strings.Contains(listed.Body.String(), "two/file.txt") {
		t.Fatal("share listing missing paths")
	}
	router := chi.NewRouter()
	router.Get("/s/{token}", a.publicShare)
	get := func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/s/"+share.Token+query, nil))
		return w
	}
	page := get("")
	expect(t, page, 200)
	if !strings.Contains(page.Body.String(), "3 shared items") {
		t.Fatal("missing collection page")
	}
	for _, name := range []string{"file.txt", "folder/public.txt"} {
		if !strings.Contains(page.Body.String(), name) {
			t.Fatal("missing shared item", name)
		}
	}
	expect(t, get("?download=1&item=one%2Ffile.txt"), 200)
	expect(t, get("?download=1&item=folder%2Fprivate%2Fsecret.txt"), 404)
	expect(t, get("?download=1&item=..%2Fsecret"), 400)
	write(t, a, "one/unshared.txt", "unshared")
	expect(t, get("?download=1&item=one%2Funshared.txt"), 404)
	archive := get("?download=1")
	expect(t, archive, 200)
	reader, err := zip.NewReader(bytes.NewReader(archive.Body.Bytes()), int64(archive.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	contents := map[string]string{}
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		f, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(f)
		f.Close()
		contents[entry.Name] = string(b)
	}
	if contents["one/file.txt"] != "one" || contents["two/file.txt"] != "two" || contents["folder/public.txt"] != "public" || len(contents) != 3 {
		t.Fatalf("incorrect collection ZIP: %v", contents)
	}
	grant(t, a, user, "two", "none")
	expect(t, get("?download=1"), 404)
	expect(t, jsonRequest(a, user, "POST", "/api/shares", map[string]any{"paths": []string{"one/file.txt", "two/file.txt"}, "days": 7}, a.createShare), 403)
	expect(t, jsonRequest(a, user, "POST", "/api/shares", map[string]any{"paths": []string{"one/file.txt", "../secret"}, "days": 7}, a.createShare), 400)
}

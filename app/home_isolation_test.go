package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestNormalUserHomeIsHardBoundary(t *testing.T) {
	a, admin, user := setup(t)
	write(t, a, "alice/deep/nested/own.txt", "own")
	write(t, a, "bob/secret.txt", "secret")
	write(t, a, "alice-other/secret.txt", "secret")
	if _, err := a.DB.Exec("INSERT INTO user_homes(user_id,path) VALUES(?,'alice')", user.ID); err != nil {
		t.Fatal(err)
	}
	grant(t, a, user, "", "write")
	grant(t, a, user, "bob", "write")
	for _, p := range []string{"", "bob", "bob/secret.txt", "alice-other", "alice-other/secret.txt"} {
		if a.Can(user, p, false) || a.Can(user, p, true) || a.CanBrowse(user, p) {
			t.Fatalf("escaped home through ACL: %q", p)
		}
	}
	for _, p := range []string{"alice", "alice/deep", "alice/deep/nested/own.txt"} {
		if !a.Can(user, p, false) || !a.Can(user, p, true) || !a.CanBrowse(user, p) {
			t.Fatalf("home descendant denied: %q", p)
		}
	}
	profile := a.Profile(user)
	if profile["home"] != "alice" || profile["rootAccessible"] != false {
		t.Fatal("incorrect restricted profile", profile)
	}
	for _, p := range []string{"", "bob", "alice-other"} {
		expect(t, request(a, user, "GET", "/api/files?path="+p, nil, a.ListFiles), 403)
		expect(t, request(a, user, "GET", "/api/files/search?q=txt&path="+p, nil, a.SearchFiles), 403)
	}
	expect(t, request(a, user, "GET", "/api/files/info?path=bob/secret.txt", nil, a.FileInfo), 403)
	expect(t, request(a, user, "GET", "/api/files/download?path=bob/secret.txt", nil, a.Download), 403)
	expect(t, request(a, user, "GET", "/api/files/download?path=alice/deep/nested/own.txt", nil, a.Download), 200)
	expect(t, request(a, admin, "GET", "/api/files?path=bob", nil, a.ListFiles), 200)
	expect(t, jsonRequest(a, user, "POST", "/api/files/folder", map[string]any{"path": "bob/new"}, a.Mkdir), 403)
	expect(t, jsonRequest(a, user, "POST", "/api/files/folder", map[string]any{"path": "alice/deep/new"}, a.Mkdir), 200)
	expect(t, jsonRequest(a, user, "POST", "/api/files/copy", map[string]any{"from": "alice/deep/nested/own.txt", "to": "bob/copy.txt"}, a.CopyMoveV5(false)), 403)
	expect(t, jsonRequest(a, user, "POST", "/api/shares", map[string]any{"path": "bob/secret.txt", "never": true}, a.CreateShare), 403)
	expect(t, jsonRequest(a, admin, "POST", "/api/permissions", map[string]any{"userId": user.ID, "path": "bob", "access": "read"}, a.SavePermission), 400)
}

func TestHistoricalPathsOutsideHomeAreHidden(t *testing.T) {
	a, _, user := setup(t)
	write(t, a, "alice/own.txt", "own")
	write(t, a, "bob/secret.txt", "secret")
	_, _ = a.DB.Exec("INSERT INTO user_homes(user_id,path) VALUES(?,'alice')", user.ID)
	grant(t, a, user, "", "write")
	for _, p := range []string{"alice/own.txt", "bob/secret.txt"} {
		_, _ = a.DB.Exec("INSERT INTO favorites(user_id,path) VALUES(?,?)", user.ID, p)
		_, _ = a.DB.Exec("INSERT INTO trash(user_id,original_path,trash_name) VALUES(?,?,?)", user.ID, p, p)
		_, _ = a.DB.Exec("INSERT INTO shares(user_id,path,token) VALUES(?,?,?)", user.ID, p, p)
	}
	for _, h := range []http.HandlerFunc{a.Favorites, a.ListTrash, a.Shares} {
		w := request(a, user, "GET", "/api/list", nil, h)
		expect(t, w, 200)
		if strings.Contains(w.Body.String(), "bob") || !strings.Contains(w.Body.String(), "alice") {
			t.Fatal("historical outside path exposed", w.Body.String())
		}
	}
	router := chi.NewRouter()
	router.Get("/s/{token}", a.PublicShare)
	_, _ = a.DB.Exec("UPDATE shares SET token='outside-token' WHERE path='bob/secret.txt'")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/s/outside-token", nil))
	expect(t, w, 404)
}

func TestMissingHomeFailsClosedAndLegacyMigrationProvisionsHome(t *testing.T) {
	a, _, user := setup(t)
	_, _ = a.DB.Exec("INSERT INTO permissions(user_id,path,access) VALUES(?,'','write')", user.ID)
	if a.Can(user, "shared/file.txt", false) || a.CanBrowse(user, "shared") || a.Profile(user)["home"] != "" {
		t.Fatal("missing home fell back to broad permissions")
	}
	if err := a.MigrateV5(); err != nil {
		t.Fatal(err)
	}
	if a.Home(user.ID) != user.Username || !a.Can(user, user.Username+"/child/file.txt", true) || a.Can(user, "shared/file.txt", false) {
		t.Fatal("legacy home boundary is incorrect")
	}
	if err := a.MigrateV5(); err != nil {
		t.Fatal("home migration is not idempotent", err)
	}
}

func TestHomeAssignmentCannotBeReusedWhenFolderIsMissing(t *testing.T) {
	a, admin, user := setup(t)
	if _, err := a.DB.Exec("INSERT INTO user_homes(user_id,path) VALUES(?,'reserved-home')", user.ID); err != nil {
		t.Fatal(err)
	}
	expect(t, jsonRequest(a, admin, "POST", "/api/users", map[string]any{"username": "reserved-home", "password": "test-password-long", "role": "user"}, a.SaveUser), 409)
	result, err := a.DB.Exec("INSERT INTO registration_requests(username,password_hash) VALUES('reserved-home','unused')")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	router := chi.NewRouter()
	router.Post("/api/users/requests/{id}/approve", a.ApproveRegistration)
	expect(t, jsonRequest(a, admin, "POST", fmt.Sprintf("/api/users/requests/%d/approve", id), map[string]any{"quota": 0}, router.ServeHTTP), 409)

}

func TestDuplicateHomeAssignmentsFailClosed(t *testing.T) {
	a, _, user := setup(t)
	_, _ = a.DB.Exec("INSERT INTO user_homes(user_id,path) VALUES(?,'alice')", user.ID)
	grant(t, a, user, "", "write")
	result, err := a.DB.Exec("INSERT INTO users(username,password_hash,role) VALUES('another-member','unused','user')")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := result.LastInsertId()
	if _, err = a.DB.Exec("INSERT INTO user_homes(user_id,path) VALUES(?,'alice')", other); err != nil {
		t.Fatal(err)
	}
	if a.Home(user.ID) != "" || a.Can(user, "alice/file.txt", false) {
		t.Fatal("ambiguous home assignment allowed access")
	}
}

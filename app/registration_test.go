package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rajibdpi/trynas/core"
)

func registrationRequest(t *testing.T, a *App, actor *core.User, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if actor != nil {
		token := core.Token()
		if _, err := a.DB.Exec("INSERT INTO sessions(token,user_id,expires_at) VALUES(?,?,?)", token, actor.ID, time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
		r.AddCookie(&http.Cookie{Name: "trynas_session", Value: token})
	}
	w := httptest.NewRecorder()
	a.Router().ServeHTTP(w, r)
	return w
}

func enableRegistration(t *testing.T, a *App, admin core.User, enabled bool) {
	t.Helper()
	settings := core.AppSettings{AppName: "TryNAS", FileView: "grid", ShareExpiry: "7", RegistrationEnabled: enabled}
	expect(t, jsonRequest(a, admin, "POST", "/api/settings", settings, a.Settings), 200)
}

func TestRegistrationApprovalAndSwitch(t *testing.T) {
	a, admin, user := setup(t)
	body := `{"username":"new-user","password":"registration-password","role":"admin"}`
	expect(t, registrationRequest(t, a, nil, "POST", "/api/auth/register", body), 403)
	enableRegistration(t, a, admin, true)
	expect(t, registrationRequest(t, a, nil, "POST", "/api/auth/register", body), 202)
	expect(t, registrationRequest(t, a, nil, "POST", "/api/auth/register", body), 409)
	if _, err := os.Stat(filepath.Join(a.Root, "new-user")); !os.IsNotExist(err) {
		t.Fatal("folder created before approval", err)
	}
	var count, id int
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM users WHERE username='new-user'").Scan(&count); err != nil || count != 0 {
		t.Fatal("pending user is active", err)
	}
	expect(t, registrationRequest(t, a, nil, "POST", "/api/auth/login", body), 401)
	expect(t, registrationRequest(t, a, nil, "GET", "/api/users/requests", ""), 401)
	expect(t, registrationRequest(t, a, &user, "GET", "/api/users/requests", ""), 403)
	if err := a.DB.QueryRow("SELECT id FROM registration_requests WHERE username='new-user'").Scan(&id); err != nil {
		t.Fatal(err)
	}
	path := "/api/users/requests/" + strconv.Itoa(id) + "/approve"
	expect(t, registrationRequest(t, a, &user, "POST", path, `{"quota":1024}`), 403)
	enableRegistration(t, a, admin, false)
	expect(t, registrationRequest(t, a, nil, "POST", "/api/auth/register", `{"username":"another","password":"registration-password"}`), 403)
	options := registrationRequest(t, a, nil, "GET", "/api/auth/registration", "")
	if !strings.Contains(options.Body.String(), `"enabled":false`) {
		t.Fatal("public switch is stale")
	}
	expect(t, registrationRequest(t, a, &admin, "POST", path, `{"quota":1024}`), 200)
	expect(t, registrationRequest(t, a, &admin, "POST", path, `{"quota":1024}`), 404)
	st, err := os.Stat(filepath.Join(a.Root, "new-user"))
	if err != nil || !st.IsDir() {
		t.Fatal("approved folder missing", err)
	}
	var approved core.User
	if err := a.DB.QueryRow("SELECT id,username,role FROM users WHERE username='new-user'").Scan(&approved.ID, &approved.Username, &approved.Role); err != nil {
		t.Fatal(err)
	}
	if approved.Role != "user" || !a.Can(approved, "new-user/file", true) || a.Can(approved, "other/file", false) {
		t.Fatal("incorrect approved access")
	}
	var quota int64
	if err := a.DB.QueryRow("SELECT quota_bytes FROM user_limits WHERE user_id=?", approved.ID).Scan(&quota); err != nil || quota != 1024 {
		t.Fatal("quota missing", err)
	}
	login := registrationRequest(t, a, nil, "POST", "/api/auth/login", body)
	expect(t, login, 200)
	var profile map[string]any
	if err := json.Unmarshal(login.Body.Bytes(), &profile); err != nil || profile["home"] != "new-user" {
		t.Fatal("approved login home incorrect", err)
	}
}

func TestRegistrationConflictAndRejection(t *testing.T) {
	a, admin, _ := setup(t)
	enableRegistration(t, a, admin, true)
	expect(t, registrationRequest(t, a, nil, "POST", "/api/auth/register", `{"username":"../escape","password":"registration-password"}`), 400)
	expect(t, registrationRequest(t, a, nil, "POST", "/api/auth/register", `{"username":"requested","password":"registration-password"}`), 202)
	var id int
	if err := a.DB.QueryRow("SELECT id FROM registration_requests WHERE username='requested'").Scan(&id); err != nil {
		t.Fatal(err)
	}
	write(t, a, "requested/existing.txt", "keep")
	path := "/api/users/requests/" + strconv.Itoa(id)
	expect(t, registrationRequest(t, a, &admin, "POST", path+"/approve", `{"quota":0}`), 409)
	var count int
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM users WHERE username='requested'").Scan(&count); err != nil || count != 0 {
		t.Fatal("failed approval left account", err)
	}
	data, err := os.ReadFile(filepath.Join(a.Root, "requested/existing.txt"))
	if err != nil || string(data) != "keep" {
		t.Fatal("existing files changed", err)
	}
	expect(t, registrationRequest(t, a, &admin, "DELETE", path, ""), 200)
	expect(t, registrationRequest(t, a, &admin, "DELETE", path, ""), 404)
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM registration_requests").Scan(&count); err != nil || count != 0 {
		t.Fatal("request not rejected", err)
	}
}

func TestRegistrationApprovalRollsBackFolder(t *testing.T) {
	a, admin, _ := setup(t)
	enableRegistration(t, a, admin, true)
	expect(t, registrationRequest(t, a, nil, "POST", "/api/auth/register", `{"username":"rollback-user","password":"registration-password"}`), 202)
	var id int
	if err := a.DB.QueryRow("SELECT id FROM registration_requests WHERE username='rollback-user'").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(`CREATE TRIGGER reject_approved_quota BEFORE INSERT ON user_limits BEGIN SELECT RAISE(ABORT, 'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	expect(t, registrationRequest(t, a, &admin, "POST", "/api/users/requests/"+strconv.Itoa(id)+"/approve", `{"quota":0}`), 500)
	if _, err := os.Stat(filepath.Join(a.Root, "rollback-user")); !os.IsNotExist(err) {
		t.Fatal("failed approval left folder", err)
	}
	var count int
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM users WHERE username='rollback-user'").Scan(&count); err != nil || count != 0 {
		t.Fatal("failed approval left user", err)
	}
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM registration_requests WHERE id=?", id).Scan(&count); err != nil || count != 1 {
		t.Fatal("failed approval lost request", err)
	}
}

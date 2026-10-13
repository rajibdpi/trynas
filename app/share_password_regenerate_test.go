package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	chi "github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"testing"
)

func TestRegenerateSharePasswordPreservesLink(t *testing.T) {
	a, admin, user := setup(t)
	write(t, a, "shared/file.txt", "content")
	grant(t, a, user, "shared", "read")
	oldPassword := "original-password"
	created := jsonRequest(a, user, "POST", "/api/shares", map[string]any{"path": "shared/file.txt", "days": 7, "password": oldPassword}, a.CreateShare)
	expect(t, created, 200)
	var shared struct{ Token string }
	if err := json.Unmarshal(created.Body.Bytes(), &shared); err != nil {
		t.Fatal(err)
	}
	var id int64
	_ = a.DB.QueryRow("SELECT id FROM shares WHERE token=?", shared.Token).Scan(&id)
	var initialHash string
	if err := a.DB.QueryRow("SELECT password_hash FROM share_passwords WHERE share_id=?", id).Scan(&initialHash); err != nil {
		t.Fatal(err)
	}
	if initialHash == oldPassword || bcrypt.CompareHashAndPassword([]byte(initialHash), []byte(oldPassword)) != nil {
		t.Fatal("created password is not stored as a bcrypt hash")
	}
	_, _ = a.DB.Exec("INSERT INTO share_sessions(token,share_id,expires_at) VALUES('old-session',?,datetime('now','+1 day'))", id)
	router := chi.NewRouter()
	router.Post("/api/shares/{id}/password", a.RegenerateSharePassword)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { router.ServeHTTP(w, r) })
	url := fmt.Sprintf("/api/shares/%d/password", id)
	expect(t, jsonRequest(a, admin, "POST", url, map[string]any{}, handler), 404)
	regenerated := jsonRequest(a, user, "POST", url, map[string]any{}, handler)
	expect(t, regenerated, 200)
	var result struct{ Password string }
	if err := json.Unmarshal(regenerated.Body.Bytes(), &result); err != nil || len(result.Password) != 24 || result.Password == oldPassword {
		t.Fatal("invalid generated password", err)
	}
	if regenerated.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("password must not be cached")
	}
	var token, hash string
	_ = a.DB.QueryRow("SELECT s.token,p.password_hash FROM shares s JOIN share_passwords p ON p.share_id=s.id WHERE s.id=?", id).Scan(&token, &hash)
	if hash == result.Password {
		t.Fatal("regenerated password stored as plaintext")
	}
	if cost, err := bcrypt.Cost([]byte(hash)); err != nil || cost != bcrypt.DefaultCost {
		t.Fatal("regenerated password does not use the expected bcrypt cost")
	}
	var secretTables int
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='share_password_secrets'").Scan(&secretTables); err != nil || secretTables != 0 {
		t.Fatal("recoverable password storage must not exist")
	}
	if token != shared.Token || bcrypt.CompareHashAndPassword([]byte(hash), []byte(result.Password)) != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPassword)) == nil {
		t.Fatal("link changed or password not replaced")
	}
	var count int
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM share_sessions WHERE share_id=?", id).Scan(&count)
	if count != 0 {
		t.Fatal("old share sessions still valid")
	}
	list := request(a, user, "GET", "/api/shares", nil, a.Shares)
	expect(t, list, 200)
	if bytes.Contains(list.Body.Bytes(), []byte(result.Password)) {
		t.Fatal("share listing exposed password")
	}
	expect(t, request(a, user, "GET", url, nil, handler), 405)
}

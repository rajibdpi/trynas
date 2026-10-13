package app

import (
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"testing"
)

func TestProfileUpdatesOwnAccount(t *testing.T) {
	a, _, user := setup(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	if _, err := a.DB.Exec("UPDATE users SET password_hash=? WHERE id=?", string(hash), user.ID); err != nil {
		t.Fatal(err)
	}
	update := map[string]any{"username": user.Username, "displayName": "Member Name", "email": "member@example.com", "role": "admin", "id": 1}
	saved := jsonRequest(a, user, "POST", "/api/profile", update, a.UpdateProfile)
	expect(t, saved, 200)
	var result map[string]any
	if err := json.Unmarshal(saved.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["displayName"] != "Member Name" || result["email"] != "member@example.com" || result["role"] != "user" || result["id"] != float64(user.ID) {
		t.Fatalf("wrong profile: %v", result)
	}
	update["username"] = "renamed-member"
	expect(t, jsonRequest(a, user, "POST", "/api/profile", update, a.UpdateProfile), 400)
	update["currentPassword"] = "old-password"
	expect(t, jsonRequest(a, user, "POST", "/api/profile", update, a.UpdateProfile), 200)
	user.Username = "renamed-member"
	update["newPassword"] = "new-password"
	if _, err := a.DB.Exec("INSERT INTO sessions(token,user_id,expires_at) VALUES('old-session',?,datetime('now','+1 day'))", user.ID); err != nil {
		t.Fatal(err)
	}
	expect(t, jsonRequest(a, user, "POST", "/api/profile", update, a.UpdateProfile), 200)
	var stored string
	if err := a.DB.QueryRow("SELECT password_hash FROM users WHERE id=?", user.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(stored), []byte("new-password")) != nil {
		t.Fatal("new password not stored")
	}
	var count int
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id=?", user.ID).Scan(&count)
	if count != 0 {
		t.Fatal("old sessions not invalidated")
	}
	update["newPassword"] = ""
	update["email"] = "invalid-email"
	expect(t, jsonRequest(a, user, "POST", "/api/profile", update, a.UpdateProfile), 400)
	update["email"] = "member@example.com"
	update["username"] = "test-admin"
	update["currentPassword"] = "new-password"
	expect(t, jsonRequest(a, user, "POST", "/api/profile", update, a.UpdateProfile), 409)
	update["username"] = "../invalid"
	expect(t, jsonRequest(a, user, "POST", "/api/profile", update, a.UpdateProfile), 400)
}

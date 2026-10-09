package app

import (
	"path/filepath"
	"testing"

	"github.com/rajibdpi/trynas/database"
	"golang.org/x/crypto/bcrypt"
)

func TestRuntimeAdminCredentials(t *testing.T) {
	for _, override := range []string{"", "runtime-password-long"} {
		t.Run(override, func(t *testing.T) {
			t.Setenv("ADMIN_USER", "")
			t.Setenv("ADMIN_PASSWORD", override)
			db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			a := New(db, "", "")
			if override == "" {
				if err := a.Migrate(); err == nil {
					t.Fatal("fresh installation accepted missing admin password")
				}
				return
			}
			if err := a.Migrate(); err != nil {
				t.Fatal(err)
			}
			var username, hash string
			if err := db.QueryRow("SELECT username,password_hash FROM users").Scan(&username, &hash); err != nil {
				t.Fatal(err)
			}
			password := override
			if username != "admin" {
				t.Fatalf("username = %q", username)
			}
			if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
				t.Fatal(err)
			}
			t.Setenv("ADMIN_PASSWORD", "different-runtime-password")
			if err := a.Migrate(); err != nil {
				t.Fatal(err)
			}
			var unchanged string
			if err := db.QueryRow("SELECT password_hash FROM users").Scan(&unchanged); err != nil {
				t.Fatal(err)
			}
			if unchanged != hash {
				t.Fatal("migration reset existing password")
			}
		})
	}
}

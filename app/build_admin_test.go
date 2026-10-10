package app

import (
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/rajibdpi/trynas/core"
	"github.com/rajibdpi/trynas/database"
	"golang.org/x/crypto/bcrypt"
)

func TestEmbeddedAdminCredentials(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("build-password-long"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	previous := core.BuildAdminPasswordHash
	previousEncoded := core.BuildAdminPasswordEncoded
	core.BuildAdminPasswordHash = string(hash)
	core.BuildAdminPasswordEncoded = base64.StdEncoding.EncodeToString([]byte("build-password-long"))
	t.Cleanup(func() { core.BuildAdminPasswordHash = previous; core.BuildAdminPasswordEncoded = previousEncoded })
	for _, runtimePassword := range []string{"", "runtime-password-long", "short"} {
		t.Run(runtimePassword, func(t *testing.T) {
			t.Setenv("ADMIN_PASSWORD", runtimePassword)
			t.Setenv("ADMIN_USER", "admin")
			db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			a := New(db, "", "")
			if runtimePassword == "short" {
				if err := a.Migrate(); err == nil {
					t.Fatal("accepted invalid runtime override")
				}
				return
			}
			if err := a.Migrate(); err != nil {
				t.Fatal(err)
			}
			var stored string
			if err := db.QueryRow("SELECT password_hash FROM users").Scan(&stored); err != nil {
				t.Fatal(err)
			}
			password := runtimePassword
			if password == "" {
				password = "build-password-long"
			}
			if a.InitialAdminPassword != password {
				t.Fatal("wrong first-start password")
			}
			if err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)); err != nil {
				t.Fatal(err)
			}
			t.Setenv("ADMIN_PASSWORD", "replacement-password")
			a.InitialAdminPassword = ""
			if err := a.Migrate(); err != nil {
				t.Fatal(err)
			}
			var unchanged string
			if err := db.QueryRow("SELECT password_hash FROM users").Scan(&unchanged); err != nil {
				t.Fatal(err)
			}
			if stored != unchanged {
				t.Fatal("reset existing account")
			}
			if a.InitialAdminPassword != "" {
				t.Fatal("displayed credentials for existing account")
			}
		})
	}
}

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

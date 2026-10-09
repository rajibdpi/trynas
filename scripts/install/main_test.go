package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPlatforms(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		for _, arch := range []string{"amd64", "arm64"} {
			asset, executable, err := names(platform, arch)
			if err != nil || asset == "" || executable == "" {
				t.Fatal(platform, arch, err)
			}
			if platform == "windows" && executable != "trynas.exe" {
				t.Fatal("missing Windows extension")
			}
		}
	}
	if _, _, err := names("linux", "386"); err == nil {
		t.Fatal("accepted unsupported platform")
	}
}

func TestChecksumParsing(t *testing.T) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte("release")))
	asset := "trynas-linux-amd64"
	if got, err := expectedHash([]byte(hash+"  "+asset+"\n"), asset); err != nil || got != hash {
		t.Fatal(got, err)
	}
	for _, manifest := range []string{"", "invalid  " + asset, hash + "  " + asset + "\n" + hash + "  " + asset} {
		if _, err := expectedHash([]byte(manifest), asset); err == nil {
			t.Fatal("accepted missing, invalid, or duplicate hash")
		}
	}
}

func TestVerifiedInstallationPreservesExistingOnMismatch(t *testing.T) {
	dir := t.TempDir()
	source, destination := filepath.Join(dir, "release"), filepath.Join(dir, "bin", "trynas")
	payload := []byte("release bytes")
	if err := os.WriteFile(source, payload, 0600); err != nil {
		t.Fatal(err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(payload))
	if err := copyVerified(source, destination, hash); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := copyVerified(source, destination, hash); err == nil {
		t.Fatal("accepted altered binary")
	}
	installed, err := os.ReadFile(destination)
	if err != nil || string(installed) != string(payload) {
		t.Fatal("existing installation changed", err)
	}
}

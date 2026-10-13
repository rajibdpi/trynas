package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTargets(t *testing.T) {
	got, err := targets(false, "linux/amd64, windows/arm64,linux/amd64")
	if err != nil || !reflect.DeepEqual(got, []string{"linux/amd64", "windows/arm64"}) {
		t.Fatalf("targets: %v, %v", got, err)
	}
	for _, value := range []string{"linux/386", "linux/amd64,", "../bad"} {
		if _, err := targets(false, value); err == nil {
			t.Fatalf("accepted invalid target %q", value)
		}
	}
	if _, err := targets(true, "linux/amd64"); err == nil {
		t.Fatal("accepted conflicting options")
	}
	all, err := targets(true, "")
	if err != nil || !reflect.DeepEqual(all, supported) {
		t.Fatalf("all targets: %v, %v", all, err)
	}
}

func TestChecksums(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("test release")
	for _, target := range []string{"windows/amd64", "linux/arm64"} {
		if err := os.WriteFile(filepath.Join(dir, artifactName(target)), payload, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "unrelated.txt"), payload, 0644); err != nil {
		t.Fatal(err)
	}
	if err := checksums(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(payload))
	want := hash + "  trynas-linux-arm64\n" + hash + "  trynas-windows-amd64.exe\n"
	if string(got) != want || strings.Contains(string(got), "unrelated") {
		t.Fatalf("checksums = %q, want %q", got, want)
	}
}

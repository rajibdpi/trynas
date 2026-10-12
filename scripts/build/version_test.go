package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNextVersion(t *testing.T) {
	for _, tt := range []struct {
		tags []string
		want string
	}{
		{nil, "v1.0.0"},
		{[]string{"v1.0.9", "v1.0.10", "v2.0.0-rc.1", "main-42"}, "v1.0.11"},
		{[]string{"v2.3.4", "v1.99.99"}, "v2.3.5"},
	} {
		got, err := nextVersion(tt.tags)
		if err != nil || got != tt.want {
			t.Fatalf("nextVersion(%v) = %q, %v; want %q", tt.tags, got, err, tt.want)
		}
	}
}

func TestBuildVersion(t *testing.T) {
	root := t.TempDir()
	command := exec.Command("git", "init", root)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", output, err)
	}
	dist := filepath.Join(root, "dist")
	got, err := buildVersion(root, dist, "")
	if err != nil || got != "v1.0.0" {
		t.Fatalf("initial version = %q, %v", got, err)
	}
	if err := os.MkdirAll(dist, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "VERSION"), []byte("v1.0.9\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err = buildVersion(root, dist, "")
	if err != nil || got != "v1.0.10" {
		t.Fatalf("incremented version = %q, %v", got, err)
	}
	got, err = buildVersion(root, dist, "v2.0.0-rc.1")
	if err != nil || got != "v2.0.0-rc.1" {
		t.Fatalf("explicit version = %q, %v", got, err)
	}
	if _, err := buildVersion(root, dist, "main"); err == nil {
		t.Fatal("accepted invalid explicit version")
	}
}

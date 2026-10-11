package providers

import (
	"context"
	"github.com/rclone/rclone/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifySourceRejectsChangedTree(t *testing.T) {
	root := t.TempDir()
	stage := t.TempDir()
	ctx := context.Background()
	if err := os.Mkdir(filepath.Join(root, "folder"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "folder/a"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := fs.NewFs(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := stageRemote(ctx, f, "folder", stage)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySource(ctx, f, "folder", entries); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "folder/a"), []byte("world"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifySource(ctx, f, "folder", entries); err == nil {
		t.Fatal("same-size changed file passed verification")
	}
	if err := os.WriteFile(filepath.Join(root, "folder/a"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "folder/new"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifySource(ctx, f, "folder", entries); err == nil {
		t.Fatal("new child passed verification")
	}
}

func TestPublishRemoteCollisionPreservesExisting(t *testing.T) {
	root := t.TempDir()
	stage := t.TempDir()
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := fs.NewFs(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishRemote(ctx, f, "a", stage, []stagedEntry{{rel: "content"}}); err == nil {
		t.Fatal("overwrote existing file")
	}
	if data, err := os.ReadFile(filepath.Join(root, "a")); err != nil || string(data) != "existing" {
		t.Fatal("existing data changed")
	}
}

package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestShellInstallerPlatformsAndTampering(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Unix shell tools")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}
	script, err := filepath.Abs("../../get.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, platform := range []struct{ uname, goos string }{{"Linux", "linux"}, {"Darwin", "darwin"}, {"MINGW64_NT", "windows"}} {
		for _, arch := range []struct{ uname, goarch string }{{"x86_64", "amd64"}, {"aarch64", "arm64"}} {
			t.Run(platform.goos+"/"+arch.goarch, func(t *testing.T) {
				dir := t.TempDir()
				source, destination, mocks := filepath.Join(dir, "source"), filepath.Join(dir, "bin"), filepath.Join(dir, "mocks")
				for _, path := range []string{source, mocks} {
					if err := os.Mkdir(path, 0755); err != nil {
						t.Fatal(err)
					}
				}
				// Only uname is mocked; hashing, filesystem operations, and shell parsing
				// run for real, covering every supported release filename.
				uname := "#!/bin/sh\ncase \"$1\" in -s) echo " + platform.uname + ";; -m) echo " + arch.uname + ";; esac\n"
				if err := os.WriteFile(filepath.Join(mocks, "uname"), []byte(uname), 0755); err != nil {
					t.Fatal(err)
				}
				asset, executable, _ := names(platform.goos, arch.goarch)
				payload := []byte("test release binary")
				file := filepath.Join(source, asset)
				if err := os.WriteFile(file, payload, 0600); err != nil {
					t.Fatal(err)
				}
				manifest := fmt.Sprintf("%x  %s\n", sha256.Sum256(payload), asset)
				if err := os.WriteFile(filepath.Join(source, "SHA256SUMS"), []byte(manifest), 0600); err != nil {
					t.Fatal(err)
				}
				run := func() ([]byte, error) {
					cmd := exec.Command(bash, script, "--source", source, "--dir", destination)
					cmd.Env = append(os.Environ(), "PATH="+mocks+string(os.PathListSeparator)+os.Getenv("PATH"))
					return cmd.CombinedOutput()
				}
				if output, err := run(); err != nil {
					t.Fatalf("install failed: %v: %s", err, output)
				}
				if err := os.WriteFile(file, []byte("altered"), 0600); err != nil {
					t.Fatal(err)
				}
				if output, err := run(); err == nil {
					t.Fatalf("tampered install accepted: %s", output)
				}
				installed, err := os.ReadFile(filepath.Join(destination, executable))
				if err != nil || string(installed) != string(payload) {
					t.Fatal("existing installation changed", err)
				}
			})
		}
	}
}

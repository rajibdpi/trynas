package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestShellUpdateModes(t *testing.T) {
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
	for _, tc := range []struct {
		name, mode, existing, manifest, payload, message string
		wantBinary, wantError                            bool
	}{
		{name: "current check", mode: "--check-update", existing: "latest", message: "up to date"},
		{name: "old check", mode: "--check-update", existing: "old", message: "Update available"},
		{name: "missing check", mode: "--check-update", message: "not installed"},
		{name: "current update", mode: "--update", existing: "latest", message: "up to date"},
		{name: "old update", mode: "--update", existing: "old", wantBinary: true, message: "Restart TryNAS"},
		{name: "missing update", mode: "--update", wantBinary: true, message: "Installed"},
		{name: "tampered update", mode: "--update", existing: "old", payload: "tampered", wantBinary: true, wantError: true},
		{name: "invalid manifest", mode: "--update", existing: "old", manifest: "invalid", wantError: true},
		{name: "failed manifest download", mode: "--check-update", existing: "old", manifest: "download-error", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			destination, mocks := filepath.Join(dir, "bin with spaces"), filepath.Join(dir, "mocks")
			for _, path := range []string{destination, mocks} {
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, content string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(content), 0755); err != nil {
					t.Fatal(err)
				}
			}
			binary := filepath.Join(destination, "trynas")
			if tc.existing != "" {
				write(binary, tc.existing)
			}
			manifest := fmt.Sprintf("%x  trynas-linux-amd64\n", sha256.Sum256([]byte("latest")))
			if tc.manifest != "" {
				manifest = tc.manifest
			}
			write(filepath.Join(dir, "SHA256SUMS"), manifest)
			payload := tc.payload
			if payload == "" {
				payload = "latest"
			}
			write(filepath.Join(dir, "trynas-linux-amd64"), payload)
			write(filepath.Join(mocks, "uname"), "#!/bin/sh\ncase \"$1\" in -s) echo Linux;; -m) echo x86_64;; esac\n")
			write(filepath.Join(mocks, "curl"), `#!/bin/sh
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output) output=$2; shift 2 ;;
    https://*) url=$1; shift ;;
    *) shift ;;
  esac
done
case "$url" in
  https://github.com/rajibdpi/trynas/releases/latest/download/*) ;;
  *) exit 9 ;;
esac
asset=${url##*/}
echo "$asset" >> "$MOCK_RELEASE/downloads"
if [ "$asset" = SHA256SUMS ] && [ "$(cat "$MOCK_RELEASE/SHA256SUMS")" = download-error ]; then exit 22; fi
cp "$MOCK_RELEASE/$asset" "$output"
`)
			cmd := exec.Command(bash, script, tc.mode, "--dir", destination)
			cmd.Env = append(os.Environ(), "PATH="+mocks+string(os.PathListSeparator)+os.Getenv("PATH"), "MOCK_RELEASE="+dir)
			output, err := cmd.CombinedOutput()
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v, output=%s", err, output)
			}
			if tc.message != "" && !strings.Contains(string(output), tc.message) {
				t.Fatalf("missing message %q: %s", tc.message, output)
			}
			downloads, err := os.ReadFile(filepath.Join(dir, "downloads"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(downloads), "trynas-linux-amd64") != tc.wantBinary {
				t.Fatalf("unexpected downloads: %s", downloads)
			}
			want := tc.existing
			if tc.wantBinary && !tc.wantError {
				want = "latest"
			}
			installed, err := os.ReadFile(binary)
			if want == "" {
				if !os.IsNotExist(err) {
					t.Fatalf("check created an installation: %v", err)
				}
			} else if err != nil || string(installed) != want {
				t.Fatalf("installed=%q, error=%v; want %q", installed, err, want)
			}
		})
	}
	for _, args := range [][]string{
		{"--update", "--check-update"},
		{"--update", "--source", "."},
		{"--check-update", "--version", "v1.0.0"},
	} {
		if output, err := exec.Command(bash, append([]string{script}, args...)...).CombinedOutput(); err == nil {
			t.Fatalf("accepted conflicting options %v: %s", args, output)
		}
	}
}

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

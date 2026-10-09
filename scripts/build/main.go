// Command build compiles the frontend and standalone TryNAS release binaries.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

var supported = []string{"linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64", "darwin/amd64", "darwin/arm64"}

func main() {
	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
}

func targets(all bool, value string) ([]string, error) {
	if all && value != "" {
		return nil, fmt.Errorf("use either --all or --target")
	}
	if all {
		return append([]string(nil), supported...), nil
	}
	if value == "" {
		value = runtime.GOOS + "/" + runtime.GOARCH
	}
	seen := map[string]bool{}
	result := []string{}
	for _, target := range strings.Split(value, ",") {
		target = strings.TrimSpace(target)
		valid := false
		for _, candidate := range supported {
			valid = valid || target == candidate
		}
		if !valid {
			return nil, fmt.Errorf("unsupported target %q; supported: %s", target, strings.Join(supported, ", "))
		}
		if !seen[target] {
			result = append(result, target)
			seen[target] = true
		}
	}
	return result, nil
}

func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.HasPrefix(string(data), "module github.com/rajibdpi/trynas\n") {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("run from inside the TryNAS project")
		}
		dir = parent
	}
}

func run(dir, command string, env map[string]string, args ...string) error {
	if runtime.GOOS == "windows" && command == "pnpm" {
		args = append([]string{"/d", "/s", "/c", "pnpm"}, args...)
		command = "cmd.exe"
	}
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", command, err)
	}
	return nil
}

func artifactName(target string) string {
	name := "trynas-" + strings.ReplaceAll(target, "/", "-")
	if strings.HasPrefix(target, "windows/") {
		name += ".exe"
	}
	return name
}

func checksums(dir string) error {
	lines := []string{}
	for _, target := range supported {
		name := artifactName(target)
		file, err := os.Open(filepath.Join(dir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, err = io.Copy(hash, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		lines = append(lines, fmt.Sprintf("%x  %s", hash.Sum(nil), name))
	}
	sort.Strings(lines)
	return os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func build() error {
	all := flag.Bool("all", false, "Build all six supported OS/architecture targets")
	target := flag.String("target", "", "Comma-separated targets (default: current OS/architecture)")
	skipFrontend := flag.Bool("skip-frontend", false, "Reuse frontend/dist; only rebuild Go binaries")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flag.Args(), " "))
	}
	selected, err := targets(*all, *target)
	if err != nil {
		return err
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	frontend, dist := filepath.Join(root, "frontend"), filepath.Join(root, "dist")
	if !*skipFrontend {
		if _, err := os.Stat(filepath.Join(frontend, "node_modules")); os.IsNotExist(err) {
			if err := run(frontend, "pnpm", nil, "install", "--frozen-lockfile"); err != nil {
				return err
			}
		}
		for _, name := range []string{".next", "dist"} {
			if err := os.RemoveAll(filepath.Join(frontend, name)); err != nil {
				return err
			}
		}
		fmt.Println("Building frontend from clean release output...")
		if err := run(frontend, "pnpm", map[string]string{"TRYNAS_EMBEDDED_BUILD": "true", "NEXT_PUBLIC_EMBEDDED": "true", "NEXT_PUBLIC_API_URL": ""}, "build"); err != nil {
			return err
		}
	} else {
		fmt.Println("Reusing existing frontend/dist assets; frontend source changes are not rebuilt.")
	}
	info, err := os.Stat(filepath.Join(frontend, "dist", "index.html"))
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("frontend/dist/index.html is missing; run without --skip-frontend")
	}
	if err := os.MkdirAll(dist, 0755); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dist, "admin-credentials.txt")); err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, target := range selected {
		parts := strings.Split(target, "/")
		fmt.Printf("Building %s...\n", target)
		// Publish each binary only after a successful build, preserving an existing
		// release executable if compilation fails.
		temp, err := os.CreateTemp(dist, ".trynas-build-*")
		if err != nil {
			return err
		}
		tempPath := temp.Name()
		if err := temp.Close(); err != nil {
			os.Remove(tempPath)
			return err
		}
		err = run(root, "go", map[string]string{"CGO_ENABLED": "0", "GOOS": parts[0], "GOARCH": parts[1]}, "build", "-trimpath", "-ldflags=-s -w", "-o", tempPath, ".")
		if err == nil {
			err = os.Rename(tempPath, filepath.Join(dist, artifactName(target)))
		}
		if err != nil {
			os.Remove(tempPath)
			return err
		}
	}
	if err := checksums(dist); err != nil {
		return err
	}
	fmt.Printf("Release binaries and SHA256SUMS ready in %s\n", dist)
	fmt.Println("Fresh installations require ADMIN_PASSWORD at runtime. Restart the executable to use this release.")
	return nil
}

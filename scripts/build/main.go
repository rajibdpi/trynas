// Command build compiles the frontend and standalone TryNAS release binaries.
package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rajibdpi/trynas/core"
	"github.com/rajibdpi/trynas/scripts/internal/tooling"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if err := build(); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
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
	adminHash, adminEncoded, err := buildAdminCredentials(root)
	if err != nil {
		return err
	}
	frontend, dist := filepath.Join(root, "frontend"), filepath.Join(root, "dist")
	if !*skipFrontend {
		if _, err := os.Stat(filepath.Join(frontend, "node_modules")); os.IsNotExist(err) {
			if err := tooling.Command(frontend, "pnpm", "install", "--frozen-lockfile").Run(); err != nil {
				return err
			}
		}
		for _, name := range []string{".next", "dist"} {
			if err := os.RemoveAll(filepath.Join(frontend, name)); err != nil {
				return err
			}
		}
		fmt.Println("Building frontend from clean release output...")
		if err := buildFrontend(frontend); err != nil {
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
		err = buildBinary(root, tempPath, parts[0], parts[1], adminHash, adminEncoded)
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
	fmt.Println("Build-time .env ADMIN_PASSWORD hash included for first startup; runtime ADMIN_PASSWORD overrides it. Existing accounts are unchanged.")
	return nil
}

func buildFrontend(dir string) error {
	cmd := tooling.Command(dir, "pnpm", "build")
	cmd.Env = append(cmd.Env, "TRYNAS_EMBEDDED_BUILD=true", "NEXT_PUBLIC_EMBEDDED=true", "NEXT_PUBLIC_API_URL=")
	return cmd.Run()
}

func buildAdminCredentials(root string) (string, string, error) {
	values, err := godotenv.Read(filepath.Join(root, ".env"))
	if err != nil {
		return "", "", fmt.Errorf("read build .env: %w", err)
	}
	password := values["ADMIN_PASSWORD"]
	if !core.ValidAdminPassword(password) {
		return "", "", fmt.Errorf("set ADMIN_PASSWORD in .env to a strong 8–72 byte password before building")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(hash), base64.StdEncoding.EncodeToString([]byte(password)), err
}

func buildBinary(root, output, goos, arch, adminHash, adminEncoded string) error {
	flags := "-s -w -X github.com/rajibdpi/trynas/core.BuildAdminPasswordHash=" + adminHash
	flags += " -X github.com/rajibdpi/trynas/core.BuildAdminPasswordEncoded=" + adminEncoded
	cmd := tooling.Command(root, "go", "build", "-trimpath", "-ldflags="+flags, "-o", output, ".")
	cmd.Env = append(cmd.Env, "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+arch)
	return cmd.Run()
}

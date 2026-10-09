// Command install installs a checksum-verified TryNAS release binary.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const repository = "rajibdpi/trynas"

func main() {
	if err := install(); err != nil {
		fmt.Fprintln(os.Stderr, "install:", err)
		os.Exit(1)
	}
}

func names(goos, arch string) (string, string, error) {
	if (goos != "linux" && goos != "darwin" && goos != "windows") || (arch != "amd64" && arch != "arm64") {
		return "", "", fmt.Errorf("unsupported platform %s/%s", goos, arch)
	}
	asset, executable := "trynas-"+goos+"-"+arch, "trynas"
	if goos == "windows" {
		asset += ".exe"
		executable += ".exe"
	}
	return asset, executable, nil
}

func defaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "TryNAS", "bin"), nil
	}
	return filepath.Join(home, ".local", "bin"), nil
}

func download(ctx context.Context, url string, dest io.Writer, limit int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "TryNAS-installer")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d (check that release assets have been published)", url, response.StatusCode)
	}
	n, err := io.Copy(dest, io.LimitReader(response.Body, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
		return fmt.Errorf("download exceeds %d bytes", limit)
	}
	return nil
}

func expectedHash(manifest []byte, asset string) (string, error) {
	found := ""
	for _, line := range strings.Split(string(manifest), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != asset {
			continue
		}
		decoded, err := hex.DecodeString(fields[0])
		if err != nil || len(decoded) != sha256.Size || found != "" {
			return "", fmt.Errorf("invalid or duplicate checksum for %s", asset)
		}
		found = strings.ToLower(fields[0])
	}
	if found == "" {
		return "", fmt.Errorf("SHA256SUMS has no checksum for %s", asset)
	}
	return found, nil
}

func copyVerified(source, destination, expected string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	out, err := os.CreateTemp(filepath.Dir(destination), ".trynas-install-*")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, hash), in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != expected {
		return fmt.Errorf("checksum mismatch; existing installation was not replaced")
	}
	if err := os.Chmod(out.Name(), 0755); err != nil {
		return err
	}
	return os.Rename(out.Name(), destination)
}

func install() error {
	defaults, err := defaultDir()
	if err != nil {
		return err
	}
	dir := flag.String("dir", defaults, "Installation directory")
	source := flag.String("source", "", "Local release directory containing binaries and SHA256SUMS")
	version := flag.String("version", "latest", "GitHub release tag, or latest")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if *source != "" && *version != "latest" {
		return fmt.Errorf("--version cannot be combined with --source")
	}
	asset, executable, err := names(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	releaseDir := *source
	if releaseDir == "" {
		if *version == "" || strings.ContainsAny(*version, "/\\?# \r\n") {
			return fmt.Errorf("invalid release tag")
		}
		temp, err := os.MkdirTemp("", "trynas-download-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(temp)
		releaseDir = temp
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		base := "https://github.com/" + repository + "/releases/latest/download/"
		if *version != "latest" {
			base = "https://github.com/" + repository + "/releases/download/" + *version + "/"
		}
		for _, name := range []string{"SHA256SUMS", asset} {
			f, err := os.Create(filepath.Join(temp, name))
			if err != nil {
				return err
			}
			limit := int64(1 << 30)
			if name == "SHA256SUMS" {
				limit = 1 << 20
			}
			err = download(ctx, base+name, f, limit)
			closeErr := f.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	manifest, err := os.ReadFile(filepath.Join(releaseDir, "SHA256SUMS"))
	if err != nil {
		return err
	}
	expected, err := expectedHash(manifest, asset)
	if err != nil {
		return err
	}
	destination, err := filepath.Abs(filepath.Join(*dir, executable))
	if err != nil {
		return err
	}
	if err := copyVerified(filepath.Join(releaseDir, asset), destination, expected); err != nil {
		return err
	}
	fmt.Printf("Installed %s (SHA-256 verified).\n", destination)
	fmt.Println("Add its directory to PATH. Configure ADMIN_PASSWORD before first launch; existing accounts and data are unchanged.")
	return nil
}

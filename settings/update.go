package settings

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rajibdpi/trynas/core"
)

const releaseRepository = "https://github.com/rajibdpi/trynas/releases"

var releaseTag = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type releaseUpdater struct {
	mu                               sync.Mutex
	client                           *http.Client
	path, asset, runningHash, reason string
	pending                          bool
}

type updateStatus struct {
	LatestVersion   string `json:"latestVersion,omitempty"`
	ReleaseURL      string `json:"releaseUrl,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
	CanUpdate       bool   `json:"canUpdate"`
	Reason          string `json:"reason,omitempty"`
	RestartRequired bool   `json:"restartRequired"`
}

func newReleaseUpdater() *releaseUpdater {
	u := &releaseUpdater{client: &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return fmt.Errorf("release downloads require HTTPS")
		}
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	}}}
	u.path, _ = os.Executable()
	u.asset = "trynas-" + runtime.GOOS + "-" + runtime.GOARCH
	switch {
	case runtime.GOOS != "linux" && runtime.GOOS != "darwin":
		u.reason = "In-app updates support standalone Linux and macOS installations. Use your platform installer."
	case runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64":
		u.reason = "No release binary is available for this CPU architecture."
	case fileExists("/.dockerenv") || fileExists("/run/.containerenv"):
		u.reason = "Update the container image and redeploy to install a new release."
	case filepath.Base(u.path) != "trynas" && filepath.Base(u.path) != u.asset:
		u.reason = "Use an installed standalone TryNAS binary to enable in-app updates."
	}
	if u.reason == "" {
		var err error
		u.runningHash, err = hashExecutable(u.path)
		if err != nil {
			u.reason = "The running executable could not be read. Use the command-line installer."
		}
	}
	return u
}
func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }
func hashExecutable(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (u *releaseUpdater) download(ctx context.Context, url string, dest io.Writer, limit int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "TryNAS-updater")
	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach GitHub releases: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub releases returned HTTP %d", resp.StatusCode)
	}
	n, err := io.Copy(dest, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
		return fmt.Errorf("release download exceeds size limit")
	}
	return nil
}
func (u *releaseUpdater) latest(ctx context.Context) (updateStatus, string, string, error) {
	status := updateStatus{CanUpdate: u.reason == "", Reason: u.reason, RestartRequired: u.pending}
	var data bytes.Buffer
	if err := u.download(ctx, "https://api.github.com/repos/rajibdpi/trynas/releases/latest", &data, 1<<20); err != nil {
		return status, "", "", err
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.Unmarshal(data.Bytes(), &release); err != nil || !releaseTag.MatchString(release.Tag) || release.Draft || release.Prerelease {
		return status, "", "", fmt.Errorf("invalid latest release metadata")
	}
	status.LatestVersion = release.Tag
	status.ReleaseURL = releaseRepository + "/tag/" + release.Tag
	if !status.CanUpdate {
		return status, "", "", nil
	}
	data.Reset()
	base := releaseRepository + "/download/" + release.Tag
	if err := u.download(ctx, base+"/SHA256SUMS", &data, 1<<20); err != nil {
		return status, "", "", err
	}
	expected := ""
	for _, line := range strings.Split(data.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != u.asset {
			continue
		}
		decoded, err := hex.DecodeString(fields[0])
		if err != nil || len(decoded) != sha256.Size || expected != "" {
			return status, "", "", fmt.Errorf("invalid or duplicate release checksum")
		}
		expected = strings.ToLower(fields[0])
	}
	if expected == "" {
		return status, "", "", fmt.Errorf("release has no checksum for this platform")
	}
	installed, err := hashExecutable(u.path)
	if err != nil {
		return status, "", "", fmt.Errorf("could not read installed executable")
	}
	status.UpdateAvailable = installed != expected
	status.RestartRequired = u.pending || installed != u.runningHash
	return status, base + "/" + u.asset, expected, nil
}

// Install beside the actual executable, preserving its permissions and replacing
// it only after the pinned release's checksum has been verified. No shell runs.
func (u *releaseUpdater) install(ctx context.Context, url, expected string) error {
	original, err := os.Stat(u.path)
	if err != nil {
		return err
	}
	staged, err := os.CreateTemp(filepath.Dir(u.path), ".trynas-update-*")
	if err != nil {
		return fmt.Errorf("executable directory is not writable; update with the command-line installer")
	}
	defer os.Remove(staged.Name())
	defer staged.Close()
	hash := sha256.New()
	if err := u.download(ctx, url, io.MultiWriter(staged, hash), 512<<20); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return fmt.Errorf("checksum mismatch; existing installation was not changed")
	}
	if err := staged.Chmod(original.Mode().Perm()); err != nil {
		return err
	}
	if err := staged.Sync(); err != nil {
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(staged.Name(), u.path); err != nil {
		return fmt.Errorf("could not replace executable; use the command-line installer")
	}
	u.pending = true
	return nil
}

func (a *Module) Updates(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(core.UserKey).(core.User)
	if !ok || user.Role != "admin" {
		core.Bad(w, "administrator required", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		core.Bad(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	a.updateOnce.Do(func() { a.updater = newReleaseUpdater() })
	u := a.updater
	if !u.mu.TryLock() {
		core.Bad(w, "an update operation is already in progress", http.StatusConflict)
		return
	}
	defer u.mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	status, url, expected, err := u.latest(ctx)
	if err != nil {
		core.Bad(w, err.Error(), http.StatusBadGateway)
		return
	}
	if r.Method == http.MethodPost {
		if !status.CanUpdate {
			core.Bad(w, status.Reason, http.StatusConflict)
			return
		}
		if status.UpdateAvailable {
			if err := u.install(ctx, url, expected); err != nil {
				core.Bad(w, err.Error(), http.StatusInternalServerError)
				return
			}
			status.UpdateAvailable = false
			status.RestartRequired = true
			a.Log(user.ID, "update", status.LatestVersion, user.IP)
		}
	}
	core.JSON(w, status)
}

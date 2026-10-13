package app

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rajibdpi/trynas/core"
)

type countedUpload struct {
	*bytes.Reader
	read int
}

func (r *countedUpload) Read(p []byte) (int, error) {
	n, e := r.Reader.Read(p)
	r.read += n
	return n, e
}

func TestUploadStopsReadingAtQuota(t *testing.T) {
	a, _, user := setup(t)
	if err := os.Mkdir(filepath.Join(a.Root, "shared"), 0700); err != nil {
		t.Fatal(err)
	}
	grant(t, a, user, "shared", "write")
	if _, err := a.DB.Exec("INSERT INTO user_limits(user_id,quota_bytes) VALUES(?,32) ON CONFLICT(user_id) DO UPDATE SET quota_bytes=32", user.ID); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	_ = form.WriteField("path", "shared")
	part, _ := form.CreateFormFile("files", "oversized.txt")
	_, _ = io.Copy(part, strings.NewReader(strings.Repeat("x", 1<<20)))
	_ = form.Close()
	counted := &countedUpload{Reader: bytes.NewReader(body.Bytes())}
	r := httptest.NewRequest("POST", "/api/files/upload", counted)
	r.Header.Set("Content-Type", form.FormDataContentType())
	r = r.WithContext(context.WithValue(r.Context(), core.UserKey, user))
	w := httptest.NewRecorder()
	a.Guard(http.HandlerFunc(a.UploadV5)).ServeHTTP(w, r)
	expect(t, w, 413)
	if counted.read >= body.Len() {
		t.Fatal("read entire over-quota upload")
	}
	if _, err := os.Stat(filepath.Join(a.Root, "shared", "oversized.txt")); !os.IsNotExist(err) {
		t.Fatal("over-quota file installed")
	}
	stages, _ := filepath.Glob(filepath.Join(a.Root, ".trynas-upload-*"))
	if len(stages) != 0 {
		t.Fatal("staging files leaked")
	}
}

func TestSlowUploadDoesNotHoldMutationLock(t *testing.T) {
	a, _, user := setup(t)
	if err := os.Mkdir(filepath.Join(a.Root, "shared"), 0700); err != nil {
		t.Fatal(err)
	}
	grant(t, a, user, "shared", "write")
	input, output := io.Pipe()
	defer input.Close()
	defer output.Close()
	form := multipart.NewWriter(output)
	r := httptest.NewRequest("POST", "/api/files/upload", input)
	r.Header.Set("Content-Type", form.FormDataContentType())
	r = r.WithContext(context.WithValue(r.Context(), core.UserKey, user))
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); a.Guard(http.HandlerFunc(a.UploadV5)).ServeHTTP(w, r) }()
	headers := make(chan error, 1)
	go func() {
		if err := form.WriteField("path", "shared"); err != nil {
			headers <- err
			return
		}
		_, err := form.CreateFormFile("files", "slow.txt")
		headers <- err
	}()
	select {
	case err := <-headers:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("upload did not read headers")
	}
	if !a.Mutation.TryLock() {
		t.Fatal("slow upload holds global mutation lock")
	}
	a.Mutation.Unlock()
	_ = output.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("upload did not cancel")
	}
}

func TestDownloadDoesNotRevealUnauthorizedFileExistence(t *testing.T) {
	a, _, user := setup(t)
	if err := os.Mkdir(filepath.Join(a.Root, "shared"), 0700); err != nil {
		t.Fatal(err)
	}
	grant(t, a, user, "shared", "write")
	write(t, a, "private.txt", "secret")
	for _, path := range []string{"private.txt", "missing-private.txt"} {
		r := httptest.NewRequest("GET", "/api/files/download?path="+path, nil)
		r = r.WithContext(context.WithValue(r.Context(), core.UserKey, user))
		w := httptest.NewRecorder()
		a.Guard(http.HandlerFunc(a.Download)).ServeHTTP(w, r)
		expect(t, w, 403)
	}
}

func TestUploadMultipartFieldOrderCompatibility(t *testing.T) {
	for _, pathFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "path-first", false: "path-last"}[pathFirst], func(t *testing.T) {
			a, _, user := setup(t)
			if err := os.Mkdir(filepath.Join(a.Root, "shared"), 0700); err != nil {
				t.Fatal(err)
			}
			grant(t, a, user, "shared", "write")
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			if pathFirst {
				_ = form.WriteField("path", "shared")
			}
			part, _ := form.CreateFormFile("file", "compatible.txt")
			_, _ = io.WriteString(part, "content")
			if !pathFirst {
				_ = form.WriteField("path", "shared")
			}
			_ = form.Close()
			r := httptest.NewRequest("POST", "/api/files/upload", &body)
			r.Header.Set("Content-Type", form.FormDataContentType())
			r = r.WithContext(context.WithValue(r.Context(), core.UserKey, user))
			w := httptest.NewRecorder()
			a.Guard(http.HandlerFunc(a.UploadV5)).ServeHTTP(w, r)
			expect(t, w, 200)
			data, err := os.ReadFile(filepath.Join(a.Root, "shared", "compatible.txt"))
			if err != nil || string(data) != "content" {
				t.Fatal("upload field order changed contents", err)
			}
		})
	}
}

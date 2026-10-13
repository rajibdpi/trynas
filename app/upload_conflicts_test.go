package app

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rajibdpi/trynas/core"
)

func uploadWithChoices(a *App, u core.User, files map[string]string, choices map[string]string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	_ = form.WriteField("path", "shared")
	policy, _ := json.Marshal(choices)
	_ = form.WriteField("conflicts", string(policy))
	for name, data := range files {
		part, _ := form.CreateFormFile("files", name)
		_, _ = part.Write([]byte(data))
	}
	_ = form.Close()
	r := httptest.NewRequest("POST", "/api/files/upload", &body)
	r.Header.Set("Content-Type", form.FormDataContentType())
	r = r.WithContext(context.WithValue(r.Context(), core.UserKey, u))
	w := httptest.NewRecorder()
	a.Guard(http.HandlerFunc(a.UploadV5)).ServeHTTP(w, r)
	return w
}

func TestUploadConflictChoicesAndNumbering(t *testing.T) {
	a, _, u := setup(t)
	_ = os.Mkdir(filepath.Join(a.Root, "shared"), 0700)
	grant(t, a, u, "shared", "write")
	expect(t, uploadRequest(a, u, map[string]string{"report.txt": "old"}), 200)
	// Same byte size requires an explicit decision, even if contents differ.
	expect(t, uploadRequest(a, u, map[string]string{"report.txt": "new"}), 409)
	check := jsonRequest(a, u, "POST", "/api/files/upload/check", map[string]any{"path": "shared", "files": []map[string]any{{"name": "report.txt", "size": 3}, {"name": "different.txt", "size": 1}}}, a.UploadConflicts)
	expect(t, check, 200)
	var result struct {
		Conflicts []struct {
			Name string
			Size int64
		}
	}
	if err := json.Unmarshal(check.Body.Bytes(), &result); err != nil || len(result.Conflicts) != 1 || result.Conflicts[0].Name != "report.txt" {
		t.Fatal("incorrect conflict manifest", err)
	}
	expect(t, uploadWithChoices(a, u, map[string]string{"report.txt": "new"}, map[string]string{"report.txt": "keep"}), 200)
	expect(t, uploadRequest(a, u, map[string]string{"report.txt": "longer"}), 200)
	for name, want := range map[string]string{"report.txt": "old", "report(1).txt": "new", "report(2).txt": "longer"} {
		data, err := os.ReadFile(filepath.Join(a.Root, "shared", name))
		if err != nil || string(data) != want {
			t.Fatalf("%s contents=%q err=%v", name, data, err)
		}
	}
	expect(t, uploadWithChoices(a, u, map[string]string{"report.txt": "NEW"}, map[string]string{"report.txt": "replace"}), 200)
	data, _ := os.ReadFile(filepath.Join(a.Root, "shared/report.txt"))
	if string(data) != "NEW" {
		t.Fatal("replacement failed")
	}
	// A stale replace selection must not overwrite a different-size existing file.
	expect(t, uploadWithChoices(a, u, map[string]string{"report.txt": "much longer"}, map[string]string{"report.txt": "replace"}), 200)
	data, _ = os.ReadFile(filepath.Join(a.Root, "shared/report.txt"))
	if string(data) != "NEW" {
		t.Fatal("different-size upload overwrote existing file")
	}
	if _, err := os.Stat(filepath.Join(a.Root, "shared/report(3).txt")); err != nil {
		t.Fatal(err)
	}
}

func TestUploadReplacementReusesQuotaAndRollsBack(t *testing.T) {
	a, _, u := setup(t)
	_ = os.Mkdir(filepath.Join(a.Root, "shared"), 0700)
	grant(t, a, u, "shared", "write")
	_, _ = a.DB.Exec("INSERT INTO user_limits(user_id,quota_bytes) VALUES(?,4)", u.ID)
	expect(t, uploadRequest(a, u, map[string]string{"a.txt": "orig"}), 200)
	expect(t, uploadWithChoices(a, u, map[string]string{"a.txt": "next"}, map[string]string{"a.txt": "replace"}), 200)
	used, _, _ := a.Usage(u.ID)
	if used != 4 {
		t.Fatalf("replacement double-counted quota: %d", used)
	}
	_, err := a.DB.Exec(`CREATE TRIGGER fail_replace BEFORE INSERT ON file_owners WHEN NEW.path='shared/a.txt' BEGIN SELECT RAISE(FAIL,'injected failure'); END;`)
	if err != nil {
		t.Fatal(err)
	}
	failed := uploadWithChoices(a, u, map[string]string{"a.txt": "FAIL"}, map[string]string{"a.txt": "replace"})
	if failed.Code < 400 {
		t.Fatal("injected failure was ignored")
	}
	data, err := os.ReadFile(filepath.Join(a.Root, "shared/a.txt"))
	if err != nil || string(data) != "next" {
		t.Fatal("failed replacement lost original", err)
	}
	used, _, _ = a.Usage(u.ID)
	if used != 4 {
		t.Fatal("failed replacement changed quota")
	}
}

func TestConcurrentDifferentSizeUploadsGetDistinctNames(t *testing.T) {
	a, _, u := setup(t)
	_ = os.Mkdir(filepath.Join(a.Root, "shared"), 0700)
	grant(t, a, u, "shared", "write")
	expect(t, uploadRequest(a, u, map[string]string{"file.txt": "x"}), 200)
	results := make(chan int, 2)
	for _, data := range []string{"ab", "abc"} {
		go func(data string) { results <- uploadRequest(a, u, map[string]string{"file.txt": data}).Code }(data)
	}
	for i := 0; i < 2; i++ {
		if code := <-results; code != 200 {
			t.Fatalf("concurrent upload failed: %d", code)
		}
	}
	for _, name := range []string{"file.txt", "file(1).txt", "file(2).txt"} {
		if _, err := os.Stat(filepath.Join(a.Root, "shared", name)); err != nil {
			t.Fatal(err)
		}
	}
	used, _, _ := a.Usage(u.ID)
	if used != 6 {
		t.Fatalf("incorrect concurrent quota: %d", used)
	}
}

func TestUploadDoesNotReplaceDirectoriesOrExposeDeniedPaths(t *testing.T) {
	a, _, u := setup(t)
	_ = os.MkdirAll(filepath.Join(a.Root, "shared/folder"), 0700)
	grant(t, a, u, "shared", "write")
	expect(t, uploadRequest(a, u, map[string]string{"folder": "x"}), 409)
	expect(t, jsonRequest(a, u, "POST", "/api/files/upload/check", map[string]any{"path": "private", "files": []map[string]any{{"name": "secret.txt", "size": 1}}}, a.UploadConflicts), 403)
}

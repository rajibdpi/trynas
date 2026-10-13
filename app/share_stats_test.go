package app

import (
	"encoding/json"
	chi "github.com/go-chi/chi/v5"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSharedLinkVisitAndDownloadCounts(t *testing.T) {
	a, _, user := setup(t)
	write(t, a, "shared/file.txt", "shared contents")
	grant(t, a, user, "shared", "read")
	router := chi.NewRouter()
	router.Get("/s/{token}", a.PublicShare)
	router.Head("/s/{token}", a.PublicShare)
	for _, path := range []string{"shared/file.txt", "shared"} {
		created := jsonRequest(a, user, "POST", "/api/shares", map[string]any{"path": path, "days": 7}, a.CreateShare)
		expect(t, created, 200)
		var shared struct{ Token string }
		if err := json.Unmarshal(created.Body.Bytes(), &shared); err != nil {
			t.Fatal(err)
		}
		get := func(method, query, header string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(method, "http://example.com/s/"+shared.Token+query, nil)
			if header != "" {
				req.Header.Set("Range", header)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			return response
		}
		expect(t, get("GET", "", ""), 200)
		expect(t, get("GET", "", ""), 200)
		expect(t, get("GET", "?download=1", ""), 200)
		expect(t, get("GET", "?qr=1&download=1", ""), 200)
		expect(t, get("HEAD", "", ""), 200)
		expect(t, get("GET", "?download=1&item=not-shared.txt", ""), 404)
		if strings.HasSuffix(path, ".txt") {
			expect(t, get("GET", "?download=1", "bytes=0-2"), 206)
		}
		var visits, downloads int
		if err := a.DB.QueryRow("SELECT visits,downloads FROM share_stats WHERE share_id=(SELECT id FROM shares WHERE token=?)", shared.Token).Scan(&visits, &downloads); err != nil {
			t.Fatal(err)
		}
		if visits != 2 || downloads != 1 {
			t.Fatalf("wrong counts for %s: visits=%d downloads=%d", path, visits, downloads)
		}
	}
	response := request(a, user, "GET", "/api/shares", nil, a.Shares)
	expect(t, response, 200)
	var shares []struct {
		Visits    int
		Downloads int
	}
	if err := json.Unmarshal(response.Body.Bytes(), &shares); err != nil {
		t.Fatal(err)
	}
	if len(shares) != 2 {
		t.Fatal("missing shares")
	}
	for _, share := range shares {
		if share.Visits != 2 || share.Downloads != 1 {
			t.Fatal("missing counts in shared links response")
		}
	}
}

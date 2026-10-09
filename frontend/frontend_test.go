package frontend

import (
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestFileLocationRoutes(t *testing.T) {
	handler := StaticFrontend(fstest.MapFS{"files.html": {Data: []byte("file browser")}})
	for _, location := range []string{"/files/", "/files/Docs/", "/files/rajib/Test/photo.jpg", "/files/My%20Docs/"} {
		t.Run(location, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", location, nil))
			if response.Code != 200 || response.Body.String() != "file browser" {
				t.Fatalf("got %d %q", response.Code, response.Body.String())
			}
		})
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/other/missing", nil))
	if response.Code != 404 {
		t.Fatalf("missing route returned %d", response.Code)
	}
}

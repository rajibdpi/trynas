package app

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rajibdpi/trynas/core"
)

func TestLogoUpload(t *testing.T) {
	a, admin, user := setup(t)
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	upload := func(u core.User, data []byte) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		file, err := writer.CreateFormFile("logo", "logo.png")
		if err != nil {
			t.Fatal(err)
		}
		file.Write(data)
		writer.Close()
		r := httptest.NewRequest("POST", "/api/settings/logo", &body)
		r.Header.Set("Content-Type", writer.FormDataContentType())
		r.Header.Set("Origin", "http://localhost:3000")
		r = r.WithContext(context.WithValue(r.Context(), core.UserKey, u))
		w := httptest.NewRecorder()
		a.Cors(http.HandlerFunc(a.UploadLogo)).ServeHTTP(w, r)
		return w
	}
	expect(t, upload(user, imageData.Bytes()), 403)
	expect(t, upload(admin, []byte("<svg>not a raster image</svg>")), 400)
	expect(t, upload(admin, imageData.Bytes()), 200)
	w := httptest.NewRecorder()
	a.Logo(w, httptest.NewRequest("GET", "/api/branding/logo", nil))
	expect(t, w, 200)
	if w.Header().Get("Content-Type") != "image/png" || !bytes.Equal(w.Body.Bytes(), imageData.Bytes()) {
		t.Fatal("uploaded logo did not round-trip")
	}
	expect(t, upload(admin, bytes.Repeat([]byte("x"), 2<<20+1)), 400)
}

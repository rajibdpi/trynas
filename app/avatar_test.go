package app

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/rajibdpi/trynas/core"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProfileAvatarUploadAndIsolation(t *testing.T) {
	a, admin, user := setup(t)
	upload := func(data []byte) *httptest.ResponseRecorder {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, err := form.CreateFormFile("avatar", "avatar.png")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(data)
		_ = form.Close()
		r := httptest.NewRequest("POST", "/api/profile/avatar", &body)
		r.Header.Set("Content-Type", form.FormDataContentType())
		r = r.WithContext(context.WithValue(r.Context(), core.UserKey, user))
		w := httptest.NewRecorder()
		a.Cors(a.Guard(http.HandlerFunc(a.UploadAvatar))).ServeHTTP(w, r)
		return w
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	saved := upload(imageData.Bytes())
	expect(t, saved, 200)
	var profile struct {
		AvatarURL string `json:"avatarUrl"`
	}
	if err := json.Unmarshal(saved.Body.Bytes(), &profile); err != nil {
		t.Fatal(err)
	}
	if profile.AvatarURL == "" {
		t.Fatal("missing avatar URL")
	}
	loaded := request(a, user, "GET", profile.AvatarURL, nil, a.Avatar)
	expect(t, loaded, 200)
	if loaded.Header().Get("Content-Type") != "image/png" || !bytes.Equal(loaded.Body.Bytes(), imageData.Bytes()) {
		t.Fatal("wrong image response")
	}
	expect(t, request(a, admin, "GET", profile.AvatarURL, nil, a.Avatar), 404)
	expect(t, upload([]byte("<svg>invalid</svg>")), 400)
	expect(t, upload(make([]byte, 2*1024*1024+1)), 400)
	removed := request(a, user, "DELETE", "/api/profile/avatar", nil, a.RemoveAvatar)
	expect(t, removed, 200)
	expect(t, request(a, user, "GET", profile.AvatarURL, nil, a.Avatar), 404)
}

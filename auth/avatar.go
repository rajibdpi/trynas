package auth

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/rajibdpi/trynas/core"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
)

func (a *Module) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	const limit = 2 << 20
	r.Body = http.MaxBytesReader(w, r.Body, limit+65536)
	if err := r.ParseMultipartForm(limit); err != nil {
		core.Bad(w, "image must be at most 2 MiB", 400)
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, _, err := r.FormFile("avatar")
	if err != nil {
		core.Bad(w, "choose an image", 400)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || len(data) > limit {
		core.Bad(w, "image must be at most 2 MiB", 400)
		return
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width > 4096 || config.Height > 4096 {
		core.Bad(w, "choose a PNG, JPEG, or GIF up to 4096 × 4096 pixels", 400)
		return
	}
	mime := map[string]string{"png": "image/png", "jpeg": "image/jpeg", "gif": "image/gif"}[format]
	if mime == "" {
		core.Bad(w, "unsupported image", 400)
		return
	}
	version := fmt.Sprintf("%x", sha256.Sum256(data))
	if _, err := a.DB.Exec("INSERT INTO user_avatars(user_id,mime,data,version) VALUES(?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET mime=excluded.mime,data=excluded.data,version=excluded.version", u.ID, mime, data, version); err != nil {
		core.Bad(w, "could not save image", 500)
		return
	}
	core.JSON(w, a.Profile(u))
}

func (a *Module) Avatar(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	var mime string
	var data []byte
	if err := a.DB.QueryRow("SELECT mime,data FROM user_avatars WHERE user_id=?", u.ID).Scan(&mime, &data); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-cache")
	_, _ = w.Write(data)
}

func (a *Module) RemoveAvatar(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	if _, err := a.DB.Exec("DELETE FROM user_avatars WHERE user_id=?", u.ID); err != nil {
		core.Bad(w, "could not remove image", 500)
		return
	}
	core.JSON(w, a.Profile(u))
}

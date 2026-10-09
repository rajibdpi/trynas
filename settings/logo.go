package settings

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"

	"github.com/rajibdpi/trynas/core"
)

const maxLogoBytes = 2 << 20

func (a *Module) UploadLogo(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(core.UserKey).(core.User)
	if user.Role != "admin" {
		core.Bad(w, "administrator required", 403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxLogoBytes+65536)
	if err := r.ParseMultipartForm(maxLogoBytes); err != nil {
		core.Bad(w, "logo must be at most 2 MiB", 400)
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, _, err := r.FormFile("logo")
	if err != nil {
		core.Bad(w, "choose a logo file", 400)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxLogoBytes+1))
	if err != nil || len(data) > maxLogoBytes {
		core.Bad(w, "logo must be at most 2 MiB", 400)
		return
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width > 4096 || config.Height > 4096 {
		core.Bad(w, "choose a PNG, JPEG, or GIF image up to 4096 × 4096 pixels", 400)
		return
	}
	mime := map[string]string{"png": "image/png", "jpeg": "image/jpeg", "gif": "image/gif"}[format]
	if mime == "" {
		core.Bad(w, "unsupported logo image", 400)
		return
	}
	if _, err := a.DB.Exec("INSERT INTO branding_assets(key,mime,data) VALUES('logo',?,?) ON CONFLICT(key) DO UPDATE SET mime=excluded.mime,data=excluded.data", mime, data); err != nil {
		core.Bad(w, "could not save logo", 500)
		return
	}
	a.Log(user.ID, "settings", "logo", user.IP)
	core.JSON(w, map[string]string{"logoUrl": fmt.Sprintf("/api/branding/logo?v=%x", sha256.Sum256(data))})
}

func (a *Module) Logo(w http.ResponseWriter, r *http.Request) {
	var mime string
	var data []byte
	if err := a.DB.QueryRow("SELECT mime,data FROM branding_assets WHERE key='logo'").Scan(&mime, &data); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(data)
}

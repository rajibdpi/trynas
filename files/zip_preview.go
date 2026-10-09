package files

import (
	"archive/zip"
	"net/http"
	"os"

	"github.com/rajibdpi/trynas/core"
)

// ZipContents reads archive metadata without extracting any entries.
func (a *Module) ZipContents(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	p := r.URL.Query().Get("path")
	if !a.Can(u, p, false) {
		core.Bad(w, "file permission denied", 403)
		return
	}
	full, err := core.Safe(a.Root, p)
	if err != nil {
		core.Bad(w, "invalid file path", 400)
		return
	}
	file, err := os.Open(full)
	if err != nil {
		core.Bad(w, "could not open archive", 404)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		core.Bad(w, "select a ZIP file", 400)
		return
	}
	if info.Size() > 1<<30 {
		core.Bad(w, "ZIP preview supports archives up to 1 GiB", 413)
		return
	}
	archive, err := zip.NewReader(file, info.Size())
	if err != nil {
		core.Bad(w, "invalid or unsupported ZIP archive", 400)
		return
	}
	type entry struct {
		Name           string `json:"name"`
		Directory      bool   `json:"directory"`
		Size           uint64 `json:"size"`
		CompressedSize uint64 `json:"compressedSize"`
		Encrypted      bool   `json:"encrypted"`
	}
	entries := make([]entry, 0)
	for _, item := range archive.File {
		if len(entries) == 10000 {
			break
		}
		entries = append(entries, entry{item.Name, item.FileInfo().IsDir(), item.UncompressedSize64, item.CompressedSize64, item.Flags&1 != 0})
	}
	core.JSON(w, map[string]any{"entries": entries, "total": len(archive.File), "truncated": len(archive.File) > len(entries)})
}

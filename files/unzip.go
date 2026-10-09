package files

import (
	"archive/zip"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rajibdpi/trynas/core"
)

func (a *Module) Unzip(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	var input struct{ Path, Destination string }
	if !core.Decode(w, r, &input) {
		return
	}
	if !a.Can(u, input.Path, false) {
		core.Bad(w, "archive permission denied", 403)
		return
	}
	source, err := core.Safe(a.Root, input.Path)
	if err != nil {
		core.Bad(w, "invalid archive path", 400)
		return
	}
	destination, err := core.CleanPath(input.Destination)
	if err != nil || destination == "" {
		core.Bad(w, "choose a destination folder", 400)
		return
	}
	full, err := a.Destination(u, destination)
	if err != nil {
		core.Report(w, err)
		return
	}
	archive, err := zip.OpenReader(source)
	if err != nil {
		core.Bad(w, "invalid or unsupported ZIP", 400)
		return
	}
	defer archive.Close()
	if len(archive.File) > 10000 {
		core.Bad(w, "archive exceeds 10,000 entries", 413)
		return
	}
	var total uint64
	seen := map[string]bool{}
	for _, item := range archive.File {
		clean, err := core.CleanPath(strings.TrimSuffix(item.Name, "/"))
		if err != nil || clean == "" || clean != strings.TrimSuffix(item.Name, "/") || seen[clean] || item.Mode()&os.ModeSymlink != 0 || (!item.FileInfo().IsDir() && !item.Mode().IsRegular()) || item.Flags&1 != 0 {
			core.Bad(w, "archive contains unsafe, duplicate, or encrypted entries", 400)
			return
		}
		seen[clean] = true
		if !a.Can(u, path.Join(destination, clean), true) {
			core.Bad(w, "destination permission denied", 403)
			return
		}
		if item.UncompressedSize64 > 1<<30 || total > (1<<30)-item.UncompressedSize64 {
			core.Bad(w, "extracted contents exceed 1 GiB", 413)
			return
		}
		total += item.UncompressedSize64
	}
	if err := a.CheckQuota(u.ID, int64(total)); err != nil {
		core.Report(w, err)
		return
	}
	stage, err := os.MkdirTemp(filepath.Dir(full), ".trynas-unzip-")
	if err != nil {
		core.Bad(w, "could not prepare extraction", 500)
		return
	}
	defer os.RemoveAll(stage)
	for _, item := range archive.File {
		if r.Context().Err() != nil {
			return
		}
		target := filepath.Join(stage, filepath.FromSlash(item.Name))
		if item.FileInfo().IsDir() {
			err = os.MkdirAll(target, 0755)
		} else {
			err = os.MkdirAll(filepath.Dir(target), 0755)
			if err == nil {
				var in io.ReadCloser
				in, err = item.Open()
				if err == nil {
					var out *os.File
					out, err = os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
					if err == nil {
						var count int64
						count, err = io.Copy(out, io.LimitReader(in, int64(item.UncompressedSize64)+1))
						closeErr := out.Close()
						if err == nil {
							err = closeErr
						}
						if count != int64(item.UncompressedSize64) {
							err = io.ErrUnexpectedEOF
						}
					}
					in.Close()
				}
			}
		}
		if err != nil {
			core.Bad(w, "could not extract archive; destination was not created", 400)
			return
		}
	}
	tx, err := a.DB.Begin()
	if err != nil {
		core.Bad(w, "could not save extracted files", 500)
		return
	}
	defer tx.Rollback()
	if err = a.RecordTree(tx, stage, destination, u.ID); err != nil {
		core.Bad(w, "could not record extracted files", 500)
		return
	}
	if err = os.Rename(stage, full); err != nil {
		core.Bad(w, "could not save extracted folder", 500)
		return
	}
	if err = tx.Commit(); err != nil {
		os.RemoveAll(full)
		core.Bad(w, "could not save extraction metadata", 500)
		return
	}
	a.Log(u.ID, "unzip", input.Path)
	core.JSON(w, map[string]string{"path": destination})
}

package files

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/rajibdpi/trynas/core"
)

const maxEditBytes = 2 << 20

func (a *Module) EditFile(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	p := r.URL.Query().Get("path")
	var input struct{ Path, Content, Version string }
	if r.Method == "POST" {
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 12<<20)).Decode(&input) != nil {
			core.Bad(w, "invalid editor request", 400)
			return
		}
		p = input.Path
	}
	p, err := core.CleanPath(p)
	if err != nil || p == "" {
		core.Bad(w, "invalid file path", 400)
		return
	}
	if !a.Can(u, p, false) || (r.Method == "POST" && !a.Can(u, p, true)) {
		core.Bad(w, "file permission denied", 403)
		return
	}
	full, err := core.Safe(a.Root, p)
	if err != nil {
		core.Bad(w, "invalid file path", 400)
		return
	}
	info, err := os.Stat(full)
	if err != nil || !info.Mode().IsRegular() {
		core.Bad(w, "select an existing regular file", 400)
		return
	}
	if info.Size() > maxEditBytes {
		core.Bad(w, "editor supports files up to 2 MiB", 413)
		return
	}
	data, err := os.ReadFile(full)
	if err != nil {
		core.Bad(w, "could not read file", 500)
		return
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		core.Bad(w, "editor supports UTF-8 text files only", 400)
		return
	}
	version := fmt.Sprintf("%x", sha256.Sum256(data))
	if r.Method == "GET" {
		core.JSON(w, map[string]any{"content": string(data), "version": version, "writable": a.Can(u, p, true)})
		return
	}
	if input.Version != version {
		core.Bad(w, "file changed since opening; close and reopen before editing", 409)
		return
	}
	next := []byte(input.Content)
	if len(next) > maxEditBytes || !utf8.Valid(next) || bytes.IndexByte(next, 0) >= 0 {
		core.Bad(w, "content must be UTF-8 text up to 2 MiB", 400)
		return
	}
	owner := u.ID
	tracked := a.DB.QueryRow("SELECT user_id FROM file_owners WHERE path=?", p).Scan(&owner) == nil
	extra := int64(len(next))
	if tracked {
		extra -= info.Size()
	}
	if err := a.CheckQuota(owner, extra); err != nil {
		core.Report(w, err)
		return
	}
	temp, err := os.CreateTemp(filepath.Dir(full), ".trynas-edit-*")
	if err != nil {
		core.Bad(w, "could not stage file", 500)
		return
	}
	defer os.Remove(temp.Name())
	if err = temp.Chmod(info.Mode().Perm()); err == nil {
		_, err = temp.Write(next)
	}
	if err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err != nil || closeErr != nil {
		core.Bad(w, "could not stage file", 500)
		return
	}
	tx, err := a.DB.Begin()
	if err != nil {
		core.Bad(w, "could not save file", 500)
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO file_owners(path,user_id,bytes) VALUES(?,?,?) ON CONFLICT(path) DO UPDATE SET bytes=excluded.bytes", p, owner, len(next)); err != nil {
		core.Bad(w, "could not save file", 500)
		return
	}
	if err = os.Rename(temp.Name(), full); err != nil {
		core.Bad(w, "could not replace file", 500)
		return
	}
	if err = tx.Commit(); err != nil {
		_ = os.WriteFile(full, data, info.Mode().Perm())
		core.Bad(w, "could not save file metadata", 500)
		return
	}
	a.Log(u.ID, "edit", p)
	core.JSON(w, map[string]string{"version": fmt.Sprintf("%x", sha256.Sum256(next))})
}

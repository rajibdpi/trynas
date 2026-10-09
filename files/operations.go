package files

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/rajibdpi/trynas/core"
)

func (a *Module) Destination(u core.User, p string) (string, error) {
	p, err := core.CleanPath(p)
	if err != nil || p == "" {
		return "", core.OperationError{"invalid destination", 400}
	}
	if !a.Can(u, p, true) || !a.Can(u, path.Dir(p), true) {
		return "", core.Forbidden("destination folder permission denied")
	}
	f, err := core.Safe(a.Root, p)
	if err != nil {
		return "", core.OperationError{err.Error(), 400}
	}
	if _, err = os.Lstat(f); err == nil {
		return "", core.Conflict("destination exists")
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	st, err := os.Stat(filepath.Dir(f))
	if err != nil || !st.IsDir() {
		return "", core.OperationError{"destination folder does not exist", 400}
	}
	return f, nil
}

func (a *Module) RecordTree(tx *sql.Tx, full, rel string, uid int) error {
	return filepath.Walk(full, func(f string, st os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if st.Mode().IsRegular() {
			suffix, _ := filepath.Rel(full, f)
			p := rel
			if suffix != "." {
				p = core.RelJoin(rel, suffix)
			}
			_, err = tx.Exec("INSERT INTO file_owners(path,user_id,bytes) VALUES(?,?,?)", p, uid, st.Size())
			return err
		}
		return nil
	})
}

func (a *Module) MoveOwned(from, to, src, dst string) error {
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = core.RemapOwners(tx, from, to); err != nil {
		return err
	}
	if err = os.Rename(src, dst); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		_ = os.Rename(dst, src)
		return err
	}
	return nil
}

func (a *Module) CopyOrMove(u core.User, from, to string, move bool) error {
	from, err := core.CleanPath(from)
	if err != nil || from == "" {
		return core.OperationError{"select a file or folder", 400}
	}
	to, err = core.CleanPath(to)
	if err != nil || core.Under(to, from) {
		return core.OperationError{"destination cannot be inside the source", 400}
	}
	if err = a.AllowTree(u, from, move); err != nil {
		return core.Forbidden(err.Error())
	}
	src, err := core.Safe(a.Root, from)
	if err != nil {
		return err
	}
	dst, err := a.Destination(u, to)
	if err != nil {
		return err
	}

	err = filepath.Walk(src, func(f string, st os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		suffix, _ := filepath.Rel(src, f)
		target := to
		if suffix != "." {
			target = core.RelJoin(to, suffix)
		}
		if !a.Can(u, target, true) {
			return core.Forbidden("nested destination folder permission denied")
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Path-based ACLs must not be orphaned or silently relocated by a rename.
	rows, err := a.DB.Query("SELECT path FROM permissions")
	if err != nil {
		return err
	}
	protected := false
	for rows.Next() {
		var p string
		_ = rows.Scan(&p)
		if core.Under(p, from) || core.Under(p, to) {
			protected = true
		}
	}
	rows.Close()
	if protected && move {
		return core.Conflict("remove explicit permissions inside the source or destination before moving this folder")
	}
	if move {
		err = a.MoveOwned(from, to, src, dst)
	} else {
		size, e := core.TreeBytes(src)
		if e != nil {
			return e
		}
		if e = a.CheckQuota(u.ID, size); e != nil {
			return core.OperationError{e.Error(), 413}
		}
		stage, e := os.MkdirTemp(filepath.Dir(dst), ".trynas-copy-")
		if e != nil {
			return e
		}
		defer os.RemoveAll(stage)
		staged := filepath.Join(stage, "content")
		if e = core.CopyTree(src, staged); e != nil {
			return e
		}
		tx, e := a.DB.Begin()
		if e != nil {
			return e
		}
		defer tx.Rollback()
		if e = a.RecordTree(tx, staged, to, u.ID); e != nil {
			return e
		}
		if e = os.Rename(staged, dst); e != nil {
			return e
		}
		if e = tx.Commit(); e != nil {
			_ = os.RemoveAll(dst)
			return e
		}
	}
	if err == nil {
		act := "copy"
		if move {
			act = "move"
		}
		a.Log(u.ID, act, from+" -> "+to)
	}
	return err
}

func (a *Module) CopyMoveV5(move bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var x struct{ From, To string }
		if !core.Decode(w, r, &x) {
			return
		}
		if err := a.CopyOrMove(r.Context().Value(core.UserKey).(core.User), x.From, x.To, move); err != nil {
			core.Report(w, err)
			return
		}
		core.JSON(w, map[string]bool{"ok": true})
	}
}

func (a *Module) RenameV5(w http.ResponseWriter, r *http.Request) { a.CopyMoveV5(true)(w, r) }

func (a *Module) UploadV5(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	r.Body = http.MaxBytesReader(w, r.Body, 10<<30)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		core.Bad(w, "invalid upload or upload exceeds 10 GiB", 413)
		return
	}
	defer r.MultipartForm.RemoveAll()
	dir, err := core.CleanPath(r.FormValue("path"))
	if err != nil {
		core.Bad(w, "invalid path", 400)
		return
	}
	if !a.Can(u, dir, true) {
		core.Bad(w, "folder permission denied", 403)
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		files = r.MultipartForm.File["file"]
	}
	if len(files) == 0 {
		core.Bad(w, "no files", 400)
		return
	}
	stage, err := os.MkdirTemp(a.Root, ".trynas-upload-")
	if err != nil {
		core.Report(w, err)
		return
	}
	defer os.RemoveAll(stage)
	type upload struct {
		staged, dest, rel string
		size              int64
	}
	pending := []upload{}
	names := map[string]bool{}
	var total int64
	for i, h := range files {
		name := filepath.Base(h.Filename)
		rel := core.RelJoin(dir, name)
		if names[rel] {
			core.Bad(w, "duplicate upload filename", 409)
			return
		}
		names[rel] = true
		dest, e := a.Destination(u, rel)
		if e != nil {
			core.Report(w, e)
			return
		}
		f, e := h.Open()
		if e != nil {
			core.Report(w, e)
			return
		}
		tmp := filepath.Join(stage, fmt.Sprintf("%d", i))
		out, e := os.Create(tmp)
		if e != nil {
			f.Close()
			core.Report(w, e)
			return
		}
		n, e := io.Copy(out, f)
		closeErr := out.Close()
		f.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			core.Report(w, e)
			return
		}
		total += n
		pending = append(pending, upload{tmp, dest, rel, n})
	}
	if err = a.CheckQuota(u.ID, total); err != nil {
		core.Bad(w, err.Error(), 413)
		return
	}
	tx, err := a.DB.Begin()
	if err != nil {
		core.Report(w, err)
		return
	}
	defer tx.Rollback()
	installed := []string{}
	rollback := func() {
		for _, f := range installed {
			_ = os.Remove(f)
		}
	}
	for _, x := range pending {
		if _, err = tx.Exec("INSERT INTO file_owners(path,user_id,bytes) VALUES(?,?,?)", x.rel, u.ID, x.size); err != nil {
			rollback()
			core.Report(w, err)
			return
		}
		if err = os.Rename(x.staged, x.dest); err != nil {
			rollback()
			core.Report(w, err)
			return
		}
		installed = append(installed, x.dest)
	}
	if err = tx.Commit(); err != nil {
		rollback()
		core.Report(w, err)
		return
	}
	for _, x := range pending {
		a.Log(u.ID, "upload", x.rel)
	}
	core.JSON(w, map[string]any{"ok": true, "files": len(pending), "bytes": total})
}

func (a *Module) ZipOne(u core.User, p string) error {
	p, err := core.CleanPath(p)
	if err != nil || p == "" {
		return core.OperationError{"select a file or folder", 400}
	}
	if err = a.AllowTree(u, p, false); err != nil {
		return core.Forbidden(err.Error())
	}
	src, err := core.Safe(a.Root, p)
	if err != nil {
		return err
	}
	dst, err := a.Destination(u, p+".zip")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(dst), ".trynas-zip-")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	zw := zip.NewWriter(temp)
	err = filepath.Walk(src, func(f string, st os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if st.IsDir() {
			return nil
		}
		h, e := zip.FileInfoHeader(st)
		if e != nil {
			return e
		}
		h.Name, _ = filepath.Rel(filepath.Dir(src), f)
		h.Name = filepath.ToSlash(h.Name)
		h.Method = zip.Deflate
		out, e := zw.CreateHeader(h)
		if e != nil {
			return e
		}
		in, e := os.Open(f)
		if e != nil {
			return e
		}
		defer in.Close()
		_, e = io.Copy(out, in)
		return e
	})
	closeErr := zw.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	st, err := temp.Stat()
	if err != nil {
		return err
	}
	if err = a.CheckQuota(u.ID, st.Size()); err != nil {
		return core.OperationError{err.Error(), 413}
	}
	if err = temp.Close(); err != nil {
		return err
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO file_owners(path,user_id,bytes) VALUES(?,?,?)", p+".zip", u.ID, st.Size()); err != nil {
		return err
	}
	if err = os.Rename(temp.Name(), dst); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	a.Log(u.ID, "zip", p)
	return nil
}

func (a *Module) ZipV5(w http.ResponseWriter, r *http.Request) {
	var x struct{ Path string }
	if !core.Decode(w, r, &x) {
		return
	}
	if err := a.ZipOne(r.Context().Value(core.UserKey).(core.User), x.Path); err != nil {
		core.Report(w, err)
		return
	}
	core.JSON(w, map[string]bool{"ok": true})
}

func (a *Module) Bulk(w http.ResponseWriter, r *http.Request) {
	var x struct {
		Action, Destination string
		Paths               []string
	}
	if !core.Decode(w, r, &x) {
		return
	}
	if len(x.Paths) == 0 || len(x.Paths) > 100 {
		core.Bad(w, "select 1–100 items", 400)
		return
	}
	if x.Action != "trash" && x.Action != "copy" && x.Action != "move" && x.Action != "zip" && x.Action != "favorite" {
		core.Bad(w, "invalid bulk action", 400)
		return
	}
	u := r.Context().Value(core.UserKey).(core.User)
	results := []map[string]any{}
	seen := map[string]bool{}
	for _, p := range x.Paths {
		p, err := core.CleanPath(p)
		if err == nil && seen[p] {
			continue
		}
		seen[p] = true
		if err == nil {
			switch x.Action {
			case "trash":
				err = a.TrashOne(u, p)
			case "copy", "move":
				err = a.CopyOrMove(u, p, core.RelJoin(x.Destination, path.Base(p)), x.Action == "move")
			case "zip":
				err = a.ZipOne(u, p)
			case "favorite":
				if !a.Can(u, p, false) {
					err = core.Forbidden("folder permission denied")
				} else {
					_, err = a.DB.Exec("INSERT OR IGNORE INTO favorites(user_id,path) VALUES(?,?)", u.ID, p)
				}
			}
		}
		result := map[string]any{"path": p, "ok": err == nil}
		if err != nil {
			result["error"] = err.Error()
		}
		results = append(results, result)
	}
	core.JSON(w, map[string]any{"results": results})
}

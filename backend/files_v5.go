package main

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type operationError struct {
	message string
	status  int
}

func (e operationError) Error() string { return e.message }
func conflict(s string) error          { return operationError{s, 409} }
func forbidden(s string) error         { return operationError{s, 403} }
func report(w http.ResponseWriter, err error) {
	status := 500
	if e, ok := err.(operationError); ok {
		status = e.status
	}
	bad(w, err.Error(), status)
}
func (a *App) destination(u User, p string) (string, error) {
	p, err := cleanPath(p)
	if err != nil || p == "" {
		return "", operationError{"invalid destination", 400}
	}
	if !a.can(u, p, true) || !a.can(u, path.Dir(p), true) {
		return "", forbidden("destination folder permission denied")
	}
	f, err := safe(a.root, p)
	if err != nil {
		return "", operationError{err.Error(), 400}
	}
	if _, err = os.Lstat(f); err == nil {
		return "", conflict("destination exists")
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	st, err := os.Stat(filepath.Dir(f))
	if err != nil || !st.IsDir() {
		return "", operationError{"destination folder does not exist", 400}
	}
	return f, nil
}
func treeBytes(f string) (int64, error) {
	var n int64
	err := filepath.Walk(f, func(p string, st os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not supported")
		}
		if st.Mode().IsRegular() {
			n += st.Size()
		}
		return nil
	})
	return n, err
}
func (a *App) recordTree(tx *sql.Tx, full, rel string, uid int) error {
	return filepath.Walk(full, func(f string, st os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if st.Mode().IsRegular() {
			suffix, _ := filepath.Rel(full, f)
			p := rel
			if suffix != "." {
				p = relJoin(rel, suffix)
			}
			_, err = tx.Exec("INSERT INTO file_owners(path,user_id,bytes) VALUES(?,?,?)", p, uid, st.Size())
			return err
		}
		return nil
	})
}
func remapOwners(tx *sql.Tx, from, to string) error {
	rows, err := tx.Query("SELECT path,user_id,bytes FROM file_owners")
	if err != nil {
		return err
	}
	type owned struct {
		p    string
		uid  int
		size int64
	}
	items := []owned{}
	for rows.Next() {
		var x owned
		if err = rows.Scan(&x.p, &x.uid, &x.size); err != nil {
			rows.Close()
			return err
		}
		if under(x.p, from) {
			items = append(items, x)
		}
	}
	rows.Close()
	for _, x := range items {
		next := to + strings.TrimPrefix(x.p, from)
		if _, err = tx.Exec("UPDATE file_owners SET path=? WHERE path=?", next, x.p); err != nil {
			return err
		}
	}
	return nil
}
func (a *App) moveOwned(from, to, src, dst string) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = remapOwners(tx, from, to); err != nil {
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
func (a *App) copyOrMove(u User, from, to string, move bool) error {
	from, err := cleanPath(from)
	if err != nil || from == "" {
		return operationError{"select a file or folder", 400}
	}
	to, err = cleanPath(to)
	if err != nil || under(to, from) {
		return operationError{"destination cannot be inside the source", 400}
	}
	if err = a.allowTree(u, from, move); err != nil {
		return forbidden(err.Error())
	}
	src, err := safe(a.root, from)
	if err != nil {
		return err
	}
	dst, err := a.destination(u, to)
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
			target = relJoin(to, suffix)
		}
		if !a.can(u, target, true) {
			return forbidden("nested destination folder permission denied")
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Path-based ACLs must not be orphaned or silently relocated by a rename.
	rows, err := a.db.Query("SELECT path FROM permissions")
	if err != nil {
		return err
	}
	protected := false
	for rows.Next() {
		var p string
		_ = rows.Scan(&p)
		if under(p, from) || under(p, to) {
			protected = true
		}
	}
	rows.Close()
	if protected && move {
		return conflict("remove explicit permissions inside the source or destination before moving this folder")
	}
	if move {
		err = a.moveOwned(from, to, src, dst)
	} else {
		size, e := treeBytes(src)
		if e != nil {
			return e
		}
		if e = a.checkQuota(u.ID, size); e != nil {
			return operationError{e.Error(), 413}
		}
		stage, e := os.MkdirTemp(filepath.Dir(dst), ".trynas-copy-")
		if e != nil {
			return e
		}
		defer os.RemoveAll(stage)
		staged := filepath.Join(stage, "content")
		if e = copyTree(src, staged); e != nil {
			return e
		}
		tx, e := a.db.Begin()
		if e != nil {
			return e
		}
		defer tx.Rollback()
		if e = a.recordTree(tx, staged, to, u.ID); e != nil {
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
		a.log(u.ID, act, from+" -> "+to)
	}
	return err
}
func (a *App) copyMoveV5(move bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var x struct{ From, To string }
		if !decode(w, r, &x) {
			return
		}
		if err := a.copyOrMove(r.Context().Value(userKey).(User), x.From, x.To, move); err != nil {
			report(w, err)
			return
		}
		jsonOut(w, map[string]bool{"ok": true})
	}
}
func (a *App) renameV5(w http.ResponseWriter, r *http.Request) { a.copyMoveV5(true)(w, r) }
func (a *App) uploadV5(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	r.Body = http.MaxBytesReader(w, r.Body, 10<<30)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		bad(w, "invalid upload or upload exceeds 10 GiB", 413)
		return
	}
	defer r.MultipartForm.RemoveAll()
	dir, err := cleanPath(r.FormValue("path"))
	if err != nil {
		bad(w, "invalid path", 400)
		return
	}
	if !a.can(u, dir, true) {
		bad(w, "folder permission denied", 403)
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		files = r.MultipartForm.File["file"]
	}
	if len(files) == 0 {
		bad(w, "no files", 400)
		return
	}
	stage, err := os.MkdirTemp(a.root, ".trynas-upload-")
	if err != nil {
		report(w, err)
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
		rel := relJoin(dir, name)
		if names[rel] {
			bad(w, "duplicate upload filename", 409)
			return
		}
		names[rel] = true
		dest, e := a.destination(u, rel)
		if e != nil {
			report(w, e)
			return
		}
		f, e := h.Open()
		if e != nil {
			report(w, e)
			return
		}
		tmp := filepath.Join(stage, fmt.Sprintf("%d", i))
		out, e := os.Create(tmp)
		if e != nil {
			f.Close()
			report(w, e)
			return
		}
		n, e := io.Copy(out, f)
		closeErr := out.Close()
		f.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			report(w, e)
			return
		}
		total += n
		pending = append(pending, upload{tmp, dest, rel, n})
	}
	if err = a.checkQuota(u.ID, total); err != nil {
		bad(w, err.Error(), 413)
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		report(w, err)
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
			report(w, err)
			return
		}
		if err = os.Rename(x.staged, x.dest); err != nil {
			rollback()
			report(w, err)
			return
		}
		installed = append(installed, x.dest)
	}
	if err = tx.Commit(); err != nil {
		rollback()
		report(w, err)
		return
	}
	for _, x := range pending {
		a.log(u.ID, "upload", x.rel)
	}
	jsonOut(w, map[string]any{"ok": true, "files": len(pending), "bytes": total})
}
func (a *App) trashOne(u User, p string) error {
	p, err := cleanPath(p)
	if err != nil || p == "" {
		return operationError{"select a file or folder", 400}
	}
	if err = a.allowTree(u, p, true); err != nil {
		return forbidden(err.Error())
	}
	// Explicit permission entries must be removed before a protected folder can be deleted.
	rows, err := a.db.Query("SELECT path FROM permissions")
	if err != nil {
		return err
	}
	protected := false
	for rows.Next() {
		var base string
		_ = rows.Scan(&base)
		if under(base, p) {
			protected = true
		}
	}
	rows.Close()
	if protected {
		return conflict("remove permissions inside this folder before deleting it")
	}
	src, err := safe(a.root, p)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("%d-%s", time.Now().UnixNano(), token())
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = remapOwners(tx, p, ".trynas-trash/"+name); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO trash(user_id,original_path,trash_name) VALUES(?,?,?)", u.ID, p, name); err != nil {
		return err
	}
	dst := filepath.Join(a.trash, name)
	if err = os.Rename(src, dst); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		_ = os.Rename(dst, src)
		return err
	}
	a.log(u.ID, "trash", p)
	return nil
}
func (a *App) trashV5(w http.ResponseWriter, r *http.Request) {
	if err := a.trashOne(r.Context().Value(userKey).(User), r.URL.Query().Get("path")); err != nil {
		report(w, err)
		return
	}
	jsonOut(w, map[string]bool{"ok": true})
}
func (a *App) restoreV5(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	var x struct{ ID int }
	if !decode(w, r, &x) {
		return
	}
	var p, name string
	if a.db.QueryRow("SELECT original_path,trash_name FROM trash WHERE id=? AND (user_id=? OR ?='admin')", x.ID, u.ID, u.Role).Scan(&p, &name) != nil {
		bad(w, "trash entry not found", 404)
		return
	}
	dst, err := a.destination(u, p)
	if err != nil {
		report(w, err)
		return
	}
	src := filepath.Join(a.trash, name)
	err = filepath.Walk(src, func(f string, st os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		suffix, _ := filepath.Rel(src, f)
		target := p
		if suffix != "." {
			target = relJoin(p, suffix)
		}
		if !a.can(u, target, true) {
			return forbidden("restore folder permission denied")
		}
		return nil
	})
	if err != nil {
		report(w, err)
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		report(w, err)
		return
	}
	defer tx.Rollback()
	if err = remapOwners(tx, ".trynas-trash/"+name, p); err == nil {
		_, err = tx.Exec("DELETE FROM trash WHERE id=?", x.ID)
	}
	if err != nil {
		report(w, err)
		return
	}
	if err = os.Rename(src, dst); err != nil {
		report(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		_ = os.Rename(dst, src)
		report(w, err)
		return
	}
	a.log(u.ID, "restore", p)
	jsonOut(w, map[string]bool{"ok": true})
}
func (a *App) purgeV5(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	var name string
	id := r.URL.Query().Get("id")
	if a.db.QueryRow("SELECT trash_name FROM trash WHERE id=? AND (user_id=? OR ?='admin')", id, u.ID, u.Role).Scan(&name) != nil {
		bad(w, "trash entry not found", 404)
		return
	}
	prefix := ".trynas-trash/" + name
	tx, err := a.db.Begin()
	if err != nil {
		report(w, err)
		return
	}
	defer tx.Rollback()
	// Use a bounded prefix comparison rather than SQL LIKE (filenames may contain % or _).
	rows, err := tx.Query("SELECT path FROM file_owners")
	if err != nil {
		report(w, err)
		return
	}
	paths := []string{}
	for rows.Next() {
		var p string
		_ = rows.Scan(&p)
		if under(p, prefix) {
			paths = append(paths, p)
		}
	}
	rows.Close()
	for _, p := range paths {
		if _, err = tx.Exec("DELETE FROM file_owners WHERE path=?", p); err != nil {
			report(w, err)
			return
		}
	}
	if _, err = tx.Exec("DELETE FROM trash WHERE id=?", id); err != nil {
		report(w, err)
		return
	}
	// Do not release quota until removal succeeds. If the database commit fails
	// after removal, the next purge can finish deleting the retained metadata.
	src := filepath.Join(a.trash, name)
	if err = os.RemoveAll(src); err != nil {
		report(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		report(w, err)
		return
	}
	a.log(u.ID, "purge-trash", id)
	jsonOut(w, map[string]bool{"ok": true})
}
func (a *App) zipOne(u User, p string) error {
	p, err := cleanPath(p)
	if err != nil || p == "" {
		return operationError{"select a file or folder", 400}
	}
	if err = a.allowTree(u, p, false); err != nil {
		return forbidden(err.Error())
	}
	src, err := safe(a.root, p)
	if err != nil {
		return err
	}
	dst, err := a.destination(u, p+".zip")
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
	if err = a.checkQuota(u.ID, st.Size()); err != nil {
		return operationError{err.Error(), 413}
	}
	if err = temp.Close(); err != nil {
		return err
	}
	tx, err := a.db.Begin()
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
	a.log(u.ID, "zip", p)
	return nil
}
func (a *App) zipV5(w http.ResponseWriter, r *http.Request) {
	var x struct{ Path string }
	if !decode(w, r, &x) {
		return
	}
	if err := a.zipOne(r.Context().Value(userKey).(User), x.Path); err != nil {
		report(w, err)
		return
	}
	jsonOut(w, map[string]bool{"ok": true})
}
func (a *App) bulk(w http.ResponseWriter, r *http.Request) {
	var x struct {
		Action, Destination string
		Paths               []string
	}
	if !decode(w, r, &x) {
		return
	}
	if len(x.Paths) == 0 || len(x.Paths) > 100 {
		bad(w, "select 1–100 items", 400)
		return
	}
	if x.Action != "trash" && x.Action != "copy" && x.Action != "move" && x.Action != "zip" && x.Action != "favorite" {
		bad(w, "invalid bulk action", 400)
		return
	}
	u := r.Context().Value(userKey).(User)
	results := []map[string]any{}
	seen := map[string]bool{}
	for _, p := range x.Paths {
		p, err := cleanPath(p)
		if err == nil && seen[p] {
			continue
		}
		seen[p] = true
		if err == nil {
			switch x.Action {
			case "trash":
				err = a.trashOne(u, p)
			case "copy", "move":
				err = a.copyOrMove(u, p, relJoin(x.Destination, path.Base(p)), x.Action == "move")
			case "zip":
				err = a.zipOne(u, p)
			case "favorite":
				if !a.can(u, p, false) {
					err = forbidden("folder permission denied")
				} else {
					_, err = a.db.Exec("INSERT OR IGNORE INTO favorites(user_id,path) VALUES(?,?)", u.ID, p)
				}
			}
		}
		result := map[string]any{"path": p, "ok": err == nil}
		if err != nil {
			result["error"] = err.Error()
		}
		results = append(results, result)
	}
	jsonOut(w, map[string]any{"results": results})
}

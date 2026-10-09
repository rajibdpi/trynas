package trash

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rajibdpi/trynas/core"
)

func (a *Module) TrashOne(u core.User, p string) error {
	p, err := core.CleanPath(p)
	if err != nil || p == "" {
		return core.OperationError{Message: "select a file or folder", Status: 400}
	}
	if err = a.AllowTree(u, p, true); err != nil {
		return core.Forbidden(err.Error())
	}
	// Explicit permission entries must be removed before a protected folder can be deleted.
	rows, err := a.DB.Query("SELECT path FROM permissions")
	if err != nil {
		return err
	}
	protected := false
	for rows.Next() {
		var base string
		_ = rows.Scan(&base)
		if core.Under(base, p) {
			protected = true
		}
	}
	rows.Close()
	if protected {
		return core.Conflict("remove permissions inside this folder before deleting it")
	}
	src, err := core.Safe(a.Root, p)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("%d-%s", time.Now().UnixNano(), core.Token())
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = core.RemapOwners(tx, p, ".trynas-trash/"+name); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO trash(user_id,original_path,trash_name) VALUES(?,?,?)", u.ID, p, name); err != nil {
		return err
	}
	dst := filepath.Join(a.Trash, name)
	if err = os.Rename(src, dst); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		_ = os.Rename(dst, src)
		return err
	}
	a.Log(u.ID, "trash", p)
	return nil
}

func (a *Module) TrashV5(w http.ResponseWriter, r *http.Request) {
	if err := a.TrashOne(r.Context().Value(core.UserKey).(core.User), r.URL.Query().Get("path")); err != nil {
		core.Report(w, err)
		return
	}
	core.JSON(w, map[string]bool{"ok": true})
}

func (a *Module) RestoreV5(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	var x struct{ ID int }
	if !core.Decode(w, r, &x) {
		return
	}
	var p, name string
	if a.DB.QueryRow("SELECT original_path,trash_name FROM trash WHERE id=? AND (user_id=? OR ?='admin')", x.ID, u.ID, u.Role).Scan(&p, &name) != nil {
		core.Bad(w, "trash entry not found", 404)
		return
	}
	dst, err := a.Destination(u, p)
	if err != nil {
		core.Report(w, err)
		return
	}
	src := filepath.Join(a.Trash, name)
	err = filepath.Walk(src, func(f string, st os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		suffix, _ := filepath.Rel(src, f)
		target := p
		if suffix != "." {
			target = core.RelJoin(p, suffix)
		}
		if !a.Can(u, target, true) {
			return core.Forbidden("restore folder permission denied")
		}
		return nil
	})
	if err != nil {
		core.Report(w, err)
		return
	}
	tx, err := a.DB.Begin()
	if err != nil {
		core.Report(w, err)
		return
	}
	defer tx.Rollback()
	if err = core.RemapOwners(tx, ".trynas-trash/"+name, p); err == nil {
		_, err = tx.Exec("DELETE FROM trash WHERE id=?", x.ID)
	}
	if err != nil {
		core.Report(w, err)
		return
	}
	if err = os.Rename(src, dst); err != nil {
		core.Report(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		_ = os.Rename(dst, src)
		core.Report(w, err)
		return
	}
	a.Log(u.ID, "restore", p)
	core.JSON(w, map[string]bool{"ok": true})
}

func (a *Module) PurgeV5(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	var name string
	id := r.URL.Query().Get("id")
	if a.DB.QueryRow("SELECT trash_name FROM trash WHERE id=? AND (user_id=? OR ?='admin')", id, u.ID, u.Role).Scan(&name) != nil {
		core.Bad(w, "trash entry not found", 404)
		return
	}
	prefix := ".trynas-trash/" + name
	tx, err := a.DB.Begin()
	if err != nil {
		core.Report(w, err)
		return
	}
	defer tx.Rollback()
	// Use a bounded prefix comparison rather than SQL LIKE (filenames may contain % or _).
	rows, err := tx.Query("SELECT path FROM file_owners")
	if err != nil {
		core.Report(w, err)
		return
	}
	paths := []string{}
	for rows.Next() {
		var p string
		_ = rows.Scan(&p)
		if core.Under(p, prefix) {
			paths = append(paths, p)
		}
	}
	rows.Close()
	for _, p := range paths {
		if _, err = tx.Exec("DELETE FROM file_owners WHERE path=?", p); err != nil {
			core.Report(w, err)
			return
		}
	}
	if _, err = tx.Exec("DELETE FROM trash WHERE id=?", id); err != nil {
		core.Report(w, err)
		return
	}
	// Do not release quota until removal succeeds. If the database commit fails
	// after removal, the next purge can finish deleting the retained metadata.
	src := filepath.Join(a.Trash, name)
	if err = os.RemoveAll(src); err != nil {
		core.Report(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		core.Report(w, err)
		return
	}
	a.Log(u.ID, "purge-trash", id)
	core.JSON(w, map[string]bool{"ok": true})
}

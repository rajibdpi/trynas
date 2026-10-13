package files

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
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
		return "", core.OperationError{Message: "invalid destination", Status: 400}
	}
	if !a.Can(u, p, true) || !a.Can(u, path.Dir(p), true) {
		return "", core.Forbidden("destination folder permission denied")
	}
	f, err := core.Safe(a.Root, p)
	if err != nil {
		return "", core.OperationError{Message: err.Error(), Status: 400}
	}
	if _, err = os.Lstat(f); err == nil {
		return "", core.Conflict("destination exists")
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	st, err := os.Stat(filepath.Dir(f))
	if err != nil || !st.IsDir() {
		return "", core.OperationError{Message: "destination folder does not exist", Status: 400}
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
		return core.OperationError{Message: "select a file or folder", Status: 400}
	}
	to, err = core.CleanPath(to)
	if err != nil || core.Under(to, from) {
		return core.OperationError{Message: "destination cannot be inside the source", Status: 400}
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
			return core.OperationError{Message: e.Error(), Status: 413}
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
		a.Log(u.ID, act, from+" -> "+to, u.IP)
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
	// Reserve bounded staging capacity without holding the application mutation lock.
	a.Uploads.Lock()
	if a.ActiveUploads >= 2 {
		a.Uploads.Unlock()
		core.Bad(w, "upload capacity busy; retry later", 429)
		return
	}
	a.ActiveUploads++
	a.Uploads.Unlock()
	defer func() {
		a.Uploads.Lock()
		a.ActiveUploads--
		a.Uploads.Unlock()
	}()
	r.Body = http.MaxBytesReader(w, r.Body, 10<<30)
	reader, err := r.MultipartReader()
	if err != nil {
		core.Bad(w, "invalid multipart upload", 400)
		return
	}
	var used, quota int64
	if err = a.DB.QueryRow(`SELECT COALESCE((SELECT SUM(bytes) FROM file_owners WHERE user_id=?),0), COALESCE((SELECT quota_bytes FROM user_limits WHERE user_id=?),0)`, u.ID, u.ID).Scan(&used, &quota); err != nil {
		core.Bad(w, "quota unavailable", 500)
		return
	}
	remaining := int64(10 << 30)
	if quota > 0 {
		remaining = min(remaining, max(int64(0), quota-used))
	}
	policies := map[string]string{}
	policiesSeen := false
	credited := map[string]bool{}
	dir := ""
	pathSeen := false
	stage := ""
	defer func() {
		if stage != "" {
			_ = os.RemoveAll(stage)
		}
	}()
	type upload struct {
		staged, dest, rel string
		replace           bool
		size              int64
	}
	pending := []upload{}
	names := map[string]bool{}
	var total int64
	for {
		part, e := reader.NextPart()
		if e == io.EOF {
			break
		}
		if e != nil {
			core.Bad(w, "invalid upload or upload exceeds limit", 413)
			return
		}
		if part.FormName() == "conflicts" && part.FileName() == "" {
			data, e := io.ReadAll(io.LimitReader(part, (1<<20)+1))
			if e != nil || len(data) > 1<<20 || policiesSeen || len(pending) > 0 || json.Unmarshal(data, &policies) != nil {
				core.Bad(w, "invalid upload conflict choices", 400)
				return
			}
			for name, policy := range policies {
				clean, e := core.CleanPath(name)
				if e != nil || clean != name || clean == "" || path.Base(name) != name || (policy != "keep" && policy != "replace") {
					core.Bad(w, "invalid upload conflict choices", 400)
					return
				}
			}
			policiesSeen = true
			continue
		}
		if part.FormName() == "path" && part.FileName() == "" {
			data, e := io.ReadAll(io.LimitReader(part, 4097))
			if e != nil || len(data) > 4096 || pathSeen {
				core.Bad(w, "invalid upload destination", 400)
				return
			}
			dir, e = core.CleanPath(string(data))
			if e != nil {
				core.Bad(w, "invalid path", 400)
				return
			}
			if !a.Can(u, dir, true) {
				core.Bad(w, "folder permission denied", 403)
				return
			}
			pathSeen = true
			continue
		}
		if part.FileName() == "" || (part.FormName() != "files" && part.FormName() != "file") {
			core.Bad(w, "unexpected upload field", 400)
			return
		}
		if len(pending) >= 10000 {
			core.Bad(w, "upload exceeds 10,000 files", 413)
			return
		}
		name := filepath.Base(part.FileName())
		rel, e := core.CleanPath(name)
		if e != nil || rel == "" {
			core.Bad(w, "invalid filename", 400)
			return
		}
		if names[rel] {
			core.Bad(w, "duplicate upload filename", 409)
			return
		}
		names[rel] = true
		dest := ""
		if pathSeen {
			var info os.FileInfo
			dest, info, e = a.uploadTarget(u, core.RelJoin(dir, rel))
			if e != nil {
				core.Report(w, e)
				return
			}
			// Replacements may reuse the uploader's already-accounted quota.
			if info != nil && policies[rel] == "replace" && !credited[rel] {
				var owned int64
				if a.DB.QueryRow("SELECT bytes FROM file_owners WHERE path=? AND user_id=?", core.RelJoin(dir, rel), u.ID).Scan(&owned) == nil {
					remaining = min(int64(10<<30)-total, remaining+owned)
				}
				credited[rel] = true
			}
		}
		if stage == "" {
			stage, e = os.MkdirTemp(a.Root, ".trynas-upload-")
			if e != nil {
				core.Report(w, e)
				return
			}
		}
		tmp := filepath.Join(stage, fmt.Sprintf("%d", len(pending)))
		out, e := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			core.Report(w, e)
			return
		}
		n, e := io.Copy(out, io.LimitReader(part, remaining+1))
		closeErr := out.Close()
		if n > remaining {
			core.Bad(w, "storage quota or upload limit exceeded", 413)
			return
		}
		if e == nil {
			e = closeErr
		}
		if e != nil {
			core.Bad(w, "upload interrupted or exceeds limit", 413)
			return
		}
		remaining -= n
		total += n
		pending = append(pending, upload{staged: tmp, dest: dest, rel: rel, size: n})
	}
	if len(pending) == 0 {
		core.Bad(w, "no files", 400)
		return
	}
	// Revalidate under the mutation lock: permissions, quota and destinations may
	// have changed while bytes were streamed to private staging files.
	a.Mutation.Lock()
	defer a.Mutation.Unlock()
	reserved := map[string]bool{}
	// Reserve incoming original names, so generated copies cannot steal a later
	// incoming file's name within the same request.
	for _, x := range pending {
		reserved[core.RelJoin(dir, x.rel)] = true
	}
	var quotaExtra int64 = total
	for i := range pending {
		x := &pending[i]
		name := x.rel
		x.rel = core.RelJoin(dir, name)
		var info os.FileInfo
		x.dest, info, err = a.uploadTarget(u, x.rel)
		if err != nil {
			core.Report(w, err)
			return
		}
		if info != nil {
			if info.Size() != x.size || policies[name] == "keep" {
				x.rel, x.dest, err = a.numberedUploadTarget(u, x.rel, reserved)
				if err != nil {
					core.Report(w, err)
					return
				}
				reserved[x.rel] = true
			} else if policies[name] == "replace" {
				x.replace = true
				var owned int64
				if a.DB.QueryRow("SELECT bytes FROM file_owners WHERE path=? AND user_id=?", x.rel, u.ID).Scan(&owned) == nil {
					quotaExtra -= owned
				}
			} else {
				core.Bad(w, "same-name, same-size file exists; choose keep both or replace", 409)
				return
			}
		}
	}
	if err = a.CheckQuota(u.ID, quotaExtra); err != nil {
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
	backups := map[string]string{}
	rollback := func() {
		for _, f := range installed {
			_ = os.Remove(f)
		}
		for dest, backup := range backups {
			_ = os.Rename(backup, dest)
		}
	}
	for i, x := range pending {
		if x.replace {
			backup := filepath.Join(stage, fmt.Sprintf("backup-%d", i))
			if err = os.Rename(x.dest, backup); err != nil {
				rollback()
				core.Report(w, err)
				return
			}
			backups[x.dest] = backup
		}
		if _, err = tx.Exec("INSERT INTO file_owners(path,user_id,bytes) VALUES(?,?,?) ON CONFLICT(path) DO UPDATE SET user_id=excluded.user_id,bytes=excluded.bytes", x.rel, u.ID, x.size); err != nil {
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
		a.Log(u.ID, "upload", x.rel, u.IP)
	}
	core.JSON(w, map[string]any{"ok": true, "files": len(pending), "bytes": total})
}

func (a *Module) ZipOne(u core.User, p string) error {
	p, err := core.CleanPath(p)
	if err != nil || p == "" {
		return core.OperationError{Message: "select a file or folder", Status: 400}
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
		return core.OperationError{Message: err.Error(), Status: 413}
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
	a.Log(u.ID, "zip", p, u.IP)
	return nil
}

func (a *Module) ZipV5(w http.ResponseWriter, r *http.Request) {
	var x struct {
		Path        string
		Paths       []string
		Destination string
	}
	if !core.Decode(w, r, &x) {
		return
	}
	var err error
	if len(x.Paths) > 0 {
		err = a.ZipSelection(r.Context().Value(core.UserKey).(core.User), x.Paths, x.Destination)
	} else {
		err = a.ZipOne(r.Context().Value(core.UserKey).(core.User), x.Path)
	}
	if err != nil {
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

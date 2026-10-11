package providers

import (
	"context"
	"fmt"
	"net/http"
	"path"

	"github.com/rajibdpi/trynas/core"
	"github.com/rclone/rclone/fs"
	"os"
)

func (a *Module) trashRemote(ctx context.Context, u core.User, f fs.Fs, storage, source, stage string, entries []stagedEntry) error {
	if f == nil {
		return fmt.Errorf("use local trash for local files")
	}
	target := path.Join(".trynas-trash", core.Token(), path.Base(source))
	if err := publishRemote(ctx, f, target, stage, entries); err != nil {
		return err
	}
	// Metadata is saved before deleting the original, so a failure leaves a recoverable copy.
	if _, err := a.DB.Exec("INSERT INTO remote_trash(user_id,storage,original_path,trash_path) VALUES(?,?,?,?)", u.ID, storage, source, target); err != nil {
		_ = removeRemote(ctx, f, target, entries[0].dir)
		return err
	}
	if err := verifySource(ctx, f, source, entries); err != nil {
		return err
	}
	return removeRemote(ctx, f, source, entries[0].dir)
}

func (a *Module) RemoteTrash(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	if u.Role != "admin" {
		core.Bad(w, "administrator required", 403)
		return
	}
	id := r.URL.Query().Get("storage")
	a.Providers.Mu.Lock()
	connections, err := a.Connections()
	a.Providers.Mu.Unlock()
	found := false
	for _, c := range connections {
		if c.ID == id && c.Attached {
			found = true
		}
	}
	if err != nil || !found {
		core.Bad(w, "attached storage not found", 404)
		return
	}
	rows, err := a.DB.Query("SELECT id,original_path,deleted_at FROM remote_trash WHERE storage=? ORDER BY id DESC", id)
	if err != nil {
		core.Bad(w, "could not load storage trash", 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var n int64
		var p, at string
		if err = rows.Scan(&n, &p, &at); err != nil {
			core.Bad(w, "could not load storage trash", 500)
			return
		}
		out = append(out, map[string]any{"id": n, "path": p, "name": path.Base(p), "deletedAt": at})
	}
	w.Header().Set("Cache-Control", "no-store")
	core.JSON(w, out)
}

func (a *Module) restoreRemoteTrash(ctx context.Context, u core.User, f fs.Fs, storage string, id int64, purge, confirm bool) error {
	if f == nil {
		return fmt.Errorf("select an attached storage")
	}
	var original, trashed string
	if err := a.DB.QueryRow("SELECT original_path,trash_path FROM remote_trash WHERE id=? AND storage=?", id, storage).Scan(&original, &trashed); err != nil {
		return fmt.Errorf("trash item not found")
	}
	if purge {
		if !confirm {
			return fmt.Errorf("confirm permanent deletion")
		}
		dir := true
		if _, e := f.NewObject(ctx, trashed); e == nil {
			dir = false
		}
		if err := removeRemote(ctx, f, trashed, dir); err != nil {
			return err
		}
		if _, err := a.DB.Exec("DELETE FROM remote_trash WHERE id=? AND storage=?", id, storage); err != nil {
			return err
		}
		_ = f.Rmdir(ctx, path.Dir(trashed))
		a.Log(u.ID, "remote-trash-purge", storage+":"+original, u.IP)
		return nil
	}
	stage, err := os.MkdirTemp("", "trynas-restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	entries, err := stageRemote(ctx, f, trashed, stage)
	if err != nil {
		return err
	}
	if _, err = core.CleanPath(original); err != nil {
		return err
	}
	if err = publishRemote(ctx, f, original, stage, entries); err != nil {
		return err
	}
	if err = verifySource(ctx, f, trashed, entries); err != nil {
		return err
	}
	if err = removeRemote(ctx, f, trashed, entries[0].dir); err != nil {
		return err
	}
	_ = f.Rmdir(ctx, path.Dir(trashed))
	if _, err = a.DB.Exec("DELETE FROM remote_trash WHERE id=? AND storage=?", id, storage); err != nil {
		return err
	}
	a.Log(u.ID, "remote-trash-restore", storage+":"+original, u.IP)
	return nil
}

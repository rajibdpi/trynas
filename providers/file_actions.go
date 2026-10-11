package providers

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/rajibdpi/trynas/core"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
)

type stagedEntry struct {
	rel      string
	dir      bool
	size     int64
	sum      string
	modified time.Time
}

// Stage a bounded, validated tree before publishing a copy or deleting a source.
func stageRemote(ctx context.Context, f fs.Fs, source, stage string) ([]stagedEntry, error) {
	var entries []stagedEntry
	var total int64
	var visit func(string, string, int) error
	visit = func(remote, rel string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth > 64 || len(entries) >= 100000 {
			return fmt.Errorf("select a smaller folder tree")
		}
		obj, err := f.NewObject(ctx, remote)
		if err == nil {
			if obj.Size() > 10<<30 {
				return fmt.Errorf("individual files must be at most 10 GiB")
			}
			reader, err := obj.Open(ctx)
			if err != nil {
				return err
			}
			defer reader.Close()
			full := filepath.Join(stage, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
				return err
			}
			file, err := os.OpenFile(full, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			h := sha256.New()
			n, copyErr := io.Copy(io.MultiWriter(file, h), io.LimitReader(reader, (10<<30)+1))
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			if n > 10<<30 || (obj.Size() >= 0 && n != obj.Size()) {
				return fmt.Errorf("source size changed during copy")
			}
			total += n
			if total > 10<<30 {
				return fmt.Errorf("select at most 10 GiB per item")
			}
			entries = append(entries, stagedEntry{rel: rel, size: n, sum: hex.EncodeToString(h.Sum(nil)), modified: obj.ModTime(ctx)})
			return nil
		}
		if !errors.Is(err, fs.ErrorObjectNotFound) && !errors.Is(err, fs.ErrorIsDir) && !errors.Is(err, fs.ErrorNotAFile) {
			return err
		}
		children, err := f.List(ctx, remote)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(stage, filepath.FromSlash(rel)), 0700); err != nil {
			return err
		}
		entries = append(entries, stagedEntry{rel: rel, dir: true})
		for _, child := range children {
			checked := child.Remote()
			if strings.HasPrefix(source, ".trynas-trash/") {
				checked = strings.TrimPrefix(checked, source+"/")
			}
			_, e := core.CleanPath(checked)
			clean := child.Remote()
			if e != nil || clean != child.Remote() || path.Dir(clean) != DirectoryParent(remote) {
				return fmt.Errorf("remote contains an unsafe or reserved path")
			}
			if err := visit(clean, path.Join(rel, path.Base(clean)), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	err := visit(source, "content", 0)
	return entries, err
}

func remoteExists(ctx context.Context, f fs.Fs, p string) (bool, error) {
	if _, err := f.NewObject(ctx, p); err == nil {
		return true, nil
	} else if !errors.Is(err, fs.ErrorObjectNotFound) && !errors.Is(err, fs.ErrorIsDir) && !errors.Is(err, fs.ErrorNotAFile) {
		return false, err
	}
	if _, err := f.List(ctx, p); err == nil {
		return true, nil
	} else if !errors.Is(err, fs.ErrorDirNotFound) && !errors.Is(err, fs.ErrorObjectNotFound) {
		return false, err
	}
	return false, nil
}

func entryTarget(target, rel string) string {
	if rel == "content" {
		return target
	}
	return path.Join(target, strings.TrimPrefix(rel, "content/"))
}

func verifyRemote(ctx context.Context, f fs.Fs, target string, entries []stagedEntry) error {
	for _, entry := range entries {
		if entry.dir {
			continue
		}
		obj, err := f.NewObject(ctx, entryTarget(target, entry.rel))
		if err != nil {
			return err
		}
		reader, err := obj.Open(ctx)
		if err != nil {
			return err
		}
		h := sha256.New()
		n, err := io.Copy(h, reader)
		closeErr := reader.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if n != entry.size || hex.EncodeToString(h.Sum(nil)) != entry.sum {
			return fmt.Errorf("verification failed; source preserved")
		}
	}
	return nil
}

func publishRemote(ctx context.Context, f fs.Fs, target, stage string, entries []stagedEntry) error {
	exists, err := remoteExists(ctx, f, target)
	if err != nil {
		return err
	}
	if exists {
		return core.Conflict("destination exists")
	}
	created := false
	complete := false
	defer func() {
		if created && !complete {
			cleanup, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			_ = removeRemote(cleanup, f, target, entries[0].dir)
		}
	}()
	for _, entry := range entries {
		p := entryTarget(target, entry.rel)
		if entry.dir {
			err = f.Mkdir(ctx, p)
			if err == nil {
				created = true
			}
		} else {
			file, e := os.Open(filepath.Join(stage, filepath.FromSlash(entry.rel)))
			if e != nil {
				return e
			}
			created = true
			_, err = f.Put(ctx, file, object.NewStaticObjectInfo(p, entry.modified, entry.size, true, nil, f))
			file.Close()
		}
		if err != nil {
			return err
		}
	}
	if err = verifyRemote(ctx, f, target, entries); err != nil {
		return err
	}
	complete = true
	return nil
}

func removeRemote(ctx context.Context, f fs.Fs, p string, dir bool) error {
	if dir {
		return operations.Purge(ctx, f, p)
	}
	obj, err := f.NewObject(ctx, p)
	if err != nil {
		return err
	}
	return obj.Remove(ctx)
}

// Remote operations use one lease for both endpoints to avoid overlapping writes.
func (a *Module) actionConnections(ctx context.Context, ids []string) (map[string]fs.Fs, func(), error) {
	a.Providers.Mu.Lock()
	if a.Providers.RemoteOps > 0 {
		a.Providers.Mu.Unlock()
		return nil, nil, core.Conflict("storage is busy; try again shortly")
	}
	connections, err := a.Connections()
	if err != nil {
		a.Providers.Mu.Unlock()
		return nil, nil, err
	}
	selected := map[string]core.StorageConnection{}
	for _, id := range ids {
		if id == "" {
			continue
		}
		for _, c := range connections {
			if c.ID == id && c.Attached {
				selected[id] = c
			}
		}
		if _, ok := selected[id]; !ok {
			a.Providers.Mu.Unlock()
			return nil, nil, core.OperationError{Message: "attached storage not found", Status: 404}
		}
	}
	a.Providers.RemoteOps++
	a.Providers.Mu.Unlock()
	refreshed := map[string]configmap.Simple{}
	finish := func() {
		a.Providers.Mu.Lock()
		defer a.Providers.Mu.Unlock()
		a.Providers.RemoteOps--
		current, e := a.Connections()
		if e != nil {
			return
		}
		changed := false
		for i := range current {
			if opts := refreshed[current[i].ID]; opts != nil && opts["token"] != "" {
				current[i].Options["token"] = opts["token"]
				changed = true
			}
		}
		if changed {
			_ = a.SaveConnections(current)
		}
	}
	result := map[string]fs.Fs{}
	for id, c := range selected {
		f, options, e := NewProviderFS(ctx, c)
		if e != nil {
			finish()
			return nil, nil, e
		}
		result[id] = f
		refreshed[id] = options
	}
	return result, finish, nil
}

func (a *Module) FileActions(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	if u.Role != "admin" {
		core.Bad(w, "administrator required", 403)
		return
	}
	var x struct {
		Action, Storage, DestinationStorage, Destination, Name string
		Paths                                                  []string
		TrashID                                                int64
		Confirm                                                bool
	}
	if !core.Decode(w, r, &x) {
		return
	}
	if x.Action != "copy" && x.Action != "move" && x.Action != "rename" && x.Action != "zip" && x.Action != "trash" && x.Action != "restore" && x.Action != "purge" {
		core.Bad(w, "unsupported file action", 400)
		return
	}
	if len(x.Paths) > 100 || (len(x.Paths) == 0 && x.Action != "restore" && x.Action != "purge") {
		core.Bad(w, "select 1–100 items", 400)
		return
	}
	for i, p := range x.Paths {
		clean, err := core.CleanPath(p)
		if err != nil || clean == "" {
			core.Bad(w, "select a non-root file or folder", 400)
			return
		}
		x.Paths[i] = clean
	}
	for i, p := range x.Paths {
		for j, q := range x.Paths {
			if i != j && core.Under(p, q) {
				core.Bad(w, "select each file or folder only once, without its parent", 400)
				return
			}
		}
	}
	destination, err := core.CleanPath(x.Destination)
	if err != nil {
		core.Bad(w, "invalid destination", 400)
		return
	}
	if x.Action == "rename" || x.Action == "zip" {
		clean, e := core.CleanPath(x.Name)
		if e != nil || clean == "" || strings.Contains(clean, "/") {
			core.Bad(w, "enter a single file or folder name", 400)
			return
		}
		if x.Action == "rename" && len(x.Paths) != 1 {
			core.Bad(w, "rename one item at a time", 400)
			return
		}
		x.DestinationStorage = x.Storage
		if x.Action == "rename" {
			destination = path.Dir(x.Paths[0])
			if destination == "." {
				destination = ""
			}
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	remotes, finish, err := a.actionConnections(ctx, []string{x.Storage, x.DestinationStorage})
	if err != nil {
		fileActionError(w, err)
		return
	}
	defer finish()
	defer a.invalidateListings(x.Storage)
	defer a.invalidateListings(x.DestinationStorage)
	if x.Action == "restore" || x.Action == "purge" {
		err = a.restoreRemoteTrash(ctx, u, remotes[x.Storage], x.Storage, x.TrashID, x.Action == "purge", x.Confirm)
		if err != nil {
			fileActionError(w, err)
			return
		}
		core.JSON(w, map[string]bool{"ok": true})
		return
	}
	if x.Storage == "" && x.DestinationStorage == "" {
		core.Bad(w, "use local file actions for local storage", 400)
		return
	}
	if x.Action == "copy" || x.Action == "move" || x.Action == "rename" {
		names := map[string]bool{}
		for _, source := range x.Paths {
			target := path.Join(destination, path.Base(source))
			if x.Action == "rename" {
				target = path.Join(destination, x.Name)
			}
			if names[target] {
				core.Bad(w, "selected items have conflicting destination names", 409)
				return
			}
			names[target] = true
			if x.Storage == x.DestinationStorage && core.Under(target, source) {
				core.Bad(w, "destination cannot be inside the source", 400)
				return
			}
			if x.Storage == "" {
				if err = a.AllowTree(u, source, x.Action == "move"); err != nil {
					fileActionError(w, err)
					return
				}
				if x.Action == "move" {
					if err = a.canRemoveLocal(source); err != nil {
						fileActionError(w, err)
						return
					}
				}
			}
			if x.DestinationStorage == "" {
				if _, err = a.Destination(u, target); err != nil {
					fileActionError(w, err)
					return
				}
			} else {
				exists, e := remoteExists(ctx, remotes[x.DestinationStorage], target)
				if e != nil {
					fileActionError(w, e)
					return
				}
				if exists {
					core.Bad(w, "destination exists: "+target, 409)
					return
				}
			}
		}
	}
	completed := []string{}
	for _, source := range x.Paths {
		target := path.Join(destination, path.Base(source))
		if x.Action == "rename" {
			target = path.Join(destination, x.Name)
		}
		if x.Action == "zip" {
			break
		}
		if x.Storage == x.DestinationStorage && (x.Action == "copy" || x.Action == "move" || x.Action == "rename") && core.Under(target, source) {
			core.Bad(w, "destination cannot be inside the source", 400)
			return
		}
		if x.Storage == "" {
			if err = a.AllowTree(u, source, x.Action == "move"); err != nil {
				fileActionError(w, err)
				return
			}
			if x.Action == "move" {
				if err = a.canRemoveLocal(source); err != nil {
					fileActionError(w, err)
					return
				}
			}
		}
		stage, e := os.MkdirTemp("", "trynas-transfer-")
		if e != nil {
			fileActionError(w, e)
			return
		}
		entries, e := a.stageSource(ctx, remotes[x.Storage], x.Storage, source, stage)
		if e == nil && x.Action == "trash" {
			e = a.trashRemote(ctx, u, remotes[x.Storage], x.Storage, source, stage, entries)
		} else if e == nil {
			if x.DestinationStorage == "" {
				e = a.publishLocal(ctx, u, target, stage, entries)
			} else {
				e = publishRemote(ctx, remotes[x.DestinationStorage], target, stage, entries)
			}
			if e == nil && (x.Action == "move" || x.Action == "rename") {
				if x.Storage == "" {
					localFS, openErr := fs.NewFs(ctx, a.Root)
					e = openErr
					if e == nil {
						e = verifySource(ctx, localFS, source, entries)
					}
					if e == nil {
						e = a.removeLocal(source)
					}
				} else {
					e = verifySource(ctx, remotes[x.Storage], source, entries)
					if e == nil {
						e = removeRemote(ctx, remotes[x.Storage], source, entries[0].dir)
					}
				}
				if e != nil {
					e = fmt.Errorf("copy completed at %s, but source removal failed; both copies may remain: %w", target, e)
				}
			}
		}
		os.RemoveAll(stage)
		if e != nil {
			fileActionError(w, fmt.Errorf("%s (%d items completed): %w", source, len(completed), e))
			return
		}
		completed = append(completed, target)
	}
	if x.Action == "zip" {
		err = a.zipRemote(ctx, remotes[x.Storage], destination, x.Name, x.Paths)
		if err != nil {
			fileActionError(w, err)
			return
		}
	}
	a.Log(u.ID, "storage-"+x.Action, x.Storage+":"+strings.Join(x.Paths, ", "), u.IP)
	core.JSON(w, map[string]any{"ok": true, "paths": completed})
}

func fileActionError(w http.ResponseWriter, err error) {
	var op core.OperationError
	if errors.As(err, &op) {
		core.Bad(w, err.Error(), op.Status)
		return
	}
	core.Bad(w, err.Error(), 400)
}

func (a *Module) stageSource(ctx context.Context, f fs.Fs, storage, source, stage string) ([]stagedEntry, error) {
	if storage != "" {
		return stageRemote(ctx, f, source, stage)
	}
	full, err := core.Safe(a.Root, source)
	if err != nil {
		return nil, err
	}
	// Safe rejects symlink components; every descendant is checked as well.
	err = filepath.WalkDir(full, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links cannot be transferred")
		}
		rel, e := filepath.Rel(a.Root, p)
		if e != nil {
			return e
		}
		_, e = core.CleanPath(filepath.ToSlash(rel))
		return e
	})
	if err != nil {
		return nil, err
	}
	local, err := fs.NewFs(ctx, a.Root)
	if err != nil {
		return nil, err
	}
	return stageRemote(ctx, local, source, stage)
}

func (a *Module) publishLocal(ctx context.Context, u core.User, target, stage string, entries []stagedEntry) error {
	dst, err := a.Destination(u, target)
	if err != nil {
		return err
	}
	var size int64
	for _, e := range entries {
		if !e.dir {
			size += e.size
		}
		if !a.Can(u, entryTarget(target, e.rel), true) {
			return core.Forbidden("destination permission denied")
		}
	}
	if err = a.CheckQuota(u.ID, size); err != nil {
		return core.OperationError{Message: err.Error(), Status: 413}
	}
	localStage, err := os.MkdirTemp(filepath.Dir(dst), ".trynas-import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(localStage)
	content := filepath.Join(localStage, "content")
	if err = core.CopyTree(filepath.Join(stage, "content"), content); err != nil {
		return err
	}
	localFS, err := fs.NewFs(ctx, localStage)
	if err != nil {
		return err
	}
	if err = verifyRemote(ctx, localFS, "content", entries); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = a.RecordTree(tx, content, target, u.ID); err != nil {
		return err
	}
	if _, err = os.Lstat(dst); !os.IsNotExist(err) {
		return core.Conflict("destination exists")
	}
	if err = os.Rename(content, dst); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		_ = os.RemoveAll(dst)
		return err
	}
	return nil
}

func (a *Module) canRemoveLocal(source string) error {
	rows, err := a.DB.Query("SELECT path FROM permissions UNION SELECT path FROM user_homes")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			return err
		}
		if core.Under(p, source) {
			return core.Conflict("remove explicit permissions or home-folder protection before moving this item")
		}
	}
	return rows.Err()
}
func (a *Module) removeLocal(source string) error {
	full, err := core.Safe(a.Root, source)
	if err != nil {
		return err
	}
	// Use a reversible local rename while updating ownership metadata.
	stage, err := os.MkdirTemp(filepath.Dir(full), ".trynas-moved-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM file_owners WHERE path=? OR substr(path,1,?)=?", source, len(source)+1, source+"/"); err != nil {
		return err
	}
	saved := filepath.Join(stage, "content")
	if err = os.Rename(full, saved); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		_ = os.Rename(saved, full)
		return err
	}
	return nil
}

func makeRemoteZip(ctx context.Context, f fs.Fs, paths []string) (string, error) {
	stage, err := os.MkdirTemp("", "trynas-zip-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	archive, err := os.CreateTemp("", "trynas-archive-*.zip")
	if err != nil {
		return "", err
	}
	name := archive.Name()
	success := false
	defer func() {
		archive.Close()
		if !success {
			os.Remove(name)
		}
	}()
	writer := zip.NewWriter(archive)
	names := map[string]bool{}
	var total int64
	totalEntries := 0
	for i, p := range paths {
		base := path.Base(p)
		if names[base] {
			writer.Close()
			return "", fmt.Errorf("selected items have duplicate names")
		}
		names[base] = true
		folder := filepath.Join(stage, fmt.Sprint(i))
		if err = os.Mkdir(folder, 0700); err != nil {
			writer.Close()
			return "", err
		}
		entries, e := stageRemote(ctx, f, p, folder)
		if e != nil {
			writer.Close()
			return "", e
		}
		for _, entry := range entries {
			totalEntries++
			if totalEntries > 100000 {
				writer.Close()
				return "", fmt.Errorf("select at most 100000 archive entries")
			}
			total += entry.size
			if total > 10<<30 {
				writer.Close()
				return "", fmt.Errorf("ZIP selections must be at most 10 GiB")
			}
			n := entryTarget(base, entry.rel)
			if entry.dir {
				n += "/"
			}
			h := &zip.FileHeader{Name: n, Method: zip.Deflate}
			h.SetModTime(entry.modified)
			dest, e := writer.CreateHeader(h)
			if e != nil {
				writer.Close()
				return "", e
			}
			if !entry.dir {
				file, e := os.Open(filepath.Join(folder, filepath.FromSlash(entry.rel)))
				if e != nil {
					writer.Close()
					return "", e
				}
				_, e = io.Copy(dest, file)
				file.Close()
				if e != nil {
					writer.Close()
					return "", e
				}
			}
		}
	}
	if err = writer.Close(); err != nil {
		return "", err
	}
	if err = archive.Close(); err != nil {
		return "", err
	}
	success = true
	return name, nil
}
func (a *Module) zipRemote(ctx context.Context, f fs.Fs, destination, name string, paths []string) error {
	if f == nil {
		return fmt.Errorf("ZIP is available for attached storage")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".zip") {
		name += ".zip"
	}
	archive, err := makeRemoteZip(ctx, f, paths)
	if err != nil {
		return err
	}
	defer os.Remove(archive)
	stage, err := os.MkdirTemp("", "trynas-zip-upload-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err = os.Rename(archive, filepath.Join(stage, "content")); err != nil {
		return err
	}
	file, err := os.Open(filepath.Join(stage, "content"))
	if err != nil {
		return err
	}
	h := sha256.New()
	size, err := io.Copy(h, file)
	file.Close()
	if err != nil {
		return err
	}
	return publishRemote(ctx, f, path.Join(destination, name), stage, []stagedEntry{{rel: "content", size: size, sum: hex.EncodeToString(h.Sum(nil)), modified: time.Now()}})
}

func verifySource(ctx context.Context, f fs.Fs, source string, expected []stagedEntry) error {
	stage, err := os.MkdirTemp("", "trynas-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	current, err := stageRemote(ctx, f, source, stage)
	if err != nil {
		return err
	}
	if len(current) != len(expected) {
		return fmt.Errorf("source changed during transfer; source preserved")
	}
	byPath := map[string]stagedEntry{}
	for _, entry := range expected {
		byPath[entry.rel] = entry
	}
	for _, entry := range current {
		original, ok := byPath[entry.rel]
		if !ok || entry.dir != original.dir || entry.size != original.size || entry.sum != original.sum {
			return fmt.Errorf("source changed during transfer; source preserved")
		}
	}
	return nil
}

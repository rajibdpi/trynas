package files

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rajibdpi/trynas/core"
)

func (a *Module) uploadTarget(u core.User, rel string) (string, os.FileInfo, error) {
	clean, err := core.CleanPath(rel)
	if err != nil || clean == "" {
		return "", nil, core.OperationError{Message: "invalid upload destination", Status: 400}
	}
	if !a.Can(u, clean, true) || !a.Can(u, path.Dir(clean), true) {
		return "", nil, core.Forbidden("destination folder permission denied")
	}
	full, err := core.Safe(a.Root, clean)
	if err != nil {
		return "", nil, core.OperationError{Message: "invalid upload destination", Status: 400}
	}
	parent, err := os.Stat(filepath.Dir(full))
	if err != nil || !parent.IsDir() {
		return "", nil, core.OperationError{Message: "destination folder does not exist", Status: 400}
	}
	info, err := os.Lstat(full)
	if os.IsNotExist(err) {
		return full, nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	if !info.Mode().IsRegular() {
		return "", nil, core.Conflict("a folder or unsupported item already has this name")
	}
	return full, info, nil
}

// UploadConflicts checks only authorized destinations before any file data is sent.
func (a *Module) UploadConflicts(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	var input struct {
		Path  string
		Files []struct {
			Name string
			Size int64
		}
	}
	if !core.Decode(w, r, &input) {
		return
	}
	dir, err := core.CleanPath(input.Path)
	if err != nil || len(input.Files) == 0 || len(input.Files) > 10000 {
		core.Bad(w, "invalid upload manifest", 400)
		return
	}
	if !a.Can(u, dir, true) {
		core.Bad(w, "folder permission denied", 403)
		return
	}
	conflicts := []map[string]any{}
	seen := map[string]bool{}
	for _, file := range input.Files {
		clean, err := core.CleanPath(file.Name)
		if err != nil || clean == "" || clean != file.Name || strings.Contains(clean, "/") || file.Size < 0 || seen[clean] {
			core.Bad(w, "invalid or duplicate upload filename", 400)
			return
		}
		seen[clean] = true
		_, info, err := a.uploadTarget(u, core.RelJoin(dir, clean))
		if err != nil {
			core.Report(w, err)
			return
		}
		if info != nil && info.Size() == file.Size {
			conflicts = append(conflicts, map[string]any{"name": clean, "size": file.Size})
		}
	}
	core.JSON(w, map[string]any{"conflicts": conflicts})
}

func (a *Module) numberedUploadTarget(u core.User, rel string, reserved map[string]bool) (string, string, error) {
	ext := path.Ext(rel)
	if ext == path.Base(rel) {
		ext = ""
	}
	stem := strings.TrimSuffix(rel, ext)
	for i := 1; i <= 10000; i++ {
		candidate := fmt.Sprintf("%s(%d)%s", stem, i, ext)
		if reserved[candidate] {
			continue
		}
		full, err := core.Safe(a.Root, candidate)
		if err != nil {
			return "", "", core.OperationError{Message: "invalid upload destination", Status: 400}
		}
		if _, err = os.Lstat(full); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", "", err
		}
		full, _, err = a.uploadTarget(u, candidate)
		if err != nil {
			return "", "", err
		}
		return candidate, full, nil
	}
	return "", "", core.Conflict("too many numbered copies; choose another filename")
}

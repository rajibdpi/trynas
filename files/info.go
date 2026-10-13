package files

import (
	"fmt"
	"github.com/rajibdpi/trynas/core"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (a *Module) FileInfo(w http.ResponseWriter, r *http.Request) {
	p, err := core.CleanPath(r.URL.Query().Get("path"))
	if err != nil {
		core.Bad(w, "invalid path", 400)
		return
	}
	u := r.Context().Value(core.UserKey).(core.User)
	if !a.Can(u, p, false) && !a.CanBrowse(u, p) {
		core.Bad(w, "access denied", 403)
		return
	}
	full, err := core.Safe(a.Root, p)
	if err != nil {
		core.Bad(w, "invalid path", 400)
		return
	}
	st, err := os.Stat(full)
	if err != nil {
		core.Bad(w, "file or folder not found", 404)
		return
	}
	if !a.Can(u, p, false) && !st.IsDir() {
		core.Bad(w, "access denied", 403)
		return
	}
	result := map[string]any{"name": st.Name(), "path": p, "dir": st.IsDir(), "modified": st.ModTime(), "permissions": fmt.Sprintf("%04o", st.Mode().Perm()), "writable": a.Can(u, p, true), "storage": "Local storage"}
	if !st.IsDir() {
		result["size"] = st.Size()
		result["extension"] = strings.TrimPrefix(strings.ToLower(filepath.Ext(p)), ".")
		result["mimeType"] = mime.TypeByExtension(filepath.Ext(p))
	} else {
		entries, err := os.ReadDir(full)
		if err == nil {
			files, folders := 0, 0
			for _, entry := range entries {
				if entry.Type()&os.ModeSymlink != 0 || strings.HasPrefix(entry.Name(), ".trynas-") {
					continue
				}
				child := core.RelJoin(p, entry.Name())
				if !a.Can(u, child, false) && !(entry.IsDir() && a.CanBrowse(u, child)) {
					continue
				}
				if entry.IsDir() {
					folders++
				} else {
					files++
				}
			}
			result["files"] = files
			result["folders"] = folders
		}
	}
	created, owner := filesystemMetadata(full)
	if owner != "" {
		result["owner"] = owner
	}
	if created != nil {
		result["created"] = created
		// Only attribute a creator when a creation event coincides with filesystem birth.
		var username string
		err = a.DB.QueryRow(`SELECT u.username FROM audit_logs a JOIN users u ON u.id=a.user_id WHERE a.path=? AND a.action IN ('mkdir','upload','zip') AND ABS(CAST(strftime('%s',a.created_at) AS INTEGER)-?) <= 2 ORDER BY a.id LIMIT 1`, p, created.Unix()).Scan(&username)
		if err == nil {
			result["createdBy"] = username
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	core.JSON(w, result)
}

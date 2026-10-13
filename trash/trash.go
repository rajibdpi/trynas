package trash

import (
	"net/http"
	"path/filepath"

	"github.com/rajibdpi/trynas/core"
)

func (a *Module) ListTrash(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	home := a.Home(u.ID)
	rows, e := a.DB.Query("SELECT id,original_path,deleted_at FROM trash WHERE user_id=? OR ?='admin' ORDER BY id DESC", u.ID, u.Role)
	if e != nil {
		core.Bad(w, e.Error(), 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int
		var p, t string
		_ = rows.Scan(&id, &p, &t)
		if u.Role != "admin" && (home == "" || !core.Under(p, home)) {
			continue
		}
		out = append(out, map[string]any{"id": id, "path": p, "name": filepath.Base(p), "deletedAt": t})
	}
	core.JSON(w, out)
}

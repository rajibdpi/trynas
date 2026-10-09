package favorites

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/rajib/trynas/core"
)

func (a *Module) FavoriteSet(uid int) map[string]bool {
	m := map[string]bool{}
	rows, e := a.DB.Query("SELECT path FROM favorites WHERE user_id=?", uid)
	if e != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		_ = rows.Scan(&p)
		m[p] = true
	}
	return m
}

func (a *Module) Favorites(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	rows, _ := a.DB.Query("SELECT id,path,created_at FROM favorites WHERE user_id=? ORDER BY id DESC", u.ID)
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int
		var p, t string
		_ = rows.Scan(&id, &p, &t)
		if a.Can(u, p, false) {
			out = append(out, map[string]any{"id": id, "path": p, "name": filepath.Base(p), "createdAt": t})
		}
	}
	core.JSON(w, out)
}

func (a *Module) ToggleFavorite(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	var x struct{ Path string }
	_ = json.NewDecoder(r.Body).Decode(&x)
	var id int
	e := a.DB.QueryRow("SELECT id FROM favorites WHERE user_id=? AND path=?", u.ID, x.Path).Scan(&id)
	fav := false
	if e == nil {
		_, _ = a.DB.Exec("DELETE FROM favorites WHERE id=?", id)
	} else {
		_, _ = a.DB.Exec("INSERT OR IGNORE INTO favorites(user_id,path) VALUES(?,?)", u.ID, x.Path)
		fav = true
	}
	core.JSON(w, map[string]bool{"favorite": fav})
}

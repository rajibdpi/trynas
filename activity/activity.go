package activity

import (
	"net/http"

	"github.com/rajibdpi/trynas/core"
)

func (a *Module) Activity(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	rows, _ := a.DB.Query(`SELECT a.action,COALESCE(a.path,''),a.created_at,COALESCE(u.username,'system') FROM audit_logs a LEFT JOIN users u ON u.id=a.user_id WHERE (?='admin' OR a.user_id=?) ORDER BY a.id DESC LIMIT 50`, u.Role, u.ID)
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var ac, p, t, u string
		_ = rows.Scan(&ac, &p, &t, &u)
		out = append(out, map[string]any{"action": ac, "path": p, "time": t, "user": u})
	}
	core.JSON(w, out)
}

func (a *Module) Log(uid int, act, p string) {
	_, _ = a.DB.Exec("INSERT INTO audit_logs(user_id,action,path) VALUES(?,?,?)", uid, act, p)
}

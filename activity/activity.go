package activity

import (
	"net/http"

	"github.com/rajibdpi/trynas/core"
)

func (a *Module) Activity(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	rows, _ := a.DB.Query(`SELECT a.action,COALESCE(a.path,''),a.created_at,COALESCE(u.username,'system'),COALESCE(i.ip,'') FROM audit_logs a LEFT JOIN audit_log_ips i ON i.log_id=a.id LEFT JOIN users u ON u.id=a.user_id WHERE (?='admin' OR a.user_id=?) ORDER BY a.id DESC LIMIT 50`, u.Role, u.ID)
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var ac, p, t, u, ip string
		_ = rows.Scan(&ac, &p, &t, &u, &ip)
		out = append(out, map[string]any{"action": ac, "path": p, "time": t, "user": u, "ip": ip})
	}
	core.JSON(w, out)
}

func (a *Module) Log(uid int, act, p string, ip ...string) {
	result, err := a.DB.Exec("INSERT INTO audit_logs(user_id,action,path) VALUES(?,?,?)", uid, act, p)
	if err == nil && len(ip) > 0 && ip[0] != "" {
		if id, err := result.LastInsertId(); err == nil {
			_, _ = a.DB.Exec("INSERT INTO audit_log_ips(log_id,ip) VALUES(?,?)", id, ip[0])
		}
	}
}

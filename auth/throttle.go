package auth

import (
	"net/http"

	"github.com/rajibdpi/trynas/core"
)

func (a *Module) AllowAttempt(w http.ResponseWriter, r *http.Request, key string) bool {
	if !a.Attempts.Allow("ip:"+core.PeerIP(r), 30) || !a.Attempts.Allow(key, 10) {
		w.Header().Set("Retry-After", "300")
		core.Bad(w, "too many attempts; try again later", 429)
		return false
	}
	return true
}

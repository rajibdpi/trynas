package share

import (
	"net/http"

	"github.com/rajib/trynas/core"
)

// Dependencies describes the operations this module needs from other modules.
type Dependencies interface {
	AllowAttempt(w http.ResponseWriter, r *http.Request, key string) bool
	Can(u core.User, p string, write bool) bool
	CanBrowse(u core.User, p string) bool
	Log(uid int, act, p string)
}

// Module owns this feature and uses the application's shared state.
type Module struct {
	*core.State
	Dependencies
}

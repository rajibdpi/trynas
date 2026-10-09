package files

import (
	"github.com/rajibdpi/trynas/core"
)

// Dependencies describes the operations this module needs from other modules.
type Dependencies interface {
	AllowTree(u core.User, p string, write bool) error
	Can(u core.User, p string, write bool) bool
	CanBrowse(u core.User, p string) bool
	CheckQuota(uid int, extra int64) error
	FavoriteSet(uid int) map[string]bool
	Log(uid int, act, p string)
	TrashOne(u core.User, p string) error
}

// Module owns this feature and uses the application's shared state.
type Module struct {
	*core.State
	Dependencies
}

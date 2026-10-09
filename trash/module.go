package trash

import (
	"github.com/rajibdpi/trynas/core"
)

// Dependencies describes the operations this module needs from other modules.
type Dependencies interface {
	ReadSettings() (core.AppSettings, error)
	AllowTree(u core.User, p string, write bool) error
	Can(u core.User, p string, write bool) bool
	Destination(u core.User, p string) (string, error)
	Log(uid int, act, p string)
}

// Module owns this feature and uses the application's shared state.
type Module struct {
	*core.State
	Dependencies
}

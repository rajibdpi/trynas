package providers

import (
	"github.com/rajib/trynas/core"
)

// Dependencies describes the operations this module needs from other modules.
type Dependencies interface {
	Log(uid int, act, p string)
}

// Module owns this feature and uses the application's shared state.
type Module struct {
	*core.State
	Dependencies
}

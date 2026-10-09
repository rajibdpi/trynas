package database

import (
	"github.com/rajibdpi/trynas/core"
)

// Dependencies describes the operations this module needs from other modules.
type Dependencies interface {
	NormalizeShareExpiries() error
}

// Module owns this feature and uses the application's shared state.
type Module struct {
	*core.State
	Dependencies
}

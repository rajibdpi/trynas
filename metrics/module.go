package metrics

import (
	"github.com/rajibdpi/trynas/core"
	"sync"
	"time"
)

// Dependencies describes the operations this module needs from other modules.
type Dependencies interface {
}

// Module owns this feature and uses the application's shared state.
type Module struct {
	*core.State
	Dependencies
	systemMu     sync.Mutex
	systemCached map[string]any
	systemAt     time.Time
}

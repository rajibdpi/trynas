package providers

import (
	"github.com/rajibdpi/trynas/core"
	"sync"
)

// Dependencies describes the operations this module needs from other modules.
type Dependencies interface {
	Log(uid int, act, p string, ip ...string)
}

// Module owns this feature and uses the application's shared state.
type Module struct {
	*core.State
	Dependencies
	listingMu sync.Mutex
	listings  map[listingKey]cachedListing
}

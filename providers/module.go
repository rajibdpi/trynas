package providers

import (
	"database/sql"
	"github.com/rajibdpi/trynas/core"
	"sync"
)

// Dependencies describes the operations this module needs from other modules.
type Dependencies interface {
	Can(u core.User, p string, write bool) bool
	AllowTree(u core.User, p string, write bool) error
	CheckQuota(uid int, extra int64) error
	Destination(u core.User, p string) (string, error)
	RecordTree(tx *sql.Tx, full, rel string, uid int) error
	Log(uid int, act, p string, ip ...string)
}

// Module owns this feature and uses the application's shared state.
type Module struct {
	*core.State
	Dependencies
	listingMu sync.Mutex
	listings  map[listingKey]cachedListing
}

package providers

import (
	"time"
)

const listingCacheTTL = 30 * time.Second
const listingCacheMaxBytes = 8 << 20
const listingCacheMaxEntries = 128

type listingKey struct{ storage, path string }
type cachedListing struct {
	data []byte
	at   time.Time
}

// Validate the attachment on every hit, and avoid serving listings during writes.
func (a *Module) cachedListing(id, path string) []byte {
	a.Providers.Mu.Lock()
	defer a.Providers.Mu.Unlock()
	if a.Providers.RemoteOps > 0 {
		return nil
	}
	connections, err := a.Connections()
	if err != nil {
		return nil
	}
	attached := false
	for _, c := range connections {
		if c.ID == id && c.Attached {
			attached = true
			break
		}
	}
	if !attached {
		return nil
	}
	a.listingMu.Lock()
	defer a.listingMu.Unlock()
	key := listingKey{id, path}
	entry, ok := a.listings[key]
	if !ok {
		return nil
	}
	if time.Since(entry.at) >= listingCacheTTL {
		delete(a.listings, key)
		return nil
	}
	return entry.data
}

func (a *Module) cacheListing(id, path string, data []byte) {
	if len(data) > listingCacheMaxBytes {
		return
	}
	a.listingMu.Lock()
	defer a.listingMu.Unlock()
	if a.listings == nil {
		a.listings = make(map[listingKey]cachedListing)
	}
	key := listingKey{id, path}
	delete(a.listings, key)
	total := 0
	for k, entry := range a.listings {
		if time.Since(entry.at) >= listingCacheTTL {
			delete(a.listings, k)
		} else {
			total += len(entry.data)
		}
	}
	for len(a.listings) >= listingCacheMaxEntries || total+len(data) > listingCacheMaxBytes {
		var oldest listingKey
		var at time.Time
		for k, entry := range a.listings {
			if at.IsZero() || entry.at.Before(at) {
				oldest, at = k, entry.at
			}
		}
		total -= len(a.listings[oldest].data)
		delete(a.listings, oldest)
	}
	a.listings[key] = cachedListing{data: data, at: time.Now()}
}

func (a *Module) invalidateListings(id string) {
	a.listingMu.Lock()
	defer a.listingMu.Unlock()
	for key := range a.listings {
		if key.storage == id {
			delete(a.listings, key)
		}
	}
}

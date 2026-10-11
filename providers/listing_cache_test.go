package providers

import (
	"fmt"
	"testing"
	"time"
)

func TestListingCacheBoundsAndExpiry(t *testing.T) {
	a := &Module{}
	a.cacheListing("remote", "expired", []byte("old"))
	key := listingKey{"remote", "expired"}
	entry := a.listings[key]
	entry.at = time.Now().Add(-listingCacheTTL)
	a.listings[key] = entry
	for i := 0; i < listingCacheMaxEntries+5; i++ {
		a.cacheListing("remote", fmt.Sprint(i), make([]byte, 100000))
	}
	total := 0
	for _, entry := range a.listings {
		total += len(entry.data)
	}
	if len(a.listings) > listingCacheMaxEntries || total > listingCacheMaxBytes {
		t.Fatal("cache exceeded bounds")
	}
	if _, ok := a.listings[key]; ok {
		t.Fatal("expired entry retained")
	}
	a.invalidateListings("remote")
	if len(a.listings) != 0 {
		t.Fatal("connection cache not invalidated")
	}
}

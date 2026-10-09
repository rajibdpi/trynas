package metrics

import (
	"sync"
	"testing"
	"time"

	"github.com/rajibdpi/trynas/core"
)

func TestSystemCacheSharedAndExpires(t *testing.T) {
	a := &Module{State: &core.State{Root: t.TempDir()}, systemCached: map[string]any{"cpu": 42.0}, systemAt: time.Now()}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			result, err := a.systemSnapshot()
			if err != nil || result["cpu"] != 42.0 {
				t.Errorf("cached sample not shared: %v, %v", result, err)
			}
		})
	}
	wg.Wait()
	a.systemAt = time.Now().Add(-2 * time.Second)
	fresh, err := a.systemSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if fresh["storage"] == nil {
		t.Fatal("expired cache was not refreshed")
	}
	if !a.systemAt.After(time.Now().Add(-time.Second)) {
		t.Fatal("sample timestamp not updated")
	}
}

func BenchmarkSystemSnapshotCached(b *testing.B) {
	a := &Module{State: &core.State{Root: b.TempDir()}}
	if _, err := a.systemSnapshot(); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := a.systemSnapshot(); err != nil {
			b.Fatal(err)
		}
	}
}

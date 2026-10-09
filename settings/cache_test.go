package settings

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rajibdpi/trynas/core"
	"github.com/rajibdpi/trynas/database"
)

func newTestModule(t testing.TB) *Module {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec("CREATE TABLE app_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	return &Module{State: &core.State{DB: db}}
}

func TestSettingsCacheExpiresAndDoesNotCacheErrors(t *testing.T) {
	a := newTestModule(t)
	first, err := a.ReadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(`INSERT INTO app_settings VALUES ('basic', '{"appName":"Updated","fileView":"list","shareExpiry":"1"}')`); err != nil {
		t.Fatal(err)
	}
	cached, err := a.ReadSettings()
	if err != nil || cached != first {
		t.Fatalf("expected cached defaults: %v, %v", cached, err)
	}
	a.cachedAt = time.Now().Add(-31 * time.Second)
	updated, err := a.ReadSettings()
	if err != nil || updated.AppName != "Updated" {
		t.Fatalf("expected refreshed settings: %v, %v", updated, err)
	}
	a.cachedAt = time.Now().Add(-31 * time.Second)
	if _, err := a.DB.Exec(`UPDATE app_settings SET value='invalid'`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ReadSettings(); err == nil {
		t.Fatal("expected decoding error")
	}
	if _, err := a.DB.Exec(`UPDATE app_settings SET value='{"appName":"Recovered"}'`); err != nil {
		t.Fatal(err)
	}
	recovered, err := a.ReadSettings()
	if err != nil || recovered.AppName != "Recovered" {
		t.Fatalf("error was cached: %v, %v", recovered, err)
	}
}

func BenchmarkReadSettingsCached(b *testing.B) {
	a := newTestModule(b)
	if _, err := a.ReadSettings(); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := a.ReadSettings(); err != nil {
			b.Fatal(err)
		}
	}
}

package app

import (
	"database/sql"

	"github.com/rajibdpi/trynas/activity"
	"github.com/rajibdpi/trynas/auth"
	"github.com/rajibdpi/trynas/core"
	"github.com/rajibdpi/trynas/database"
	"github.com/rajibdpi/trynas/favorites"
	"github.com/rajibdpi/trynas/files"
	"github.com/rajibdpi/trynas/metrics"
	"github.com/rajibdpi/trynas/permissions"
	storage "github.com/rajibdpi/trynas/providers"
	"github.com/rajibdpi/trynas/settings"
	"github.com/rajibdpi/trynas/share"
	"github.com/rajibdpi/trynas/trash"
	"github.com/rajibdpi/trynas/users"
)

// Module aliases preserve the existing field names while promoting each
// feature's methods onto App. Cross-feature dependency interfaces stay explicit.
type ActivityModule = activity.Module
type AuthModule = auth.Module
type DatabaseModule = database.Module
type FavoritesModule = favorites.Module
type FilesModule = files.Module
type MetricsModule = metrics.Module
type PermissionsModule = permissions.Module
type SettingsModule = settings.Module
type ShareModule = share.Module
type StorageModule = storage.Module
type TrashModule = trash.Module
type UsersModule = users.Module

// App composes feature modules with shared state and cross-module dependencies.
type App struct {
	*core.State
	*ActivityModule
	*AuthModule
	*DatabaseModule
	*FavoritesModule
	*FilesModule
	*MetricsModule
	*PermissionsModule
	*SettingsModule
	*ShareModule
	*StorageModule
	*TrashModule
	*UsersModule
}

// New wires every feature to the same database, storage paths, and synchronization locks.
func New(db *sql.DB, root, trashPath string) *App {
	a := &App{State: &core.State{DB: db, Root: root, Trash: trashPath}}
	a.ActivityModule = &activity.Module{State: a.State, Dependencies: a}
	a.AuthModule = &auth.Module{State: a.State, Dependencies: a}
	a.DatabaseModule = &database.Module{State: a.State, Dependencies: a}
	a.FavoritesModule = &favorites.Module{State: a.State, Dependencies: a}
	a.FilesModule = &files.Module{State: a.State, Dependencies: a}
	a.MetricsModule = &metrics.Module{State: a.State, Dependencies: a}
	a.PermissionsModule = &permissions.Module{State: a.State, Dependencies: a}
	a.SettingsModule = &settings.Module{State: a.State, Dependencies: a}
	a.ShareModule = &share.Module{State: a.State, Dependencies: a}
	a.StorageModule = &storage.Module{State: a.State, Dependencies: a}
	a.TrashModule = &trash.Module{State: a.State, Dependencies: a}
	a.UsersModule = &users.Module{State: a.State, Dependencies: a}
	return a
}

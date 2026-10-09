package app

import (
	"context"
	"crypto/cipher"
	"database/sql"
	"net/http"

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
	"github.com/rclone/rclone/fs"
)

// App composes feature modules with shared state and cross-module dependencies.
type App struct {
	*core.State
	ActivityModule    *activity.Module
	AuthModule        *auth.Module
	DatabaseModule    *database.Module
	FavoritesModule   *favorites.Module
	FilesModule       *files.Module
	MetricsModule     *metrics.Module
	PermissionsModule *permissions.Module
	SettingsModule    *settings.Module
	ShareModule       *share.Module
	StorageModule     *storage.Module
	TrashModule       *trash.Module
	UsersModule       *users.Module
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
func (a *App) Activity(w http.ResponseWriter, r *http.Request) { a.ActivityModule.Activity(w, r) }
func (a *App) AllowAttempt(w http.ResponseWriter, r *http.Request, key string) bool {
	return a.AuthModule.AllowAttempt(w, r, key)
}
func (a *App) AllowTree(u core.User, p string, write bool) error {
	return a.PermissionsModule.AllowTree(u, p, write)
}
func (a *App) Auth(next http.Handler) http.Handler { return a.AuthModule.Auth(next) }
func (a *App) AuthorizeShare(w http.ResponseWriter, r *http.Request, id int64, tok, hash string) bool {
	return a.ShareModule.AuthorizeShare(w, r, id, tok, hash)
}
func (a *App) Bulk(w http.ResponseWriter, r *http.Request) { a.FilesModule.Bulk(w, r) }
func (a *App) Can(u core.User, p string, write bool) bool {
	return a.PermissionsModule.Can(u, p, write)
}
func (a *App) CanBrowse(u core.User, p string) bool           { return a.PermissionsModule.CanBrowse(u, p) }
func (a *App) CheckQuota(uid int, extra int64) error          { return a.UsersModule.CheckQuota(uid, extra) }
func (a *App) Connections() ([]core.StorageConnection, error) { return a.StorageModule.Connections() }
func (a *App) CopyMoveV5(move bool) http.HandlerFunc          { return a.FilesModule.CopyMoveV5(move) }
func (a *App) CopyOrMove(u core.User, from, to string, move bool) error {
	return a.FilesModule.CopyOrMove(u, from, to, move)
}
func (a *App) Cors(next http.Handler) http.Handler                { return a.AuthModule.Cors(next) }
func (a *App) CreateShare(w http.ResponseWriter, r *http.Request) { a.ShareModule.CreateShare(w, r) }
func (a *App) DecryptConnections(raw []byte) ([]byte, error) {
	return a.StorageModule.DecryptConnections(raw)
}
func (a *App) DeleteShare(w http.ResponseWriter, r *http.Request) { a.ShareModule.DeleteShare(w, r) }
func (a *App) DeleteUser(w http.ResponseWriter, r *http.Request)  { a.UsersModule.DeleteUser(w, r) }
func (a *App) Destination(u core.User, p string) (string, error) {
	return a.FilesModule.Destination(u, p)
}
func (a *App) Download(w http.ResponseWriter, r *http.Request) { a.FilesModule.Download(w, r) }
func (a *App) DownloadSharedFolder(w http.ResponseWriter, r *http.Request, owner core.User, p, full string) {
	a.ShareModule.DownloadSharedFolder(w, r, owner, p, full)
}
func (a *App) DownloadSharedItems(w http.ResponseWriter, r *http.Request, owner core.User, paths []string, name string) {
	a.ShareModule.DownloadSharedItems(w, r, owner, paths, name)
}
func (a *App) EncryptConnections(plain []byte) ([]byte, error) {
	return a.StorageModule.EncryptConnections(plain)
}
func (a *App) FavoriteSet(uid int) map[string]bool              { return a.FavoritesModule.FavoriteSet(uid) }
func (a *App) Favorites(w http.ResponseWriter, r *http.Request) { a.FavoritesModule.Favorites(w, r) }
func (a *App) Guard(next http.Handler) http.Handler             { return a.PermissionsModule.Guard(next) }
func (a *App) Hardware(w http.ResponseWriter, r *http.Request)  { a.MetricsModule.Hardware(w, r) }
func (a *App) ListFiles(w http.ResponseWriter, r *http.Request) { a.FilesModule.ListFiles(w, r) }
func (a *App) ListSharedItems(owner core.User, paths []string) ([]core.SharedPageItem, bool, error) {
	return a.ShareModule.ListSharedItems(owner, paths)
}
func (a *App) ListTrash(w http.ResponseWriter, r *http.Request) { a.TrashModule.ListTrash(w, r) }
func (a *App) Log(uid int, act, p string)                       { a.ActivityModule.Log(uid, act, p) }
func (a *App) Login(w http.ResponseWriter, r *http.Request)     { a.AuthModule.Login(w, r) }
func (a *App) Logout(w http.ResponseWriter, r *http.Request)    { a.AuthModule.Logout(w, r) }
func (a *App) Me(w http.ResponseWriter, r *http.Request)        { a.AuthModule.Me(w, r) }
func (a *App) Migrate() error                                   { return a.DatabaseModule.Migrate() }
func (a *App) MigrateV5() error                                 { return a.DatabaseModule.MigrateV5() }
func (a *App) Mkdir(w http.ResponseWriter, r *http.Request)     { a.FilesModule.Mkdir(w, r) }
func (a *App) MonitorNetwork()                                  { a.MetricsModule.MonitorNetwork() }
func (a *App) MoveOwned(from, to, src, dst string) error {
	return a.FilesModule.MoveOwned(from, to, src, dst)
}
func (a *App) Network(w http.ResponseWriter, r *http.Request) { a.MetricsModule.Network(w, r) }
func (a *App) NormalizeExpiries(table, key string) error {
	return a.ShareModule.NormalizeExpiries(table, key)
}
func (a *App) NormalizeShareExpiries() error { return a.ShareModule.NormalizeShareExpiries() }
func (a *App) OpenAttachedStorage(ctx context.Context, id string) (fs.Fs, func(), int) {
	return a.StorageModule.OpenAttachedStorage(ctx, id)
}
func (a *App) Permissions(w http.ResponseWriter, r *http.Request) {
	a.PermissionsModule.Permissions(w, r)
}
func (a *App) Preview(w http.ResponseWriter, r *http.Request)     { a.FilesModule.Preview(w, r) }
func (a *App) Profile(u core.User) map[string]any                 { return a.UsersModule.Profile(u) }
func (a *App) PublicShare(w http.ResponseWriter, r *http.Request) { a.ShareModule.PublicShare(w, r) }
func (a *App) PurgeV5(w http.ResponseWriter, r *http.Request)     { a.TrashModule.PurgeV5(w, r) }
func (a *App) Quota(w http.ResponseWriter, r *http.Request)       { a.UsersModule.Quota(w, r) }
func (a *App) ReadSettings() (core.AppSettings, error)            { return a.SettingsModule.ReadSettings() }
func (a *App) ReconcileStorageOwners(local string, uid int) error {
	return a.StorageModule.ReconcileStorageOwners(local, uid)
}
func (a *App) RecordTree(tx *sql.Tx, full, rel string, uid int) error {
	return a.FilesModule.RecordTree(tx, full, rel, uid)
}
func (a *App) RemoteStorage(w http.ResponseWriter, r *http.Request) {
	a.StorageModule.RemoteStorage(w, r)
}
func (a *App) RenameV5(w http.ResponseWriter, r *http.Request)  { a.FilesModule.RenameV5(w, r) }
func (a *App) RestoreV5(w http.ResponseWriter, r *http.Request) { a.TrashModule.RestoreV5(w, r) }
func (a *App) RunTransfer(ctx context.Context, job *core.TransferJob, c core.StorageConnection, index int, u core.User) {
	a.StorageModule.RunTransfer(ctx, job, c, index, u)
}
func (a *App) SampleNetwork() { a.MetricsModule.SampleNetwork() }
func (a *App) SaveConnections(c []core.StorageConnection) error {
	return a.StorageModule.SaveConnections(c)
}
func (a *App) SavePermission(w http.ResponseWriter, r *http.Request) {
	a.PermissionsModule.SavePermission(w, r)
}
func (a *App) SaveStorageJob(job *core.TransferJob) error         { return a.StorageModule.SaveStorageJob(job) }
func (a *App) SaveUser(w http.ResponseWriter, r *http.Request)    { a.UsersModule.SaveUser(w, r) }
func (a *App) SearchFiles(w http.ResponseWriter, r *http.Request) { a.FilesModule.SearchFiles(w, r) }
func (a *App) Settings(w http.ResponseWriter, r *http.Request)    { a.SettingsModule.Settings(w, r) }
func (a *App) SharePaths(id int64, fallback string) ([]string, error) {
	return a.ShareModule.SharePaths(id, fallback)
}
func (a *App) Shares(w http.ResponseWriter, r *http.Request) { a.ShareModule.Shares(w, r) }
func (a *App) StorageCipher() (cipher.AEAD, error)           { return a.StorageModule.StorageCipher() }
func (a *App) StorageProviders(w http.ResponseWriter, r *http.Request) {
	a.StorageModule.StorageProviders(w, r)
}
func (a *App) System(w http.ResponseWriter, r *http.Request) { a.MetricsModule.System(w, r) }
func (a *App) ToggleFavorite(w http.ResponseWriter, r *http.Request) {
	a.FavoritesModule.ToggleFavorite(w, r)
}
func (a *App) TrashOne(u core.User, p string) error            { return a.TrashModule.TrashOne(u, p) }
func (a *App) TrashV5(w http.ResponseWriter, r *http.Request)  { a.TrashModule.TrashV5(w, r) }
func (a *App) UploadV5(w http.ResponseWriter, r *http.Request) { a.FilesModule.UploadV5(w, r) }
func (a *App) Usage(uid int) (int64, int64, error)             { return a.UsersModule.Usage(uid) }
func (a *App) Users(w http.ResponseWriter, r *http.Request)    { a.UsersModule.Users(w, r) }
func (a *App) ZipOne(u core.User, p string) error              { return a.FilesModule.ZipOne(u, p) }
func (a *App) ZipV5(w http.ResponseWriter, r *http.Request)    { a.FilesModule.ZipV5(w, r) }

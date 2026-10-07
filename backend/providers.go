package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/rclone/rclone/backend/drive"
	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/s3"
	_ "github.com/rclone/rclone/backend/sftp"
	_ "github.com/rclone/rclone/backend/smb"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/filter"
	rcsync "github.com/rclone/rclone/fs/sync"
)

type storageConnection struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Provider string            `json:"provider"`
	Root     string            `json:"root"`
	Attached bool              `json:"attached"`
	Options  map[string]string `json:"options,omitempty"`
}
type transferJob struct {
	ID         string `json:"id"`
	Connection string `json:"connection"`
	Direction  string `json:"direction"`
	Mode       string `json:"mode"`
	LocalPath  string `json:"localPath"`
	DryRun     bool   `json:"dryRun"`
	Status     string `json:"status"`
	Started    string `json:"started"`
	Finished   string `json:"finished,omitempty"`
	Bytes      int64  `json:"bytes"`
	Files      int64  `json:"files"`
	Deletes    int64  `json:"deletes"`
	stats      *accounting.StatsInfo
	cancel     context.CancelFunc
}
type providerState struct {
	mu        sync.Mutex
	job       *transferJob
	group     string
	remoteOps int
}

var providerFields = map[string][]string{
	"drive": {"token", "client_id", "client_secret", "root_folder_id"},
	"sftp":  {"host", "user", "port", "pass", "key_pem", "known_hosts_file"},
	"smb":   {"host", "user", "port", "pass", "domain"},
	"s3":    {"provider", "access_key_id", "secret_access_key", "region", "endpoint"},
}

func (a *App) connections() ([]storageConnection, error) {
	var raw string
	err := a.db.QueryRow("SELECT value FROM app_settings WHERE key='storage_connections'").Scan(&raw)
	if err == sql.ErrNoRows {
		return []storageConnection{}, nil
	}
	if err != nil {
		return nil, err
	}
	var c []storageConnection
	plain, err := a.decryptConnections([]byte(raw))
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(plain, &c)
	return c, err
}
func (a *App) saveConnections(c []storageConnection) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	encrypted, err := a.encryptConnections(raw)
	if err != nil {
		return err
	}
	_, err = a.db.Exec("INSERT INTO app_settings(key,value) VALUES('storage_connections',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(encrypted))
	return err
}
func validateConnection(c *storageConnection) error {
	fields, ok := providerFields[c.Provider]
	if !ok {
		return fmt.Errorf("unsupported provider")
	}
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" || len(c.Name) > 80 {
		return fmt.Errorf("name must be 1–80 characters")
	}
	if strings.ContainsAny(c.Root, "\x00\r\n") || len(c.Root) > 1024 {
		return fmt.Errorf("invalid remote directory")
	}
	for key := range c.Options {
		allowed := false
		for _, field := range fields {
			if field == key {
				allowed = true
			}
		}
		if !allowed {
			return fmt.Errorf("unsupported connection option: %s", key)
		}
	}
	if port := c.Options["port"]; port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("port must be 1–65535")
		}
	}
	if c.Provider == "sftp" || c.Provider == "smb" {
		if c.Options["host"] == "" || c.Options["user"] == "" {
			return fmt.Errorf("host and username are required")
		}
	}
	if c.Provider == "sftp" {
		if c.Options["known_hosts_file"] == "" || c.Options["known_hosts_file"] == "none" {
			return fmt.Errorf("SFTP requires a trusted known_hosts file on the server")
		}
		if c.Options["pass"] == "" && c.Options["key_pem"] == "" {
			return fmt.Errorf("SFTP requires a password or private key")
		}
	}
	if c.Provider == "drive" {
		var token struct {
			RefreshToken string `json:"refresh_token"`
		}
		if json.Unmarshal([]byte(c.Options["token"]), &token) != nil || token.RefreshToken == "" {
			return fmt.Errorf("provide an rclone Google Drive OAuth token with a refresh token")
		}
	}
	if c.Provider == "s3" && (c.Options["access_key_id"] == "" || c.Options["secret_access_key"] == "" || c.Root == "") {
		return fmt.Errorf("S3 requires access keys and a bucket directory")
	}
	return nil
}
func newProviderFS(ctx context.Context, c storageConnection) (fs.Fs, configmap.Simple, error) {
	info, err := fs.Find(c.Provider)
	if err != nil {
		return nil, nil, err
	}
	options := configmap.Simple{}
	for _, o := range info.Options {
		if o.Default != nil {
			options[o.Name] = fmt.Sprint(o.Default)
		}
	}
	for k, v := range c.Options {
		if v != "" {
			options[k] = v
		}
	}
	if options["pass"] != "" {
		options["pass"], err = obscure.Obscure(options["pass"])
		if err != nil {
			return nil, nil, err
		}
	}
	if c.Provider == "drive" {
		options["scope"] = "drive"
	}
	if c.Provider == "sftp" {
		options["disable_hashcheck"] = "true"
		options["shell_type"] = "none"
	}
	if c.Provider == "s3" && options["provider"] == "" {
		options["provider"] = "Other"
	}
	f, err := info.NewFs(ctx, "trynas_"+c.ID, c.Root, options)
	return f, options, err
}
func (a *App) storageProviders(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	if u.Role != "admin" {
		bad(w, "administrator required", 403)
		return
	}
	a.providers.mu.Lock()
	defer a.providers.mu.Unlock()
	connections, err := a.connections()
	if err != nil {
		bad(w, "could not load connections", 500)
		return
	}
	if a.providers.job == nil {
		var raw string
		err := a.db.QueryRow("SELECT value FROM app_settings WHERE key='storage_job'").Scan(&raw)
		if err == nil {
			var previous transferJob
			if json.Unmarshal([]byte(raw), &previous) == nil {
				if previous.Status == "running" {
					previous.Status = "interrupted"
				}
				a.providers.job = &previous
			}
		}
	}
	if a.providers.job != nil {
		a.providers.job.updateStats()
	}
	if r.Method == http.MethodGet {
		safeConnections := make([]storageConnection, 0, len(connections))
		for _, c := range connections {
			c.Options = nil
			safeConnections = append(safeConnections, c)
		}
		jsonOut(w, map[string]any{"connections": safeConnections, "job": a.providers.job})
		return
	}
	var input struct {
		Action        string            `json:"action"`
		Connection    storageConnection `json:"connection"`
		ID            string            `json:"id"`
		Direction     string            `json:"direction"`
		Mode          string            `json:"mode"`
		LocalPath     string            `json:"localPath"`
		DryRun        bool              `json:"dryRun"`
		ConfirmDelete bool              `json:"confirmDelete"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&input) != nil {
		bad(w, "invalid request", 400)
		return
	}
	if input.Action == "cancel" {
		if a.providers.job != nil && a.providers.job.Status == "running" {
			a.providers.job.cancel()
		}
		jsonOut(w, map[string]bool{"ok": true})
		return
	}
	if a.providers.job != nil && a.providers.job.Status == "running" {
		bad(w, "wait for the current transfer or cancel it", 409)
		return
	}
	if input.Action == "save" {
		c := input.Connection
		c.ID = token()
		if err := validateConnection(&c); err != nil {
			bad(w, err.Error(), 400)
			return
		}
		if len(connections) >= 50 {
			bad(w, "connection limit reached", 400)
			return
		}
		connections = append(connections, c)
		if a.saveConnections(connections) != nil {
			bad(w, "could not save connection", 500)
			return
		}
		a.log(u.ID, "storage-connect", c.Name)
		c.Options = nil
		jsonOut(w, c)
		return
	}
	var selected storageConnection
	index := -1
	for i, c := range connections {
		if c.ID == input.ID {
			selected = c
			index = i
			break
		}
	}
	if index < 0 {
		bad(w, "connection not found", 404)
		return
	}
	if input.Action == "attach" || input.Action == "detach" {
		connections[index].Attached = input.Action == "attach"
		if a.saveConnections(connections) != nil {
			bad(w, "could not update attachment", 500)
			return
		}
		a.log(u.ID, "storage-"+input.Action, selected.Name)
		jsonOut(w, map[string]bool{"ok": true})
		return
	}
	if input.Action == "delete" {
		connections = append(connections[:index], connections[index+1:]...)
		if a.saveConnections(connections) != nil {
			bad(w, "could not remove connection", 500)
			return
		}
		a.log(u.ID, "storage-disconnect", selected.Name)
		jsonOut(w, map[string]bool{"ok": true})
		return
	}
	if input.Action == "test" {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		f, options, err := newProviderFS(ctx, selected)
		if err == nil {
			_, err = f.List(ctx, "")
		}
		if err != nil {
			bad(w, "connection test failed; check credentials, host verification and remote directory", 400)
			return
		}
		if options["token"] != "" {
			connections[index].Options["token"] = options["token"]
			if a.saveConnections(connections) != nil {
				bad(w, "could not persist refreshed token", 500)
				return
			}
		}
		jsonOut(w, map[string]bool{"ok": true})
		return
	}
	if input.Action == "transfer" && a.providers.remoteOps > 0 {
		bad(w, "wait for active storage operations", 409)
		return
	}
	if input.Action != "transfer" {
		bad(w, "unknown action", 400)
		return
	}
	if input.Direction != "upload" && input.Direction != "download" {
		bad(w, "choose upload or download", 400)
		return
	}
	if input.Mode != "copy" && input.Mode != "mirror" {
		bad(w, "choose copy or mirror", 400)
		return
	}
	if input.Mode == "mirror" && !input.DryRun && !input.ConfirmDelete {
		bad(w, "confirm deletion of extra destination files", 400)
		return
	}
	local, err := safe(a.root, input.LocalPath)
	if err != nil {
		bad(w, "invalid local directory", 400)
		return
	}
	st, err := os.Stat(local)
	if err != nil && !(os.IsNotExist(err) && input.Direction == "download") || err == nil && !st.IsDir() {
		bad(w, "choose a local directory", 400)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	job := &transferJob{ID: token(), Connection: selected.Name, Direction: input.Direction, Mode: input.Mode, LocalPath: input.LocalPath, DryRun: input.DryRun, Status: "running", Started: time.Now().UTC().Format(time.RFC3339), cancel: cancel}
	if a.providers.group == "" {
		a.providers.group = token()
	}
	ctx = accounting.WithStatsGroup(ctx, a.providers.group)
	job.stats = accounting.Stats(ctx)
	job.stats.ResetCounters()
	job.stats.ResetErrors()
	if err := a.saveStorageJob(job); err != nil {
		cancel()
		bad(w, "could not save transfer", 500)
		return
	}
	a.providers.job = job
	go a.runTransfer(ctx, job, selected, index, u)
	jsonOut(w, job)
}
func (a *App) runTransfer(ctx context.Context, job *transferJob, c storageConnection, index int, u User) {
	defer job.cancel()
	ctx, config := fs.AddConfig(ctx)
	config.DryRun = job.DryRun
	config.Checkers = 4
	config.Transfers = 2
	config.Retries = 1
	ctx, fi := filter.AddConfig(ctx)
	_ = fi.AddRule("- .trynas-*/**")
	_ = fi.AddRule("- .trynas-*")
	a.mutation.Lock()
	local, err := safe(a.root, job.LocalPath)
	if err == nil {
		err = ctx.Err()
	}
	if err == nil && job.Direction == "download" && !job.DryRun {
		err = os.MkdirAll(local, 0755)
	}
	if err == nil {
		err = filepath.WalkDir(local, func(path string, d os.DirEntry, e error) error {
			if os.IsNotExist(e) && job.Direction == "download" && path == local {
				return nil
			}
			if e != nil {
				return e
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("local directory contains a symlink")
			}
			if d.IsDir() && strings.HasPrefix(d.Name(), ".trynas-") {
				return filepath.SkipDir
			}
			return nil
		})
	}
	var options configmap.Simple
	if err == nil {
		var remote, localFS fs.Fs
		remote, options, err = newProviderFS(ctx, c)
		if err == nil {
			localFS, err = fs.NewFs(ctx, local)
		}
		if err == nil {
			src, dst := localFS, remote
			if job.Direction == "download" {
				src, dst = remote, localFS
			}
			if job.Mode == "mirror" {
				err = rcsync.Sync(ctx, dst, src, true)
			} else {
				err = rcsync.CopyDir(ctx, dst, src, true)
			}
		}
	}
	if job.Direction == "download" && !job.DryRun {
		if accountingErr := a.reconcileStorageOwners(local, u.ID); accountingErr != nil && err == nil {
			err = accountingErr
		}
	}
	a.mutation.Unlock()
	a.providers.mu.Lock()
	defer a.providers.mu.Unlock()
	if options != nil && options["token"] != "" {
		connections, e := a.connections()
		if e == nil && index < len(connections) && connections[index].ID == c.ID {
			connections[index].Options["token"] = options["token"]
			if e = a.saveConnections(connections); e != nil && err == nil {
				err = e
			}
		}
	}
	job.Status = "completed"
	if err != nil {
		job.Status = "failed"
	}
	if ctx.Err() != nil {
		job.Status = "cancelled"
	}
	job.Finished = time.Now().UTC().Format(time.RFC3339)
	job.updateStats()
	if a.saveStorageJob(job) != nil {
		job.Status = "failed"
	}
	a.log(u.ID, "storage-"+job.Status, c.Name+" "+job.Direction+" "+job.Mode)
}

func (j *transferJob) updateStats() {
	if j.stats != nil {
		j.Bytes = j.stats.GetBytes()
		j.Files = j.stats.GetTransfers()
		j.Deletes = j.stats.GetDeletes()
	}
}
func (a *App) saveStorageJob(job *transferJob) error {
	raw, err := json.Marshal(job)
	if err != nil {
		return err
	}
	_, err = a.db.Exec("INSERT INTO app_settings(key,value) VALUES('storage_job',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(raw))
	return err
}

// Credentials are encrypted independently from the database so database backups
// alone do not disclose provider passwords or OAuth tokens.
func (a *App) storageCipher() (cipher.AEAD, error) {
	var seq int
	var name, dbPath string
	if err := a.db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &dbPath); err != nil {
		return nil, err
	}
	if dbPath == "" {
		return nil, fmt.Errorf("storage connections require a file-backed database")
	}
	keyPath := filepath.Join(filepath.Dir(dbPath), ".trynas-storage.key")
	key, err := os.ReadFile(keyPath)
	if os.IsNotExist(err) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		var f *os.File
		f, err = os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return nil, err
		}
		_, err = f.Write(key)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid storage encryption key")
	}
	if err = os.Chmod(keyPath, 0600); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

type encryptedConnections struct {
	Nonce []byte `json:"nonce"`
	Data  []byte `json:"data"`
}

func (a *App) encryptConnections(plain []byte) ([]byte, error) {
	gcm, err := a.storageCipher()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return json.Marshal(encryptedConnections{nonce, gcm.Seal(nil, nonce, plain, []byte("trynas-storage-v1"))})
}
func (a *App) decryptConnections(raw []byte) ([]byte, error) {
	var encrypted encryptedConnections
	if err := json.Unmarshal(raw, &encrypted); err != nil {
		return nil, err
	}
	gcm, err := a.storageCipher()
	if err != nil {
		return nil, err
	}
	if len(encrypted.Nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid connection nonce")
	}
	return gcm.Open(nil, encrypted.Nonce, encrypted.Data, []byte("trynas-storage-v1"))
}

// Refresh sizes for existing owners and attribute imported files to the admin.
// Run after partial transfers too: cancellation does not undo completed writes.
func (a *App) reconcileStorageOwners(local string, uid int) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rel, err := filepath.Rel(a.root, local)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		rel = ""
	}
	rows, err := tx.Query("SELECT path FROM file_owners")
	if err != nil {
		return err
	}
	var stale []string
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		if under(p, rel) {
			full, e := safe(a.root, p)
			if e != nil {
				rows.Close()
				return e
			}
			st, e := os.Stat(full)
			if os.IsNotExist(e) || e == nil && !st.Mode().IsRegular() {
				stale = append(stale, p)
			} else if e != nil {
				rows.Close()
				return e
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, p := range stale {
		if _, err = tx.Exec("DELETE FROM file_owners WHERE path=?", p); err != nil {
			return err
		}
	}
	err = filepath.WalkDir(local, func(full string, d os.DirEntry, e error) error {
		if os.IsNotExist(e) && full == local {
			return nil
		}
		if e != nil {
			return e
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".trynas-") {
			return filepath.SkipDir
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not supported")
		}
		if d.IsDir() {
			return nil
		}
		st, e := d.Info()
		if e != nil {
			return e
		}
		if !st.Mode().IsRegular() {
			return nil
		}
		p, e := filepath.Rel(a.root, full)
		if e != nil {
			return e
		}
		p, e = cleanPath(filepath.ToSlash(p))
		if e != nil {
			return e
		}
		_, e = tx.Exec("INSERT INTO file_owners(path,user_id,bytes) VALUES(?,?,?) ON CONFLICT(path) DO UPDATE SET bytes=excluded.bytes", p, uid, st.Size())
		return e
	})
	if err != nil {
		return err
	}
	return tx.Commit()
}

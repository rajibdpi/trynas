package providers

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
	"time"

	"github.com/rajibdpi/trynas/core"
	_ "github.com/rclone/rclone/backend/drive"
	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/s3"
	_ "github.com/rclone/rclone/backend/sftp"
	_ "github.com/rclone/rclone/backend/smb"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
)

var ProviderFields = map[string][]string{
	"drive": {"token", "client_id", "client_secret", "root_folder_id"},
	"sftp":  {"host", "user", "port", "pass", "key_pem", "known_hosts_file"},
	"smb":   {"host", "user", "port", "pass", "domain"},
	"s3":    {"provider", "access_key_id", "secret_access_key", "region", "endpoint"},
}

func (a *Module) Connections() ([]core.StorageConnection, error) {
	var raw string
	err := a.DB.QueryRow("SELECT value FROM app_settings WHERE key='storage_connections'").Scan(&raw)
	if err == sql.ErrNoRows {
		return []core.StorageConnection{}, nil
	}
	if err != nil {
		return nil, err
	}
	var c []core.StorageConnection
	plain, err := a.DecryptConnections([]byte(raw))
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(plain, &c)
	return c, err
}

func (a *Module) SaveConnections(c []core.StorageConnection) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	encrypted, err := a.EncryptConnections(raw)
	if err != nil {
		return err
	}
	_, err = a.DB.Exec("INSERT INTO app_settings(key,value) VALUES('storage_connections',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(encrypted))
	return err
}

func ValidateConnection(c *core.StorageConnection) error {
	fields, ok := ProviderFields[c.Provider]
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

func NewProviderFS(ctx context.Context, c core.StorageConnection) (fs.Fs, configmap.Simple, error) {
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

func (a *Module) StorageProviders(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	if u.Role != "admin" {
		core.Bad(w, "administrator required", 403)
		return
	}
	a.Providers.Mu.Lock()
	defer a.Providers.Mu.Unlock()
	connections, err := a.Connections()
	if err != nil {
		core.Bad(w, "could not load connections", 500)
		return
	}
	if r.Method == http.MethodGet {
		safeConnections := make([]core.StorageConnection, 0, len(connections))
		for _, c := range connections {
			c.Options = nil
			safeConnections = append(safeConnections, c)
		}
		core.JSON(w, map[string]any{"connections": safeConnections})
		return
	}
	var input struct {
		Action     string                 `json:"action"`
		Connection core.StorageConnection `json:"connection"`
		ID         string                 `json:"id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&input) != nil {
		core.Bad(w, "invalid request", 400)
		return
	}
	if input.Action != "save" && input.Action != "attach" && input.Action != "detach" && input.Action != "test" && input.Action != "delete" {
		core.Bad(w, "unknown action", 400)
		return
	}
	if input.Action == "save" {
		c := input.Connection
		c.ID = core.Token()
		if err := ValidateConnection(&c); err != nil {
			core.Bad(w, err.Error(), 400)
			return
		}
		if len(connections) >= 50 {
			core.Bad(w, "connection limit reached", 400)
			return
		}
		connections = append(connections, c)
		if a.SaveConnections(connections) != nil {
			core.Bad(w, "could not save connection", 500)
			return
		}
		a.Log(u.ID, "storage-connect", c.Name, u.IP)
		c.Options = nil
		core.JSON(w, c)
		return
	}
	var selected core.StorageConnection
	index := -1
	for i, c := range connections {
		if c.ID == input.ID {
			selected = c
			index = i
			break
		}
	}
	if index < 0 {
		core.Bad(w, "connection not found", 404)
		return
	}
	if input.Action == "attach" || input.Action == "detach" {
		a.invalidateListings(input.ID)
		connections[index].Attached = input.Action == "attach"
		if a.SaveConnections(connections) != nil {
			core.Bad(w, "could not update attachment", 500)
			return
		}
		a.Log(u.ID, "storage-"+input.Action, selected.Name, u.IP)
		core.JSON(w, map[string]bool{"ok": true})
		return
	}
	if input.Action == "delete" {
		a.invalidateListings(input.ID)
		connections = append(connections[:index], connections[index+1:]...)
		if a.SaveConnections(connections) != nil {
			core.Bad(w, "could not remove connection", 500)
			return
		}
		a.Log(u.ID, "storage-disconnect", selected.Name, u.IP)
		core.JSON(w, map[string]bool{"ok": true})
		return
	}
	if input.Action == "test" {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		f, options, err := NewProviderFS(ctx, selected)
		if err == nil {
			_, err = f.List(ctx, "")
		}
		if err != nil {
			core.Bad(w, "connection test failed; check credentials, host verification and remote directory", 400)
			return
		}
		if options["token"] != "" {
			connections[index].Options["token"] = options["token"]
			if a.SaveConnections(connections) != nil {
				core.Bad(w, "could not persist refreshed token", 500)
				return
			}
		}
		core.JSON(w, map[string]bool{"ok": true})
		return
	}
}

func (a *Module) StorageCipher() (cipher.AEAD, error) {
	var seq int
	var name, dbPath string
	if err := a.DB.QueryRow("PRAGMA database_list").Scan(&seq, &name, &dbPath); err != nil {
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

func (a *Module) EncryptConnections(plain []byte) ([]byte, error) {
	gcm, err := a.StorageCipher()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return json.Marshal(core.EncryptedConnections{Nonce: nonce, Data: gcm.Seal(nil, nonce, plain, []byte("trynas-storage-v1"))})
}

func (a *Module) DecryptConnections(raw []byte) ([]byte, error) {
	var encrypted core.EncryptedConnections
	if err := json.Unmarshal(raw, &encrypted); err != nil {
		return nil, err
	}
	gcm, err := a.StorageCipher()
	if err != nil {
		return nil, err
	}
	if len(encrypted.Nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid connection nonce")
	}
	return gcm.Open(nil, encrypted.Nonce, encrypted.Data, []byte("trynas-storage-v1"))
}

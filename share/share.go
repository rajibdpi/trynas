package share

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/rajibdpi/trynas/core"
	"golang.org/x/crypto/bcrypt"
)

func (a *Module) Shares(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	home := a.Home(u.ID)
	rows, _ := a.DB.Query("SELECT id,path,token,COALESCE(strftime('%Y-%m-%dT%H:%M:%fZ',expires_at),''),created_at,EXISTS(SELECT 1 FROM share_passwords p WHERE p.share_id=shares.id),COALESCE((SELECT json_group_array(path) FROM share_items WHERE share_id=shares.id),'[]'), COALESCE((SELECT storage FROM share_storage WHERE share_id=shares.id),''),COALESCE((SELECT visits FROM share_stats WHERE share_id=shares.id),0),COALESCE((SELECT downloads FROM share_stats WHERE share_id=shares.id),0) FROM shares WHERE user_id=? ORDER BY id DESC", u.ID)
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int
		var visits, downloads int64
		var p, tok, exp, created string
		var protected bool
		var pathsJSON, storage string
		_ = rows.Scan(&id, &p, &tok, &exp, &created, &protected, &pathsJSON, &storage, &visits, &downloads)
		var paths []string
		_ = json.Unmarshal([]byte(pathsJSON), &paths)
		if len(paths) == 0 {
			paths = []string{p}
		}
		if u.Role != "admin" {
			allowed := home != "" && storage == ""
			for _, item := range paths {
				if !core.Under(item, home) {
					allowed = false
					break
				}
			}
			if !allowed {
				continue
			}
		}
		out = append(out, map[string]any{"visits": visits, "downloads": downloads, "id": id, "path": p, "paths": paths, "token": tok, "expiresAt": exp, "createdAt": created, "passwordProtected": protected, "storage": storage})
	}
	core.JSON(w, out)
}

func (a *Module) CreateShare(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	var x struct {
		Storage   string
		Path      string
		Paths     []string
		Days      int
		ExpiresAt string
		Password  string
		Never     bool
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&x); err != nil {
		core.Bad(w, "invalid share settings", 400)
		return
	}
	paths := x.Paths
	if len(paths) == 0 {
		paths = []string{x.Path}
	}
	if len(paths) > 500 {
		core.Bad(w, "share at most 500 items", 400)
		return
	}
	if x.Storage != "" {
		if u.Role != "admin" {
			core.Bad(w, "administrator required", 403)
			return
		}
		if err := a.ValidateRemoteShare(r.Context(), x.Storage, paths); err != nil {
			core.Bad(w, "select existing attached-storage files or folders", 400)
			return
		}
	}
	var cleaned []string
	for _, path := range paths {
		p, err := core.CleanPath(path)
		if err != nil {
			core.Bad(w, "invalid share path", 400)
			return
		}
		if x.Storage == "" {
			if !a.Can(u, p, false) {
				core.Bad(w, "folder permission denied", 403)
				return
			}
			full, err := core.Safe(a.Root, p)
			if err != nil {
				core.Bad(w, "invalid path", 400)
				return
			}
			st, err := os.Stat(full)
			if err != nil || (!st.IsDir() && !st.Mode().IsRegular()) {
				core.Bad(w, "select existing files or folders", 400)
				return
			}
		}
		duplicate := false
		for _, existing := range cleaned {
			if existing == p {
				duplicate = true
				break
			}
		}
		if !duplicate {
			cleaned = append(cleaned, p)
		}
	}
	// A selected folder already includes its selected descendants.
	paths = nil
	for _, p := range cleaned {
		covered := false
		for _, other := range cleaned {
			if p != other && core.Under(p, other) {
				covered = true
				break
			}
		}
		if !covered {
			paths = append(paths, p)
		}
	}
	p := paths[0]
	var err error
	now := time.Now().UTC()
	var expires time.Time
	if x.Never {
		if x.ExpiresAt != "" || x.Days != 0 {
			core.Bad(w, "choose one expiry option", 400)
			return
		}
	} else if x.ExpiresAt != "" {
		expires, err = time.Parse(time.RFC3339, x.ExpiresAt)
		if err != nil || !expires.After(now) || expires.After(now.Add(365*24*time.Hour)) {
			core.Bad(w, "choose a future expiry within 365 days", 400)
			return
		}
	} else {
		if x.Days < 1 || x.Days > 365 {
			core.Bad(w, "share expiration must be 1–365 days", 400)
			return
		}
		expires = now.Add(time.Duration(x.Days) * 24 * time.Hour)
	}
	if x.Password != "" && (len(x.Password) < 8 || len(x.Password) > 72) {
		core.Bad(w, "share password must be 8–72 bytes", 400)
		return
	}
	existing, err := a.existingShare(u.ID, x.Storage, paths)
	if err != nil {
		core.Bad(w, "could not check existing shares", 500)
		return
	}
	if existing != nil {
		core.JSON(w, existing)
		return
	}
	var hash []byte
	if x.Password != "" {
		if len(x.Password) < 8 || len(x.Password) > 72 {
			core.Bad(w, "share password must be 8–72 bytes", 400)
			return
		}
		hash, err = bcrypt.GenerateFromPassword([]byte(x.Password), bcrypt.DefaultCost)
		if err != nil {
			core.Bad(w, "could not protect share", 500)
			return
		}
	}
	tok := core.Token()
	tx, err := a.DB.Begin()
	if err != nil {
		core.Bad(w, "could not create share", 500)
		return
	}
	defer tx.Rollback()
	var exp any
	expiryText := ""
	if !x.Never {
		exp = expires.UTC().Format("2006-01-02 15:04:05.999")
		expiryText = expires.UTC().Format(time.RFC3339)
	}
	res, err := tx.Exec("INSERT INTO shares(user_id,path,token,expires_at) VALUES(?,?,?,?)", u.ID, p, tok, exp)
	if err != nil {
		core.Bad(w, "could not create share", 500)
		return
	}
	id, _ := res.LastInsertId()
	if x.Storage != "" {
		if _, err = tx.Exec("INSERT INTO share_storage(share_id,storage) VALUES(?,?)", id, x.Storage); err != nil {
			core.Bad(w, "could not save share storage", 500)
			return
		}
	}
	for _, item := range paths {
		if _, err = tx.Exec("INSERT INTO share_items(share_id,path) VALUES(?,?)", id, item); err != nil {
			core.Bad(w, "could not save shared items", 500)
			return
		}
	}
	if len(hash) > 0 {
		if _, err = tx.Exec("INSERT INTO share_passwords(share_id,password_hash) VALUES(?,?)", id, string(hash)); err != nil {
			core.Bad(w, "could not protect share", 500)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		core.Bad(w, "could not create share", 500)
		return
	}
	a.Log(u.ID, "share", p, u.IP)
	core.JSON(w, map[string]any{"id": id, "path": p, "paths": paths, "token": tok, "expiresAt": expiryText, "passwordProtected": len(hash) > 0})
}

func (a *Module) DeleteShare(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		core.Bad(w, "invalid share", 400)
		return
	}
	tx, err := a.DB.Begin()
	if err != nil {
		core.Bad(w, "could not revoke share", 500)
		return
	}
	defer tx.Rollback()
	var owned int
	err = tx.QueryRow("SELECT 1 FROM shares WHERE id=? AND user_id=?", id, u.ID).Scan(&owned)
	if err == sql.ErrNoRows {
		core.Bad(w, "share not found", 404)
		return
	}
	if err != nil {
		core.Bad(w, "could not revoke share", 500)
		return
	}
	for _, table := range []string{"share_sessions", "share_passwords", "share_stats", "share_items", "share_storage"} {
		if _, err = tx.Exec("DELETE FROM "+table+" WHERE share_id=?", id); err != nil {
			core.Bad(w, "could not revoke share", 500)
			return
		}
	}
	if _, err = tx.Exec("DELETE FROM shares WHERE id=? AND user_id=?", id, u.ID); err != nil {
		core.Bad(w, "could not revoke share", 500)
		return
	}
	if err = tx.Commit(); err != nil {
		core.Bad(w, "could not revoke share", 500)
		return
	}
	core.JSON(w, map[string]bool{"ok": true})
}

func (a *Module) PublicShare(w http.ResponseWriter, r *http.Request) {
	tok := chi.URLParam(r, "token")
	var p, passwordHash string
	var shareID int64
	var owner core.User
	e := a.DB.QueryRow("SELECT s.id,s.path,s.user_id,COALESCE(p.password_hash,'') FROM shares s LEFT JOIN share_passwords p ON p.share_id=s.id WHERE s.token=? AND (s.expires_at IS NULL OR julianday(s.expires_at)>julianday('now'))", tok).Scan(&shareID, &p, &owner.ID, &passwordHash)
	var storage string
	storageErr := a.DB.QueryRow("SELECT storage FROM share_storage WHERE share_id=?", shareID).Scan(&storage)
	if storageErr != nil && storageErr != sql.ErrNoRows {
		core.Bad(w, "share unavailable", 500)
		return
	}
	if e == nil {
		e = a.DB.QueryRow("SELECT username,role FROM users WHERE id=?", owner.ID).Scan(&owner.Username, &owner.Role)
		if e == nil && ((storage == "" && !a.Can(owner, p, false)) || (storage != "" && owner.Role != "admin")) {
			e = fmt.Errorf("access revoked")
		}
	}
	if e != nil {
		core.Bad(w, "share not found or expired", 404)
		return
	}
	paths, err := a.SharePaths(shareID, p)
	if err != nil {
		core.Bad(w, "share unavailable", 500)
		return
	}
	if storage != "" {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "same-origin")
		if passwordHash != "" && !a.AuthorizeShare(w, r, shareID, tok, passwordHash) {
			return
		}
		tracked, record := a.trackShare(w, r, shareID)
		w = tracked
		defer record()
		if r.URL.Query().Get("qr") == "1" {
			ServeQR(w, r, tok)
			return
		}
		a.ServeRemoteShare(w, r, storage, paths)
		return
	}
	for _, item := range paths {
		if !a.Can(owner, item, false) {
			core.Bad(w, "share access revoked", 404)
			return
		}
		full, err := core.Safe(a.Root, item)
		if err != nil {
			core.Bad(w, "share unavailable", 404)
			return
		}
		if _, err = os.Stat(full); err != nil {
			core.Bad(w, "shared item not found", 404)
			return
		}
	}
	f, e := core.Safe(a.Root, p)
	if e != nil {
		core.Bad(w, "invalid", 400)
		return
	}
	st, e := os.Stat(f)
	if e != nil {
		core.Bad(w, "not found", 404)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "same-origin")
	if passwordHash != "" && !a.AuthorizeShare(w, r, shareID, tok, passwordHash) {
		return
	}
	tracked, record := a.trackShare(w, r, shareID)
	w = tracked
	defer record()
	if r.URL.Query().Get("qr") == "1" {
		ServeQR(w, r, tok)
		return
	}
	if r.URL.Query().Get("download") != "1" {
		items, truncated, err := a.ListSharedItems(owner, paths)
		if err != nil {
			core.Bad(w, "could not list shared items", 500)
			return
		}
		if len(paths) > 1 {
			ServeSharePage(w, r, fmt.Sprintf("%d shared items", len(paths)), 0, true, items, truncated, true)
		} else {
			ServeSharePage(w, r, filepath.Base(f), st.Size(), st.IsDir(), items, truncated)
		}
		return
	}
	if _, present := r.URL.Query()["item"]; present {
		item, err := core.CleanPath(r.URL.Query().Get("item"))
		if err != nil {
			core.Bad(w, "invalid item", 400)
			return
		}
		included := false
		for _, root := range paths {
			if core.Under(item, root) {
				included = true
				break
			}
		}
		if !included || !a.Can(owner, item, false) {
			core.Bad(w, "item not shared", 404)
			return
		}
		full, err := core.Safe(a.Root, item)
		if err != nil {
			core.Bad(w, "item not shared", 404)
			return
		}
		info, err := os.Stat(full)
		if err != nil {
			core.Bad(w, "item not found", 404)
			return
		}
		if info.IsDir() {
			a.DownloadSharedFolder(w, r, owner, item, full)
			return
		}
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(full)}))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, full)
		return
	}
	if len(paths) > 1 {
		a.DownloadSharedItems(w, r, owner, paths, "shared-items.zip")
		return
	}
	if st.IsDir() {
		a.DownloadSharedFolder(w, r, owner, p, f)
		return
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(f)}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, f)
}

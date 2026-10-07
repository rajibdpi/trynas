package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	db          *sql.DB
	root, trash string
	mutation    sync.Mutex
	metrics     sync.Mutex
	networkLast map[string]networkSample
	smartCached map[string]any
	smartAt     time.Time
}
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}
type ctxKey string

const userKey ctxKey = "user"

func main() {
	if err := loadConfig(); err != nil {
		log.Fatal(err)
	}
	dbPath := env("DB_PATH", "./data/trynas.db")
	root := env("STORAGE_ROOT", "./storage")
	var pathErr error
	root, pathErr = filepath.Abs(root)
	if pathErr != nil {
		log.Fatal(pathErr)
	}
	dbPath, pathErr = filepath.Abs(dbPath)
	if pathErr != nil {
		log.Fatal(pathErr)
	}
	trash := filepath.Join(root, ".trynas-trash")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(trash, 0700); err != nil {
		log.Fatal(err)
	}
	db, err := openDatabase(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	a := &App{db: db, root: root, trash: trash}
	if err = a.migrate(); err != nil {
		log.Fatal(err)
	}
	if err = a.migrateV5(); err != nil {
		log.Fatal(err)
	}
	a.monitorNetwork()
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(a.cors)
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, map[string]any{"ok": true, "name": "TryNAS", "version": "5.0.0"})
	})
	r.Post("/api/auth/login", a.login)
	r.Post("/api/auth/logout", a.logout)
	r.Group(func(r chi.Router) {
		r.Use(a.auth)
		r.Use(a.guard)
		r.Get("/api/users", a.users)
		r.Post("/api/users", a.saveUser)
		r.Delete("/api/users/{id}", a.deleteUser)
		r.Get("/api/permissions", a.permissions)
		r.Post("/api/permissions", a.savePermission)
		r.Get("/api/quota", a.quota)
		r.Get("/api/hardware", a.hardware)
		r.Get("/api/network", a.network)
		r.Post("/api/files/bulk", a.bulk)
		r.Get("/api/me", a.me)
		r.Get("/api/system", a.system)
		r.Get("/api/activity", a.activity)
		r.Get("/api/files", a.listFiles)
		r.Get("/api/files/search", a.searchFiles)
		r.Get("/api/files/download", a.download)
		r.Get("/api/files/preview", a.preview)
		r.Post("/api/files/folder", a.mkdir)
		r.Post("/api/files/upload", a.uploadV5)
		r.Post("/api/files/rename", a.renameV5)
		r.Post("/api/files/copy", a.copyMoveV5(false))
		r.Post("/api/files/move", a.copyMoveV5(true))
		r.Post("/api/files/zip", a.zipV5)
		r.Delete("/api/files", a.trashV5)
		r.Get("/api/favorites", a.favorites)
		r.Post("/api/favorites", a.toggleFavorite)
		r.Get("/api/trash", a.listTrash)
		r.Post("/api/trash/restore", a.restoreV5)
		r.Delete("/api/trash", a.purgeV5)
		r.Get("/api/shares", a.shares)
		r.Post("/api/shares", a.createShare)
		r.Delete("/api/shares/{id}", a.deleteShare)
	})
	r.Get("/s/{token}", a.publicShare)
	r.Post("/s/{token}", a.publicShare)
	r.Handle("/*", embeddedFrontend())
	addr := env("ADDR", ":8080")
	log.Printf("TryNAS v5 API %s storage=%s", addr, root)
	log.Fatal(http.ListenAndServe(addr, r))
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func (a *App) migrate() error {
	q := `CREATE TABLE IF NOT EXISTS users(id INTEGER PRIMARY KEY,username TEXT UNIQUE NOT NULL,password_hash TEXT NOT NULL,role TEXT NOT NULL DEFAULT 'user',created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
 CREATE TABLE IF NOT EXISTS sessions(token TEXT PRIMARY KEY,user_id INTEGER NOT NULL,expires_at DATETIME NOT NULL,created_at DATETIME DEFAULT CURRENT_TIMESTAMP,FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
 CREATE TABLE IF NOT EXISTS audit_logs(id INTEGER PRIMARY KEY,user_id INTEGER,action TEXT NOT NULL,path TEXT,created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
 CREATE TABLE IF NOT EXISTS favorites(id INTEGER PRIMARY KEY,user_id INTEGER NOT NULL,path TEXT NOT NULL,created_at DATETIME DEFAULT CURRENT_TIMESTAMP,UNIQUE(user_id,path),FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
 CREATE TABLE IF NOT EXISTS trash(id INTEGER PRIMARY KEY,user_id INTEGER NOT NULL,original_path TEXT NOT NULL,trash_name TEXT NOT NULL,deleted_at DATETIME DEFAULT CURRENT_TIMESTAMP,FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
 CREATE TABLE IF NOT EXISTS shares(id INTEGER PRIMARY KEY,user_id INTEGER NOT NULL,path TEXT NOT NULL,token TEXT UNIQUE NOT NULL,expires_at DATETIME,created_at DATETIME DEFAULT CURRENT_TIMESTAMP,FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
 CREATE INDEX IF NOT EXISTS idx_sessions_exp ON sessions(expires_at); CREATE INDEX IF NOT EXISTS idx_fav_user ON favorites(user_id); CREATE INDEX IF NOT EXISTS idx_share_token ON shares(token);`
	if _, e := a.db.Exec(q); e != nil {
		return e
	}
	var n int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		h, err := bcrypt.GenerateFromPassword([]byte(env("ADMIN_PASSWORD", "admin123!")), 12)
		if err != nil {
			return err
		}
		_, e := a.db.Exec("INSERT INTO users(username,password_hash,role) VALUES(?,?,?)", env("ADMIN_USER", "admin"), string(h), "admin")
		return e
	}
	return nil
}
func (a *App) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o := env("CORS_ORIGIN", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Origin", o)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func bad(w http.ResponseWriter, s string, c int) { http.Error(w, s, c) }
func token() string                              { b := make([]byte, 24); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var x struct{ Username, Password string }
	if json.NewDecoder(r.Body).Decode(&x) != nil {
		bad(w, "bad request", 400)
		return
	}
	var u User
	var h string
	err := a.db.QueryRow("SELECT id,username,role,password_hash FROM users WHERE username=?", x.Username).Scan(&u.ID, &u.Username, &u.Role, &h)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(h), []byte(x.Password)) != nil {
		bad(w, "invalid credentials", 401)
		return
	}
	t := token()
	exp := time.Now().Add(7 * 24 * time.Hour)
	_, _ = a.db.Exec("INSERT INTO sessions(token,user_id,expires_at) VALUES(?,?,?)", t, u.ID, exp)
	http.SetCookie(w, &http.Cookie{Name: "trynas_session", Value: t, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: exp, Secure: env("COOKIE_SECURE", "") == "true"})
	a.log(u.ID, "login", "")
	jsonOut(w, a.profile(u))
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("trynas_session"); e == nil {
		_, _ = a.db.Exec("DELETE FROM sessions WHERE token=?", c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "trynas_session", Path: "/", MaxAge: -1, HttpOnly: true})
	jsonOut(w, map[string]bool{"ok": true})
}
func (a *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie("trynas_session")
		if e != nil {
			bad(w, "unauthorized", 401)
			return
		}
		var u User
		e = a.db.QueryRow(`SELECT u.id,u.username,u.role FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token=? AND s.expires_at>CURRENT_TIMESTAMP`, c.Value).Scan(&u.ID, &u.Username, &u.Role)
		if e != nil {
			bad(w, "unauthorized", 401)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}
func (a *App) me(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, a.profile(r.Context().Value(userKey).(User)))
}
func safe(root, p string) (string, error) {
	rel, err := cleanPath(p)
	if err != nil {
		return "", err
	}
	rr, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	ff := filepath.Join(rr, filepath.FromSlash(rel))
	// Reject symlinks in every component, including an existing destination.
	current := rr
	for _, part := range strings.Split(rel, "/") {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		st, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil && (runtime.GOOS == "windows" || runtime.GOOS == "darwin") {
			entries, e := os.ReadDir(filepath.Dir(current))
			if e != nil {
				return "", e
			}
			exact := false
			for _, entry := range entries {
				if entry.Name() == part {
					exact = true
					break
				}
			}
			if !exact {
				return "", fmt.Errorf("use the exact folder and file names shown in the browser")
			}
		}
		if err == nil && st.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symbolic links are not supported")
		}
	}
	return ff, nil
}
func relJoin(parts ...string) string {
	return strings.TrimPrefix(filepath.ToSlash(filepath.Join(parts...)), "/")
}
func (a *App) listFiles(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	f, e := safe(a.root, p)
	if e != nil {
		bad(w, "invalid path", 400)
		return
	}
	es, e := os.ReadDir(f)
	if e != nil {
		bad(w, e.Error(), 404)
		return
	}
	u := r.Context().Value(userKey).(User)
	fav := a.favoriteSet(u.ID)
	out := []map[string]any{}
	for _, x := range es {
		if x.Type()&os.ModeSymlink != 0 || strings.HasPrefix(x.Name(), ".trynas-") {
			continue
		}
		info, ie := x.Info()
		if ie != nil {
			continue
		}
		rp := relJoin(p, x.Name())
		if !a.can(u, rp, false) && !(x.IsDir() && a.canBrowse(u, rp)) {
			continue
		}
		out = append(out, map[string]any{"name": x.Name(), "path": rp, "dir": x.IsDir(), "size": info.Size(), "modified": info.ModTime(), "favorite": fav[rp], "writable": a.can(u, rp, true)})
	}
	sort.Slice(out, func(i, j int) bool {
		di := out[i]["dir"].(bool)
		dj := out[j]["dir"].(bool)
		if di != dj {
			return di
		}
		return strings.ToLower(out[i]["name"].(string)) < strings.ToLower(out[j]["name"].(string))
	})
	jsonOut(w, map[string]any{"path": p, "items": out, "writable": a.can(u, p, true)})
}
func (a *App) searchFiles(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if q == "" {
		jsonOut(w, []any{})
		return
	}
	base := r.URL.Query().Get("path")
	start, e := safe(a.root, base)
	if e != nil {
		bad(w, "invalid path", 400)
		return
	}
	out := []map[string]any{}
	_ = filepath.Walk(start, func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return nil
		}
		relPath, _ := filepath.Rel(a.root, p)
		if i.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if strings.HasPrefix(i.Name(), ".trynas-") {
			return filepath.SkipDir
		}
		user := r.Context().Value(userKey).(User)
		if !a.can(user, filepath.ToSlash(relPath), false) {
			if i.IsDir() && !a.canBrowse(user, filepath.ToSlash(relPath)) {
				return filepath.SkipDir
			}
			return nil
		}
		if p == a.trash {
			return filepath.SkipDir
		}
		if p == start {
			return nil
		}
		if strings.Contains(strings.ToLower(i.Name()), q) {
			rel, _ := filepath.Rel(a.root, p)
			out = append(out, map[string]any{"name": i.Name(), "path": filepath.ToSlash(rel), "dir": i.IsDir(), "size": i.Size(), "modified": i.ModTime(), "writable": a.can(user, filepath.ToSlash(rel), true)})
			if len(out) >= 100 {
				return filepath.SkipAll
			}
		}
		return nil
	})
	jsonOut(w, out)
}
func (a *App) mkdir(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	var x struct{ Path string }
	_ = json.NewDecoder(r.Body).Decode(&x)
	f, e := safe(a.root, x.Path)
	if e != nil {
		bad(w, "invalid path", 400)
		return
	}
	if e = os.MkdirAll(f, 0755); e != nil {
		bad(w, e.Error(), 500)
		return
	}
	a.log(u.ID, "mkdir", x.Path)
	jsonOut(w, map[string]bool{"ok": true})
}
func (a *App) download(w http.ResponseWriter, r *http.Request) {
	f, e := safe(a.root, r.URL.Query().Get("path"))
	if e != nil {
		bad(w, "invalid", 400)
		return
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(f)}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, f)
}
func (a *App) preview(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	f, e := safe(a.root, p)
	if e != nil {
		bad(w, "invalid", 400)
		return
	}
	st, e := os.Stat(f)
	if e != nil || st.IsDir() {
		bad(w, "not previewable", 400)
		return
	}
	ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(f)))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", "inline; filename=\""+strings.ReplaceAll(filepath.Base(f), "\"", "")+"\"")
	http.ServeFile(w, r, f)
}
func copyTree(src, dst string) error {
	st, e := os.Stat(src)
	if e != nil {
		return e
	}
	if st.IsDir() {
		if e = os.MkdirAll(dst, st.Mode()); e != nil {
			return e
		}
		es, e := os.ReadDir(src)
		if e != nil {
			return e
		}
		for _, x := range es {
			if e = copyTree(filepath.Join(src, x.Name()), filepath.Join(dst, x.Name())); e != nil {
				return e
			}
		}
		return nil
	}
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, st.Mode())
	if e != nil {
		return e
	}
	defer out.Close()
	_, e = io.Copy(out, in)
	return e
}
func (a *App) listTrash(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	rows, e := a.db.Query("SELECT id,original_path,deleted_at FROM trash WHERE user_id=? OR ?='admin' ORDER BY id DESC", u.ID, u.Role)
	if e != nil {
		bad(w, e.Error(), 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int
		var p, t string
		_ = rows.Scan(&id, &p, &t)
		out = append(out, map[string]any{"id": id, "path": p, "name": filepath.Base(p), "deletedAt": t})
	}
	jsonOut(w, out)
}
func (a *App) favoriteSet(uid int) map[string]bool {
	m := map[string]bool{}
	rows, e := a.db.Query("SELECT path FROM favorites WHERE user_id=?", uid)
	if e != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		_ = rows.Scan(&p)
		m[p] = true
	}
	return m
}
func (a *App) favorites(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	rows, _ := a.db.Query("SELECT id,path,created_at FROM favorites WHERE user_id=? ORDER BY id DESC", u.ID)
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int
		var p, t string
		_ = rows.Scan(&id, &p, &t)
		if a.can(u, p, false) {
			out = append(out, map[string]any{"id": id, "path": p, "name": filepath.Base(p), "createdAt": t})
		}
	}
	jsonOut(w, out)
}
func (a *App) toggleFavorite(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	var x struct{ Path string }
	_ = json.NewDecoder(r.Body).Decode(&x)
	var id int
	e := a.db.QueryRow("SELECT id FROM favorites WHERE user_id=? AND path=?", u.ID, x.Path).Scan(&id)
	fav := false
	if e == nil {
		_, _ = a.db.Exec("DELETE FROM favorites WHERE id=?", id)
	} else {
		_, _ = a.db.Exec("INSERT OR IGNORE INTO favorites(user_id,path) VALUES(?,?)", u.ID, x.Path)
		fav = true
	}
	jsonOut(w, map[string]bool{"favorite": fav})
}
func (a *App) shares(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	rows, _ := a.db.Query("SELECT id,path,token,COALESCE(strftime('%Y-%m-%dT%H:%M:%fZ',expires_at),''),created_at,EXISTS(SELECT 1 FROM share_passwords p WHERE p.share_id=shares.id),COALESCE((SELECT json_group_array(path) FROM share_items WHERE share_id=shares.id),'[]') FROM shares WHERE user_id=? ORDER BY id DESC", u.ID)
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int
		var p, tok, exp, created string
		var protected bool
		var pathsJSON string
		_ = rows.Scan(&id, &p, &tok, &exp, &created, &protected, &pathsJSON)
		var paths []string
		_ = json.Unmarshal([]byte(pathsJSON), &paths)
		if len(paths) == 0 {
			paths = []string{p}
		}
		out = append(out, map[string]any{"id": id, "path": p, "paths": paths, "token": tok, "expiresAt": exp, "createdAt": created, "passwordProtected": protected})
	}
	jsonOut(w, out)
}
func (a *App) createShare(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	var x struct {
		Path      string
		Paths     []string
		Days      int
		ExpiresAt string
		Password  string
		Never     bool
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&x); err != nil {
		bad(w, "invalid share settings", 400)
		return
	}
	paths := x.Paths
	if len(paths) == 0 {
		paths = []string{x.Path}
	}
	if len(paths) > 500 {
		bad(w, "share at most 500 items", 400)
		return
	}
	var cleaned []string
	for _, path := range paths {
		p, err := cleanPath(path)
		if err != nil {
			bad(w, "invalid share path", 400)
			return
		}
		if !a.can(u, p, false) {
			bad(w, "folder permission denied", 403)
			return
		}
		full, err := safe(a.root, p)
		if err != nil {
			bad(w, "invalid path", 400)
			return
		}
		st, err := os.Stat(full)
		if err != nil || (!st.IsDir() && !st.Mode().IsRegular()) {
			bad(w, "select existing files or folders", 400)
			return
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
			if p != other && under(p, other) {
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
			bad(w, "choose one expiry option", 400)
			return
		}
	} else if x.ExpiresAt != "" {
		expires, err = time.Parse(time.RFC3339, x.ExpiresAt)
		if err != nil || !expires.After(now) || expires.After(now.Add(365*24*time.Hour)) {
			bad(w, "choose a future expiry within 365 days", 400)
			return
		}
	} else {
		if x.Days < 1 || x.Days > 365 {
			bad(w, "share expiration must be 1–365 days", 400)
			return
		}
		expires = now.Add(time.Duration(x.Days) * 24 * time.Hour)
	}
	var hash []byte
	if x.Password != "" {
		if len(x.Password) < 8 || len(x.Password) > 72 {
			bad(w, "share password must be 8–72 bytes", 400)
			return
		}
		hash, err = bcrypt.GenerateFromPassword([]byte(x.Password), bcrypt.DefaultCost)
		if err != nil {
			bad(w, "could not protect share", 500)
			return
		}
	}
	tx, err := a.db.Begin()
	if err != nil {
		bad(w, "could not create share", 500)
		return
	}
	defer tx.Rollback()
	tok := token()
	var exp any
	expiryText := ""
	if !x.Never {
		exp = expires.UTC().Format("2006-01-02 15:04:05.999")
		expiryText = expires.UTC().Format(time.RFC3339)
	}
	res, err := tx.Exec("INSERT INTO shares(user_id,path,token,expires_at) VALUES(?,?,?,?)", u.ID, p, tok, exp)
	if err != nil {
		bad(w, "could not create share", 500)
		return
	}
	id, _ := res.LastInsertId()
	for _, item := range paths {
		if _, err = tx.Exec("INSERT INTO share_items(share_id,path) VALUES(?,?)", id, item); err != nil {
			bad(w, "could not save shared items", 500)
			return
		}
	}
	if len(hash) > 0 {
		if _, err = tx.Exec("INSERT INTO share_passwords(share_id,password_hash) VALUES(?,?)", id, string(hash)); err != nil {
			bad(w, "could not protect share", 500)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		bad(w, "could not create share", 500)
		return
	}
	a.log(u.ID, "share", p)
	jsonOut(w, map[string]any{"id": id, "path": p, "paths": paths, "token": tok, "expiresAt": expiryText, "passwordProtected": len(hash) > 0})
}
func (a *App) deleteShare(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	_, _ = a.db.Exec("DELETE FROM shares WHERE id=? AND user_id=?", chi.URLParam(r, "id"), u.ID)
	jsonOut(w, map[string]bool{"ok": true})
}
func (a *App) publicShare(w http.ResponseWriter, r *http.Request) {
	tok := chi.URLParam(r, "token")
	var p, passwordHash string
	var shareID int64
	var owner User
	e := a.db.QueryRow("SELECT s.id,s.path,s.user_id,COALESCE(p.password_hash,'') FROM shares s LEFT JOIN share_passwords p ON p.share_id=s.id WHERE s.token=? AND (s.expires_at IS NULL OR julianday(s.expires_at)>julianday('now'))", tok).Scan(&shareID, &p, &owner.ID, &passwordHash)
	if e == nil {
		e = a.db.QueryRow("SELECT username,role FROM users WHERE id=?", owner.ID).Scan(&owner.Username, &owner.Role)
		if e == nil && !a.can(owner, p, false) {
			e = fmt.Errorf("access revoked")
		}
	}
	if e != nil {
		bad(w, "share not found or expired", 404)
		return
	}
	paths, err := a.sharePaths(shareID, p)
	if err != nil {
		bad(w, "share unavailable", 500)
		return
	}
	for _, item := range paths {
		if !a.can(owner, item, false) {
			bad(w, "share access revoked", 404)
			return
		}
		full, err := safe(a.root, item)
		if err != nil {
			bad(w, "share unavailable", 404)
			return
		}
		if _, err = os.Stat(full); err != nil {
			bad(w, "shared item not found", 404)
			return
		}
	}
	f, e := safe(a.root, p)
	if e != nil {
		bad(w, "invalid", 400)
		return
	}
	st, e := os.Stat(f)
	if e != nil {
		bad(w, "not found", 404)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if passwordHash != "" && !a.authorizeShare(w, r, shareID, tok, passwordHash) {
		return
	}
	if r.URL.Query().Get("qr") == "1" {
		serveShareQR(w, r, tok)
		return
	}
	if r.URL.Query().Get("download") != "1" {
		items, truncated, err := a.listSharedItems(owner, paths)
		if err != nil {
			bad(w, "could not list shared items", 500)
			return
		}
		if len(paths) > 1 {
			serveSharePage(w, r, fmt.Sprintf("%d shared items", len(paths)), 0, true, items, truncated, true)
		} else {
			serveSharePage(w, r, filepath.Base(f), st.Size(), st.IsDir(), items, truncated)
		}
		return
	}
	if _, present := r.URL.Query()["item"]; present {
		item, err := cleanPath(r.URL.Query().Get("item"))
		if err != nil {
			bad(w, "invalid item", 400)
			return
		}
		included := false
		for _, root := range paths {
			if under(item, root) {
				included = true
				break
			}
		}
		if !included || !a.can(owner, item, false) {
			bad(w, "item not shared", 404)
			return
		}
		full, err := safe(a.root, item)
		if err != nil {
			bad(w, "item not shared", 404)
			return
		}
		info, err := os.Stat(full)
		if err != nil {
			bad(w, "item not found", 404)
			return
		}
		if info.IsDir() {
			a.downloadSharedFolder(w, r, owner, item, full)
			return
		}
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(full)}))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, full)
		return
	}
	if len(paths) > 1 {
		a.downloadSharedItems(w, r, owner, paths, "shared-items.zip")
		return
	}
	if st.IsDir() {
		a.downloadSharedFolder(w, r, owner, p, f)
		return
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(f)}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, f)
}
func (a *App) system(w http.ResponseWriter, r *http.Request) {
	v, _ := mem.VirtualMemory()
	c, _ := cpu.Percent(200*time.Millisecond, false)
	h, _ := host.Info()
	du, _ := disk.Usage(a.root)
	pct := 0.0
	if len(c) > 0 {
		pct = c[0]
	}
	jsonOut(w, map[string]any{"cpu": pct, "memoryPercent": v.UsedPercent, "memoryUsed": v.Used, "memoryTotal": v.Total, "uptime": h.Uptime, "hostname": h.Hostname, "os": h.Platform, "kernel": h.KernelVersion, "storage": map[string]any{"total": du.Total, "used": du.Used, "free": du.Free, "percent": du.UsedPercent, "path": a.root}})
}
func (a *App) activity(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(userKey).(User)
	rows, _ := a.db.Query(`SELECT a.action,COALESCE(a.path,''),a.created_at,COALESCE(u.username,'system') FROM audit_logs a LEFT JOIN users u ON u.id=a.user_id WHERE (?='admin' OR a.user_id=?) ORDER BY a.id DESC LIMIT 50`, u.Role, u.ID)
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var ac, p, t, u string
		_ = rows.Scan(&ac, &p, &t, &u)
		out = append(out, map[string]any{"action": ac, "path": p, "time": t, "user": u})
	}
	jsonOut(w, out)
}
func (a *App) log(uid int, act, p string) {
	_, _ = a.db.Exec("INSERT INTO audit_logs(user_id,action,path) VALUES(?,?,?)", uid, act, p)
}

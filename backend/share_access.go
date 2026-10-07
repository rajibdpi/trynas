package main

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Older releases stored time.Time.String(), including zone and monotonic text.
func (a *App) normalizeShareExpiries() error {
	if err := a.normalizeExpiries("shares", "id"); err != nil {
		return err
	}
	return a.normalizeExpiries("sessions", "token")
}
func (a *App) normalizeExpiries(table, key string) error {
	rows, err := a.db.Query(fmt.Sprintf("SELECT %s,CAST(expires_at AS TEXT) FROM %s WHERE expires_at IS NOT NULL AND julianday(expires_at) IS NULL", key, table))
	if err != nil {
		return err
	}
	type correction struct {
		id     string
		expiry string
	}
	var updates []correction
	for rows.Next() {
		var id string
		var text string
		if err = rows.Scan(&id, &text); err != nil {
			rows.Close()
			return err
		}
		fields := strings.Fields(text)
		if len(fields) < 3 {
			continue
		}
		parsed, parseErr := time.Parse("2006-01-02 15:04:05.999999999 -0700", strings.Join(fields[:3], " "))
		if parseErr == nil {
			updates = append(updates, correction{id, parsed.UTC().Format("2006-01-02 15:04:05.999")})
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, update := range updates {
		if _, err = a.db.Exec(fmt.Sprintf("UPDATE %s SET expires_at=? WHERE %s=?", table, key), update.expiry, update.id); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) authorizeShare(w http.ResponseWriter, r *http.Request, id int64, tok, hash string) bool {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if cookie, err := r.Cookie("trynas_share"); err == nil {
		var valid int
		if a.db.QueryRow("SELECT 1 FROM share_sessions WHERE token=? AND share_id=? AND julianday(expires_at)>julianday('now')", cookie.Value, id).Scan(&valid) == nil {
			return true
		}
	}
	errorText := ""
	if r.Method == http.MethodPost {
		if !a.allowAttempt(w, r, "share:"+tok) {
			return false
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			bad(w, "invalid password form", 400)
			return false
		}
		password := r.PostForm.Get("password")
		if len(password) > 72 || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
			errorText = "Incorrect password. Please try again."
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			session := token()
			expires := time.Now().UTC().Add(time.Hour)
			_, _ = a.db.Exec("DELETE FROM share_sessions WHERE julianday(expires_at)<=julianday('now')")
			if _, err := a.db.Exec("INSERT INTO share_sessions(token,share_id,expires_at) VALUES(?,?,?)", session, id, expires.Format(time.RFC3339)); err != nil {
				bad(w, "could not unlock share", 500)
				return false
			}
			http.SetCookie(w, &http.Cookie{Name: "trynas_share", Value: session, Path: "/s/" + tok, HttpOnly: true, Secure: r.TLS != nil || env("COOKIE_SECURE", "false") == "true", SameSite: http.SameSiteLaxMode, MaxAge: 3600})
			http.Redirect(w, r, "/s/"+tok, http.StatusSeeOther)
			return false
		}
	} else if r.URL.Query().Get("download") == "1" || r.URL.Query().Get("qr") == "1" {
		bad(w, "share password required", 401)
		return false
	}
	_ = sharePasswordPage.Execute(w, errorText)
	return false
}

var sharePasswordPage = template.Must(template.New("password").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Protected share · TryNAS</title><style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#f4f6fa;font:15px/1.5 system-ui;color:#192338}main{margin:20px;padding:32px;border-radius:20px;background:white;width:min(380px,calc(100% - 100px))}h1{font-size:24px}label,input,button{display:block}input{box-sizing:border-box;width:100%;padding:12px;margin:8px 0 16px;border:1px solid #cbd5e1;border-radius:8px}button{padding:12px 20px;border:0;border-radius:8px;background:#6254e8;color:white;cursor:pointer}p{color:#64748b}.error{color:#dc2626}</style></head><body><main><b>TryNAS</b><h1>Password-protected share</h1><p>Enter the share password to access this file or folder.</p>{{if .}}<p class="error" role="alert">{{.}}</p>{{end}}<form method="post" action=""><label for="password">Share password</label><input id="password" name="password" type="password" autocomplete="current-password" required maxlength="72"><button type="submit">Unlock share</button></form></main></body></html>`))

func (a *App) downloadSharedFolder(w http.ResponseWriter, r *http.Request, owner User, p, full string) {
	a.downloadSharedItems(w, r, owner, []string{p}, filepath.Base(full)+".zip")
}

func (a *App) sharePaths(id int64, fallback string) ([]string, error) {
	rows, err := a.db.Query("SELECT path FROM share_items WHERE share_id=? ORDER BY path", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err = rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		paths = []string{fallback}
	}
	return paths, rows.Err()
}

func (a *App) downloadSharedItems(w http.ResponseWriter, r *http.Request, owner User, paths []string, name string) {
	a.archives.Lock()
	if a.activeArchives >= 2 {
		a.archives.Unlock()
		bad(w, "too many archive downloads; retry later", 429)
		return
	}
	a.activeArchives++
	a.archives.Unlock()
	defer func() { a.archives.Lock(); a.activeArchives--; a.archives.Unlock() }()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	r = r.WithContext(ctx)
	a.mutation.Lock()
	locked := true
	defer func() {
		if locked {
			a.mutation.Unlock()
		}
	}()
	temp, err := os.CreateTemp("", "trynas-share-*.zip")
	if err != nil {
		bad(w, "could not create folder download", 500)
		return
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	archive := zip.NewWriter(&archiveWriter{target: temp, ctx: ctx})
	var inputBytes int64
	entries := 0
	seen := map[string]bool{}
	for _, p := range paths {
		full, resolveErr := safe(a.root, p)
		if resolveErr != nil {
			err = resolveErr
			break
		}
		err = filepath.Walk(full, func(f string, st os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := r.Context().Err(); err != nil {
				return err
			}
			suffix, err := filepath.Rel(full, f)
			if err != nil {
				return err
			}
			relative := p
			if suffix != "." {
				relative = relJoin(p, suffix)
			}
			if _, err := cleanPath(relative); err != nil {
				if st.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if st.Mode()&os.ModeSymlink != 0 {
				return nil
			}
			if st.IsDir() && !a.canBrowse(owner, relative) {
				return filepath.SkipDir
			}
			if !a.can(owner, relative, false) {
				return nil
			}
			if !st.IsDir() && !st.Mode().IsRegular() {
				return nil
			}
			checked, err := safe(a.root, relative)
			if err != nil {
				return err
			}
			header, err := zip.FileInfoHeader(st)
			if err != nil {
				return err
			}
			base := filepath.Base(full)
			if p == "" {
				base = "storage"
			}
			if len(paths) > 1 && p != "" {
				base = filepath.ToSlash(p)
			}
			header.Name = base
			if suffix != "." {
				header.Name += "/" + filepath.ToSlash(suffix)
			}
			if st.IsDir() {
				header.Name += "/"
			} else {
				header.Method = zip.Deflate
			}
			if seen[header.Name] {
				return nil
			}
			entries++
			if entries > maxSharedArchiveEntries {
				return fmt.Errorf("%w: too many entries", errArchiveLimit)
			}
			if !st.IsDir() {
				if st.Size() > maxSharedArchiveBytes-inputBytes {
					return fmt.Errorf("%w: files exceed 1 GiB", errArchiveLimit)
				}
				inputBytes += st.Size()
			}
			seen[header.Name] = true
			target, err := archive.CreateHeader(header)
			if err != nil || st.IsDir() {
				return err
			}
			file, err := os.Open(checked)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(target, io.LimitReader(file, st.Size()))
			return err
		})
		if err != nil {
			break
		}
	}
	closeErr := archive.Close()
	if err != nil || closeErr != nil {
		if errors.Is(err, errArchiveLimit) || errors.Is(closeErr, errArchiveLimit) {
			bad(w, "shared ZIP exceeds the 1 GiB or 10,000-entry limit", 413)
			return
		}
		bad(w, "could not archive shared folder", 500)
		return
	}
	if _, err = temp.Seek(0, io.SeekStart); err != nil {
		bad(w, "could not read folder download", 500)
		return
	}
	st, err := temp.Stat()
	if err != nil {
		bad(w, "could not read folder download", 500)
		return
	}
	a.mutation.Unlock()
	locked = false
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, name, st.ModTime(), temp)
}

package share

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rajibdpi/trynas/core"
	"golang.org/x/crypto/bcrypt"
)

func (a *Module) NormalizeShareExpiries() error {
	if err := a.NormalizeExpiries("shares", "id"); err != nil {
		return err
	}
	return a.NormalizeExpiries("sessions", "token")
}

func (a *Module) NormalizeExpiries(table, key string) error {
	rows, err := a.DB.Query(fmt.Sprintf("SELECT %s,CAST(expires_at AS TEXT) FROM %s WHERE expires_at IS NOT NULL AND julianday(expires_at) IS NULL", key, table))
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
		if _, err = a.DB.Exec(fmt.Sprintf("UPDATE %s SET expires_at=? WHERE %s=?", table, key), update.expiry, update.id); err != nil {
			return err
		}
	}
	return nil
}

func (a *Module) AuthorizeShare(w http.ResponseWriter, r *http.Request, id int64, tok, hash string) bool {
	w.Header().Set("Cache-Control", "no-store")
	resource := r.URL.Query().Get("download") == "1" || r.URL.Query().Get("qr") == "1"
	if cookie, err := r.Cookie("trynas_share"); err == nil {
		// A grant supports downloads/QR during the current visit, never a new page
		// visit or password submission. Reopening the page revokes this browser's
		// grant without affecting other visitors to the same share.
		if resource && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			var valid int
			if a.DB.QueryRow("SELECT 1 FROM share_sessions WHERE token=? AND share_id=? AND julianday(expires_at)>julianday('now')", cookie.Value, id).Scan(&valid) == nil {
				return true
			}
		} else {
			if _, err := a.DB.Exec("DELETE FROM share_sessions WHERE token=? AND share_id=?", cookie.Value, id); err != nil {
				core.Bad(w, "could not relock share", 500)
				return false
			}
			http.SetCookie(w, &http.Cookie{Name: "trynas_share", Path: "/s/" + tok, HttpOnly: true, Secure: core.SecureCookie(r), SameSite: http.SameSiteLaxMode, MaxAge: -1})
		}
	}
	errorText := ""
	if r.Method == http.MethodPost {
		if !a.AllowAttempt(w, r, "share:"+tok) {
			return false
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			core.Bad(w, "invalid password form", 400)
			return false
		}
		password := r.PostForm.Get("password")
		if len(password) > 72 || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			errorText = "Incorrect password. Please try again."
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			session := core.Token()
			expires := time.Now().UTC().Add(time.Hour)
			_, _ = a.DB.Exec("DELETE FROM share_sessions WHERE julianday(expires_at)<=julianday('now')")
			if _, err := a.DB.Exec("INSERT INTO share_sessions(token,share_id,expires_at) VALUES(?,?,?)", session, id, expires.Format(time.RFC3339)); err != nil {
				core.Bad(w, "could not unlock share", 500)
				return false
			}
			http.SetCookie(w, &http.Cookie{Name: "trynas_share", Value: session, Path: "/s/" + tok, HttpOnly: true, Secure: core.SecureCookie(r), SameSite: http.SameSiteLaxMode, MaxAge: 3600})
			// Render the unlocked page directly. A redirect to GET would count as a
			// new visit and must prompt again; no unlock token is put in the URL.
			return true
		}
	} else if resource {
		core.Bad(w, "share password required", 401)
		return false
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = SharePasswordPage.Execute(w, struct {
		Branding core.AppSettings
		Error    string
	}{shareBranding(r), errorText})
	return false
}

var SharePasswordPage = template.Must(template.New("password").Parse(shareBrandTemplate + `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Protected share · {{.Branding.AppName}}</title><script src="/api/branding/theme.js" defer></script><style>` + shareBrandStyle + `:root{--bg:#f4f6fa;--text:#192338;--card:white;--primary:#be123c;--muted:#64748b;--border:#cbd5e1}body{margin:0;min-height:100vh;display:grid;place-items:center;background:var(--bg);font:15px/1.5 system-ui;color:var(--text)}main{margin:20px;padding:32px;border-radius:20px;background:var(--card);color:var(--card-text,var(--text));width:min(380px,calc(100% - 100px))}h1{font-size:24px}label,input,button{display:block}input{box-sizing:border-box;width:100%;padding:12px;margin:8px 0 16px;border:1px solid var(--border);background:var(--bg);color:var(--text);border-radius:8px}button{padding:12px 20px;border:0;border-radius:8px;background:var(--primary);color:var(--primary-foreground,white);cursor:pointer}p{color:var(--muted)}.error{color:#dc2626}</style></head><body><main>{{template "brand" .Branding}}<h1>Password-protected share</h1><p>Enter the share password to access this file or folder. You will be asked again when you reopen this link.</p>{{if .Error}}<p class="error" role="alert">{{.Error}}</p>{{end}}<form method="post" action=""><label for="password">Share password</label><input id="password" name="password" type="password" autocomplete="current-password" required maxlength="72"><button type="submit">Unlock share</button></form></main></body></html>`))

func (a *Module) DownloadSharedFolder(w http.ResponseWriter, r *http.Request, owner core.User, p, full string) {
	a.DownloadSharedItems(w, r, owner, []string{p}, filepath.Base(full)+".zip")
}

func (a *Module) SharePaths(id int64, fallback string) ([]string, error) {
	rows, err := a.DB.Query("SELECT path FROM share_items WHERE share_id=? ORDER BY path", id)
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

func (a *Module) DownloadSharedItems(w http.ResponseWriter, r *http.Request, owner core.User, paths []string, name string) {
	a.Archives.Lock()
	if a.ActiveArchives >= 2 {
		a.Archives.Unlock()
		core.Bad(w, "too many archive downloads; retry later", 429)
		return
	}
	a.ActiveArchives++
	a.Archives.Unlock()
	defer func() { a.Archives.Lock(); a.ActiveArchives--; a.Archives.Unlock() }()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	r = r.WithContext(ctx)
	a.Mutation.Lock()
	locked := true
	defer func() {
		if locked {
			a.Mutation.Unlock()
		}
	}()
	temp, err := os.CreateTemp("", "trynas-share-*.zip")
	if err != nil {
		core.Bad(w, "could not create folder download", 500)
		return
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	archive := zip.NewWriter(&core.ArchiveWriter{Target: temp, Ctx: ctx})
	var inputBytes int64
	entries := 0
	seen := map[string]bool{}
	for _, p := range paths {
		full, resolveErr := core.Safe(a.Root, p)
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
				relative = core.RelJoin(p, suffix)
			}
			if _, err := core.CleanPath(relative); err != nil {
				if st.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if st.Mode()&os.ModeSymlink != 0 {
				return nil
			}
			if st.IsDir() && !a.CanBrowse(owner, relative) {
				return filepath.SkipDir
			}
			if !a.Can(owner, relative, false) {
				return nil
			}
			if !st.IsDir() && !st.Mode().IsRegular() {
				return nil
			}
			checked, err := core.Safe(a.Root, relative)
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
			if entries > core.MaxSharedArchiveEntries {
				return fmt.Errorf("%w: too many entries", core.ErrArchiveLimit)
			}
			if !st.IsDir() {
				if st.Size() > core.MaxSharedArchiveBytes-inputBytes {
					return fmt.Errorf("%w: files exceed 1 GiB", core.ErrArchiveLimit)
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
		if errors.Is(err, core.ErrArchiveLimit) || errors.Is(closeErr, core.ErrArchiveLimit) {
			core.Bad(w, "shared ZIP exceeds the 1 GiB or 10,000-entry limit", 413)
			return
		}
		core.Bad(w, "could not archive shared folder", 500)
		return
	}
	if _, err = temp.Seek(0, io.SeekStart); err != nil {
		core.Bad(w, "could not read folder download", 500)
		return
	}
	st, err := temp.Stat()
	if err != nil {
		core.Bad(w, "could not read folder download", 500)
		return
	}
	a.Mutation.Unlock()
	locked = false
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, name, st.ModTime(), temp)
}

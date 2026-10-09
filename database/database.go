package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/rajibdpi/trynas/core"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

func Open(filename string) (*sql.DB, error) {
	absolute, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(absolute, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	file.Close()
	if err = os.Chmod(absolute, 0600); err != nil {
		return nil, err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err = os.Chmod(absolute+suffix, 0600); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	location := url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}
	// SQLite URI paths on Windows need a slash before the drive letter.
	if location.Path != "" && location.Path[0] != '/' {
		location.Path = "/" + location.Path
	}
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	location.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", location.String())
	if err != nil {
		return nil, err
	}
	// Keep WAL readers reusable while bounding connections under load.
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)
	db.SetConnMaxIdleTime(5 * time.Minute)
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (a *Module) Migrate() error {
	q := `CREATE TABLE IF NOT EXISTS users(id INTEGER PRIMARY KEY,username TEXT UNIQUE NOT NULL,password_hash TEXT NOT NULL,role TEXT NOT NULL DEFAULT 'user',created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
 CREATE TABLE IF NOT EXISTS sessions(token TEXT PRIMARY KEY,user_id INTEGER NOT NULL,expires_at DATETIME NOT NULL,created_at DATETIME DEFAULT CURRENT_TIMESTAMP,FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
 CREATE TABLE IF NOT EXISTS audit_logs(id INTEGER PRIMARY KEY,user_id INTEGER,action TEXT NOT NULL,path TEXT,created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
 CREATE TABLE IF NOT EXISTS favorites(id INTEGER PRIMARY KEY,user_id INTEGER NOT NULL,path TEXT NOT NULL,created_at DATETIME DEFAULT CURRENT_TIMESTAMP,UNIQUE(user_id,path),FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
 CREATE TABLE IF NOT EXISTS trash(id INTEGER PRIMARY KEY,user_id INTEGER NOT NULL,original_path TEXT NOT NULL,trash_name TEXT NOT NULL,deleted_at DATETIME DEFAULT CURRENT_TIMESTAMP,FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
 CREATE TABLE IF NOT EXISTS shares(id INTEGER PRIMARY KEY,user_id INTEGER NOT NULL,path TEXT NOT NULL,token TEXT UNIQUE NOT NULL,expires_at DATETIME,created_at DATETIME DEFAULT CURRENT_TIMESTAMP,FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
 CREATE INDEX IF NOT EXISTS idx_sessions_exp ON sessions(expires_at); CREATE INDEX IF NOT EXISTS idx_fav_user ON favorites(user_id); CREATE INDEX IF NOT EXISTS idx_share_token ON shares(token);`
	if _, e := a.DB.Exec(q); e != nil {
		return e
	}
	var n int
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		password := core.Env("ADMIN_PASSWORD", "")
		if len(password) < 8 || len(password) > 72 || (password == "change-this-now" || password == "replace-with-a-strong-random-password") {
			return fmt.Errorf("set ADMIN_PASSWORD to a strong 8–72 byte password before first startup")
		}
		h, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			return err
		}
		_, e := a.DB.Exec("INSERT INTO users(username,password_hash,role) VALUES(?,?,?)", core.Env("ADMIN_USER", "admin"), string(h), "admin")
		return e
	}
	return nil
}

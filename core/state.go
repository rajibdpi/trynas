package core

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type State struct {
	DB             *sql.DB
	Root, Trash    string
	Mutation       sync.Mutex
	Metrics        sync.Mutex
	Attempts       AttemptLimiter
	Providers      ProviderState
	Archives       sync.Mutex
	ActiveArchives int
	NetworkLast    map[string]NetworkSample
	SmartCached    map[string]any
	SmartAt        time.Time
}

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type CtxKey string

const UserKey CtxKey = "user"

func Env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func JSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func Bad(w http.ResponseWriter, s string, c int) { http.Error(w, s, c) }

func Token() string { b := make([]byte, 24); _, _ = rand.Read(b); return hex.EncodeToString(b) }

func Safe(root, p string) (string, error) {
	rel, err := CleanPath(p)
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

func RelJoin(parts ...string) string {
	return strings.TrimPrefix(filepath.ToSlash(filepath.Join(parts...)), "/")
}

func CopyTree(src, dst string) error {
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
			if e = CopyTree(filepath.Join(src, x.Name()), filepath.Join(dst, x.Name())); e != nil {
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

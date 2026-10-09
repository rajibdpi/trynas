package cmd

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rajibdpi/trynas/app"
	"github.com/rajibdpi/trynas/core"
	"github.com/rajibdpi/trynas/database"
)

// serve initializes storage and starts the standalone HTTP server.
func serve() error {
	dbPath := core.Env("DB_PATH", "./data/trynas.db")
	root := core.Env("STORAGE_ROOT", "./storage")
	var pathErr error
	root, pathErr = filepath.Abs(root)
	if pathErr != nil {
		return pathErr
	}
	dbPath, pathErr = filepath.Abs(dbPath)
	if pathErr != nil {
		return pathErr
	}
	trash := filepath.Join(root, ".trynas-trash")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(trash, 0700); err != nil {
		return err
	}
	db, err := database.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	a := app.New(db, root, trash)
	if err = a.Migrate(); err != nil {
		return err
	}
	if err = a.MigrateV5(); err != nil {
		return err
	}
	a.MonitorNetwork()
	r := a.Router()
	addr := core.Env("ADDR", "127.0.0.1:8080")
	log.Printf("TryNAS v5 API %s storage=%s", addr, root)
	server := &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 15 * time.Minute, WriteTimeout: 15 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	return server.ListenAndServe()
}

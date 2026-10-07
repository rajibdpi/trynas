package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func loadConfig() error {
	config := flag.String("config", ".env", "Optional environment file")
	addr := flag.String("addr", "", "Listen address (overrides ADDR)")
	root := flag.String("root", "", "Storage folder (overrides STORAGE_ROOT)")
	db := flag.String("db", "", "SQLite database path (overrides DB_PATH)")
	flag.Parse()
	explicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			explicit = true
		}
	})
	if err := os.Chmod(*config, 0600); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("secure configuration file: %w", err)
	}
	if err := godotenv.Load(*config); err != nil && (explicit || !os.IsNotExist(err)) {
		return fmt.Errorf("load configuration: %w", err)
	}
	for key, value := range map[string]string{"ADDR": *addr, "STORAGE_ROOT": *root, "DB_PATH": *db} {
		if value != "" {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

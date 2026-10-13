package database

import (
	"os"
	"strings"

	"github.com/rajibdpi/trynas/core"
)

// Provision legacy accounts without moving files or deriving homes from broad ACLs.
func (a *Module) initializeMissingHomes() error {
	rows, err := a.DB.Query("SELECT id,username FROM users WHERE role<>'admin' AND NOT EXISTS(SELECT 1 FROM user_homes WHERE user_id=users.id)")
	if err != nil {
		return err
	}
	type account struct {
		id   int
		name string
	}
	var accounts []account
	for rows.Next() {
		var u account
		if err = rows.Scan(&u.id, &u.name); err != nil {
			rows.Close()
			return err
		}
		accounts = append(accounts, u)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, u := range accounts {
		name, err := core.CleanPath(u.name)
		if err != nil || name == "" || name != u.name || strings.Contains(name, "/") {
			continue
		}
		var assigned int
		if err = a.DB.QueryRow("SELECT COUNT(*) FROM user_homes WHERE path=?", name).Scan(&assigned); err != nil {
			return err
		}
		if assigned != 0 {
			continue
		}
		home, err := core.Safe(a.Root, name)
		if err != nil {
			continue
		}
		info, err := os.Lstat(home)
		created := false
		if os.IsNotExist(err) {
			if err = os.Mkdir(home, 0750); err != nil {
				return err
			}
			created = true
		} else if err != nil {
			return err
		} else if !info.IsDir() {
			continue
		}
		tx, err := a.DB.Begin()
		if err == nil {
			_, err = tx.Exec("INSERT INTO user_homes(user_id,path) VALUES(?,?)", u.id, name)
		}
		if err == nil {
			_, err = tx.Exec("INSERT OR IGNORE INTO permissions(user_id,path,access) VALUES(?,?,'write')", u.id, name)
		}
		if err == nil {
			err = tx.Commit()
		}
		if tx != nil {
			_ = tx.Rollback()
		}
		if err != nil {
			if created {
				_ = os.Remove(home)
			}
			return err
		}
	}
	return nil
}

package trash

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CleanupExpired releases quota only after expired trash is removed successfully.
func (a *Module) CleanupExpired(now time.Time) error {
	settings, err := a.ReadSettings()
	if err != nil || settings.TrashRetentionDays <= 0 {
		return err
	}
	a.Mutation.Lock()
	defer a.Mutation.Unlock()
	cutoff := now.UTC().AddDate(0, 0, -settings.TrashRetentionDays).Format(time.RFC3339Nano)
	rows, err := a.DB.Query("SELECT id,trash_name,user_id FROM trash WHERE julianday(deleted_at)<=julianday(?)", cutoff)
	if err != nil {
		return err
	}
	type entry struct {
		id   int
		uid  int
		name string
	}
	entries := []entry{}
	for rows.Next() {
		var item entry
		if err = rows.Scan(&item.id, &item.name, &item.uid); err != nil {
			rows.Close()
			return err
		}
		entries = append(entries, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, item := range entries {
		if item.name == "" || item.name == "." || item.name == ".." || strings.ContainsAny(item.name, "/\\") {
			return fmt.Errorf("invalid trash entry %d", item.id)
		}
		tx, err := a.DB.Begin()
		if err != nil {
			return err
		}
		prefix := ".trynas-trash/" + item.name
		_, err = tx.Exec("DELETE FROM file_owners WHERE path=? OR substr(path,1,?)=?", prefix, len(prefix)+1, prefix+"/")
		if err == nil {
			_, err = tx.Exec("DELETE FROM trash WHERE id=?", item.id)
		}
		if err == nil {
			err = os.RemoveAll(filepath.Join(a.Trash, item.name))
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			tx.Rollback()
			return err
		}
		a.Log(item.uid, "auto-purge-trash", fmt.Sprint(item.id))
	}
	return nil
}

func (a *Module) RunTrashCleanup(ctx context.Context) {
	run := func() {
		if err := a.CleanupExpired(time.Now()); err != nil {
			log.Printf("Trash cleanup: %v", err)
		}
	}
	run()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

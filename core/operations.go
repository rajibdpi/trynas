package core

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type OperationError struct {
	Message string
	Status  int
}

func (e OperationError) Error() string { return e.Message }

func Conflict(s string) error { return OperationError{s, 409} }

func Forbidden(s string) error { return OperationError{s, 403} }

func Report(w http.ResponseWriter, err error) {
	status := 500
	if e, ok := err.(OperationError); ok {
		status = e.Status
	}
	Bad(w, err.Error(), status)
}

func TreeBytes(f string) (int64, error) {
	var n int64
	err := filepath.Walk(f, func(p string, st os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not supported")
		}
		if st.Mode().IsRegular() {
			n += st.Size()
		}
		return nil
	})
	return n, err
}

func RemapOwners(tx *sql.Tx, from, to string) error {
	rows, err := tx.Query("SELECT path,user_id,bytes FROM file_owners")
	if err != nil {
		return err
	}
	type owned struct {
		p    string
		uid  int
		size int64
	}
	items := []owned{}
	for rows.Next() {
		var x owned
		if err = rows.Scan(&x.p, &x.uid, &x.size); err != nil {
			rows.Close()
			return err
		}
		if Under(x.p, from) {
			items = append(items, x)
		}
	}
	rows.Close()
	for _, x := range items {
		next := to + strings.TrimPrefix(x.p, from)
		if _, err = tx.Exec("UPDATE file_owners SET path=? WHERE path=?", next, x.p); err != nil {
			return err
		}
	}
	return nil
}

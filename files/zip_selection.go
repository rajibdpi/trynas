package files

import (
	"archive/zip"
	"fmt"
	"github.com/rajibdpi/trynas/core"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (a *Module) ZipSelection(u core.User, paths []string, destination string) error {
	if len(paths) == 0 || len(paths) > 100 {
		return core.OperationError{Message: "select 1–100 items", Status: 400}
	}
	p, err := core.CleanPath(destination)
	if err != nil || p == "" || !strings.HasSuffix(strings.ToLower(p), ".zip") {
		return core.OperationError{Message: "choose a ZIP filename", Status: 400}
	}
	clean := []string{}
	for _, selected := range paths {
		selected, err = core.CleanPath(selected)
		if err != nil || selected == "" || core.Under(p, selected) {
			return core.OperationError{Message: "invalid source or ZIP destination inside selection", Status: 400}
		}
		if err = a.AllowTree(u, selected, false); err != nil {
			return core.Forbidden(err.Error())
		}
		clean = append(clean, selected)
	}
	dst, err := a.Destination(u, p)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(dst), ".trynas-zip-")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	zw := zip.NewWriter(temp)
	written := map[string]bool{}
	count := 0
	var total int64
	for _, selected := range clean {
		src, safeErr := core.Safe(a.Root, selected)
		if safeErr != nil {
			err = safeErr
			break
		}
		err = filepath.Walk(src, func(f string, st os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if st.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("symbolic links cannot be archived")
			}
			if !st.IsDir() && !st.Mode().IsRegular() {
				return fmt.Errorf("unsupported file type")
			}
			relative, relErr := filepath.Rel(a.Root, f)
			if relErr != nil {
				return relErr
			}
			name := filepath.ToSlash(relative)
			if written[name] {
				return nil
			}
			written[name] = true
			count++
			total += st.Size()
			if count > 10000 || total > 1<<30 {
				return core.OperationError{Message: "ZIP limited to 10,000 entries and 1 GiB", Status: 413}
			}
			header, headerErr := zip.FileInfoHeader(st)
			if headerErr != nil {
				return headerErr
			}
			header.Name = name
			if st.IsDir() {
				header.Name += "/"
			} else {
				header.Method = zip.Deflate
			}
			out, createErr := zw.CreateHeader(header)
			if createErr != nil || st.IsDir() {
				return createErr
			}
			in, openErr := os.Open(f)
			if openErr != nil {
				return openErr
			}
			_, copyErr := io.Copy(out, io.LimitReader(in, st.Size()+1))
			in.Close()
			return copyErr
		})
		if err != nil {
			break
		}
	}
	closeErr := zw.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	st, err := temp.Stat()
	if err != nil {
		return err
	}
	if err = a.CheckQuota(u.ID, st.Size()); err != nil {
		return core.OperationError{Message: err.Error(), Status: 413}
	}
	if err = temp.Close(); err != nil {
		return err
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO file_owners(path,user_id,bytes) VALUES(?,?,?)", p, u.ID, st.Size()); err != nil {
		return err
	}
	if err = os.Rename(temp.Name(), dst); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	a.Log(u.ID, "zip", p, u.IP)
	return nil
}

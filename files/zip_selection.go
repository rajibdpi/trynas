package files

import (
	"archive/zip"
	"fmt"
	"github.com/rajibdpi/trynas/core"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

// A missing or zero limit leaves creation bounded by available storage and quota.
func archiveCreationLimit(name string) (int64, error) {
	value := os.Getenv(name)
	if value == "" {
		return 0, nil
	}
	limit, err := strconv.ParseInt(value, 10, 64)
	if err != nil || limit < 0 {
		return 0, core.OperationError{Message: name + " must be a non-negative integer", Status: 500}
	}
	return limit, nil
}

func (a *Module) ZipSelection(u core.User, paths []string, destination string) error {
	maxEntries, err := archiveCreationLimit("ARCHIVE_CREATE_MAX_ENTRIES")
	if err != nil {
		return err
	}
	maxBytes, err := archiveCreationLimit("ARCHIVE_CREATE_MAX_BYTES")
	if err != nil {
		return err
	}
	if len(paths) == 0 || len(paths) > 100 {
		return core.OperationError{Message: "select 1–100 items", Status: 400}
	}
	p, err := core.CleanPath(destination)
	if err != nil || p == "" || archiveFormat(p) == "" {
		return core.OperationError{Message: "choose a supported archive filename", Status: 400}
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
	var count int64
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
			if maxEntries > 0 && count > maxEntries {
				return core.OperationError{Message: fmt.Sprintf("archive exceeds configured limit of %d entries (ARCHIVE_CREATE_MAX_ENTRIES)", maxEntries), Status: 413}
			}
			if !st.IsDir() {
				if maxBytes > 0 && st.Size() > maxBytes-total {
					return core.OperationError{Message: fmt.Sprintf("archive contents exceed configured limit of %d bytes (ARCHIVE_CREATE_MAX_BYTES)", maxBytes), Status: 413}
				}
				if maxBytes > 0 {
					total += st.Size()
				}
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
	if archiveFormat(p) != "zip" {
		if err = temp.Close(); err != nil {
			return err
		}
		if err = convertArchive(temp.Name(), archiveFormat(p)); err != nil {
			return err
		}
		temp, err = os.Open(temp.Name())
		if err != nil {
			return err
		}
		defer temp.Close()
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

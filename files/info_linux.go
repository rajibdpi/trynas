//go:build linux

package files

import (
	"fmt"
	"golang.org/x/sys/unix"
	"time"
)

func filesystemMetadata(path string) (*time.Time, string) {
	var stat unix.Statx_t
	if unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, unix.STATX_BTIME|unix.STATX_UID, &stat) != nil {
		return nil, ""
	}
	owner := ""
	if stat.Mask&unix.STATX_UID != 0 {
		owner = fmt.Sprintf("UID %d", stat.Uid)
	}
	if stat.Mask&unix.STATX_BTIME == 0 {
		return nil, owner
	}
	created := time.Unix(stat.Btime.Sec, int64(stat.Btime.Nsec))
	return &created, owner
}

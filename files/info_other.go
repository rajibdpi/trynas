//go:build !linux

package files

import "time"

func filesystemMetadata(path string) (*time.Time, string) { return nil, "" }

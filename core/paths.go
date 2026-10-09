package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"runtime"
	"strings"
)

func CleanPath(p string) (string, error) {
	if strings.ContainsAny(p, "\\\x00:") || strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("use a relative storage path")
	}
	for _, part := range strings.Split(p, "/") {
		if runtime.GOOS == "windows" && part != "" && part != "." {
			base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
			reserved := base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9')
			if strings.ContainsAny(part, "<>\"|?*") || strings.TrimRight(part, ". ") != part || reserved {
				return "", fmt.Errorf("invalid Windows file name")
			}
		}
		if part == ".." || strings.HasPrefix(part, ".trynas-") {
			return "", fmt.Errorf("reserved or invalid path")
		}
	}
	p = path.Clean(p)
	if p == "." {
		p = ""
	}
	return p, nil
}

func Under(p, base string) bool { return p == base || base == "" || strings.HasPrefix(p, base+"/") }

func Decode(w http.ResponseWriter, r *http.Request, x any) bool {
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(x) != nil {
		Bad(w, "invalid request", 400)
		return false
	}
	return true
}

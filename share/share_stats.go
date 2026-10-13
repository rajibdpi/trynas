package share

import (
	"net/http"
	"strings"
)

type shareResponse struct {
	http.ResponseWriter
	status  int
	written bool
	failed  bool
}

func (w *shareResponse) WriteHeader(status int) {
	if w.written {
		return
	}
	w.status = status
	w.written = true
	w.ResponseWriter.WriteHeader(status)
}
func (w *shareResponse) Write(data []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(data)
	if err != nil {
		w.failed = true
	}
	return n, err
}
func (w *shareResponse) Flush() {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Count successful authorized page loads and full download requests, excluding QR,
// HEAD, password prompts, errors, and Range retries.
func (a *Module) trackShare(w http.ResponseWriter, r *http.Request, id int64) (http.ResponseWriter, func()) {
	if r.Method != http.MethodGet || r.URL.Query().Get("qr") == "1" {
		return w, func() {}
	}
	response := &shareResponse{ResponseWriter: w, status: http.StatusOK}
	return response, func() {
		if !response.written || response.failed || response.status != http.StatusOK {
			return
		}
		visits, downloads := 0, 0
		if r.URL.Query().Get("download") == "1" {
			if r.Header.Get("Range") != "" || !strings.HasPrefix(response.Header().Get("Content-Disposition"), "attachment") {
				return
			}
			downloads = 1
		} else {
			if !strings.HasPrefix(response.Header().Get("Content-Type"), "text/html") {
				return
			}
			visits = 1
		}
		_, _ = a.DB.Exec(`INSERT INTO share_stats(share_id,visits,downloads) VALUES(?,?,?) ON CONFLICT(share_id) DO UPDATE SET visits=share_stats.visits+excluded.visits,downloads=share_stats.downloads+excluded.downloads`, id, visits, downloads)
	}
}

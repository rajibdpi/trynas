package app

import (
	"errors"
	"net/http"
	"time"
)

// Authenticated uploads run until completion or client cancellation. Ordinary
// requests retain the HTTP server's deadlines; body-size and quota limits remain.
func unlimitedUpload(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		controller := http.NewResponseController(w)
		if err := controller.SetReadDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
			http.Error(w, "could not clear upload read deadline", http.StatusInternalServerError)
			return
		}
		if err := controller.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
			http.Error(w, "could not clear upload write deadline", http.StatusInternalServerError)
			return
		}
		next.ServeHTTP(w, r)
	})
}

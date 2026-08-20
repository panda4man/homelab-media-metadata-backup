package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

const bearerPrefix = "Bearer "

// requireToken wraps next behind bearer-token authentication, rejecting any
// request whose Authorization header isn't exactly "Bearer <token>". The
// comparison is constant-time so a wrong guess can't be timed against it.
func requireToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		supplied, ok := strings.CutPrefix(header, bearerPrefix)
		if !ok || subtle.ConstantTimeCompare([]byte(supplied), []byte(token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

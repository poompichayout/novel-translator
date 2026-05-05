package server

import (
	"crypto/subtle"
	"net/http"
)

func basicAuth(password string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pwd, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(pwd), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="paste-to-db"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

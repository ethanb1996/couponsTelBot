package security

import (
	"crypto/subtle"
	"net/http"
)

func BasicAuth(username, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			providedUser, providedPass, ok := r.BasicAuth()
			if !ok ||
				subtle.ConstantTimeCompare([]byte(providedUser), []byte(username)) != 1 ||
				subtle.ConstantTimeCompare([]byte(providedPass), []byte(password)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="admin"`)
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

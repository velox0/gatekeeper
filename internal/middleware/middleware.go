package middleware

import (
	"net/http"

	"github.com/velox0/gatekeeper/internal/config"
	"github.com/velox0/gatekeeper/internal/session"
)

// RequireAuth wraps a handler and enforces session cookie presence and validity.
func RequireAuth(next http.Handler, rc *config.ResolvedConfig, store *session.InMemoryStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// paths that don't require auth
		if r.URL.Path == "/login" || r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/favicon.ico" && rc.GetAuthorizeFavicon() {
			next.ServeHTTP(w, r)
			return
		}
		c, err := r.Cookie(rc.GetCookieName())
		if err != nil || c.Value == "" {
			// redirect to login for browser requests
			if r.Header.Get("Accept") == "application/json" || r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			} else {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}
			return
		}
		sess, ok := store.Get(c.Value)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		// inject headers for upstream
		r.Header.Set("X-User", sess.User)
		r.Header.Set("X-Authenticated", "true")
		next.ServeHTTP(w, r)
	})
}

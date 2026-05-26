package auth

import (
	"bytes"
	"embed"
	"html/template"
	"log"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/velox0/gatekeeper/internal/config"
	"github.com/velox0/gatekeeper/internal/plugins"
	"github.com/velox0/gatekeeper/internal/session"
)

//go:embed login.html
var loginPage embed.FS

type Handler struct {
	rc    *config.ResolvedConfig
	store *session.InMemoryStore
	tmpl  *template.Template
}

// loginData is the data passed to the login page template.
type loginData struct {
	AppName string
	Error   string
	Plugins []plugins.LoadedPlugin
}

func NewHandler(rc *config.ResolvedConfig, store *session.InMemoryStore) (*Handler, error) {
	t, err := template.ParseFS(loginPage, "login.html")
	if err != nil {
		return nil, err
	}
	return &Handler{rc: rc, store: store, tmpl: t}, nil
}

func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		loaded, err := plugins.LoadEnabled(h.rc.GetResolvedPlugins())
		if err != nil {
			log.Printf("warning: failed to load plugin assets: %v", err)
		}
		data := loginData{
			AppName: h.rc.AppName,
			Plugins: loaded,
		}
		if msg := r.URL.Query().Get("error"); msg != "" {
			data.Error = msg
		}
		var buf bytes.Buffer
		if err := h.tmpl.Execute(&buf, data); err != nil {
			log.Printf("template error: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := w.Write(buf.Bytes()); err != nil {
			return
		}
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		username := r.PostForm.Get("username")
		password := r.PostForm.Get("password")

		// find user in the resolved (merged) user list
		users := h.rc.GetResolvedUsers()
		var found *config.UserConfig
		for i := range users {
			if users[i].Username == username {
				found = &users[i]
				break
			}
		}
		if found == nil {
			http.Redirect(w, r, "/login?error=Invalid+credentials", http.StatusSeeOther)
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(found.PasswordHash), []byte(password)); err != nil {
			http.Redirect(w, r, "/login?error=Invalid+credentials", http.StatusSeeOther)
			return
		}

		sess, err := h.store.Create(username, h.rc.Auth.SessionTTL)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		cookie := &http.Cookie{
			Name:     h.rc.Auth.CookieName,
			Value:    sess.ID,
			Path:     "/",
			HttpOnly: true,
			Secure:   h.rc.Security.SecureCookies,
			SameSite: http.SameSiteLaxMode,
			Expires:  sess.ExpiresAt,
		}
		http.SetCookie(w, cookie)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	default:
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c, err := r.Cookie(h.rc.Auth.CookieName)
	if err == nil {
		h.store.Delete(c.Value)
		// clear cookie
		http.SetCookie(w, &http.Cookie{
			Name:     h.rc.Auth.CookieName,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   h.rc.Security.SecureCookies,
			SameSite: http.SameSiteLaxMode,
		})
	}
	w.WriteHeader(http.StatusNoContent)
}

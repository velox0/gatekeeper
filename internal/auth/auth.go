package auth

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"html/template"
	"log"
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/tdewolff/minify/v2"
	minifycss "github.com/tdewolff/minify/v2/css"
	minifyhtml "github.com/tdewolff/minify/v2/html"
	minifyjs "github.com/tdewolff/minify/v2/js"

	"golang.org/x/crypto/bcrypt"

	"github.com/velox0/gatekeeper/internal/config"
	"github.com/velox0/gatekeeper/internal/plugins"
	"github.com/velox0/gatekeeper/internal/ratelimit"
	"github.com/velox0/gatekeeper/internal/session"
	"github.com/velox0/gatekeeper/internal/statuspage"
)

//go:embed login.html
var loginPage embed.FS

// dummyHash is used when a username is not found, so that bcrypt.CompareHashAndPassword
// is always called regardless. This prevents timing-based username enumeration.
var dummyHash = []byte("$2a$10$xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")

// maxBcryptPasswordLen is the maximum password length bcrypt supports.
// Passwords longer than this are silently truncated by bcrypt, weakening security.
const maxBcryptPasswordLen = 72

// Failed login limits. The per-IP limit is higher because clients behind a
// shared address (an office NAT, for example) all count against it.
const (
	maxLoginAttemptsPerIP   = 20
	maxLoginAttemptsPerUser = 10
	loginAttemptWindow      = 15 * time.Minute
)

var loginMinifier = newLoginMinifier()

func newLoginMinifier() *minify.M {
	m := minify.New()
	m.AddFunc("text/html", minifyhtml.Minify)
	m.AddFunc("text/css", minifycss.Minify)
	m.AddFunc("application/javascript", minifyjs.Minify)
	return m
}

func minifyLoginHTML(data []byte) ([]byte, error) {
	var out bytes.Buffer
	if err := loginMinifier.Minify("text/html", &out, bytes.NewReader(data)); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

type Handler struct {
	rc    *config.ResolvedConfig
	store session.Store
	tmpl  *template.Template

	ipLimit   *ratelimit.Limiter
	userLimit *ratelimit.Limiter
}

// loginData is the data passed to the login page template.
type loginData struct {
	AppName string
	Error   string
	Plugins []plugins.LoadedPlugin
}

func NewHandler(rc *config.ResolvedConfig, store session.Store) (*Handler, error) {
	data, err := loginPage.ReadFile("login.html")
	if err != nil {
		return nil, err
	}
	if minified, err := minifyLoginHTML(data); err == nil {
		data = minified
	} else {
		log.Printf("warning: failed to minify login template: %v", err)
	}
	t, err := template.New("login.html").Parse(string(data))
	if err != nil {
		return nil, err
	}
	return &Handler{
		rc:        rc,
		store:     store,
		tmpl:      t,
		ipLimit:   ratelimit.New(maxLoginAttemptsPerIP, loginAttemptWindow),
		userLimit: ratelimit.New(maxLoginAttemptsPerUser, loginAttemptWindow),
	}, nil
}

// ipLimitKey groups IPv6 clients by /64, the smallest block usually assigned
// to a single subscriber.
func ipLimitKey(ip netip.Addr) string {
	if ip.Is6() {
		prefix, _ := ip.Prefix(64)
		return prefix.String()
	}
	return ip.String()
}

// userLimitKey hashes the submitted username, which is attacker-controlled
// and may name no real user, so that keys have a fixed size.
func userLimitKey(username string) string {
	sum := sha256.Sum256([]byte(username))
	return string(sum[:])
}

func (h *Handler) renderLogin(w http.ResponseWriter, status int, msg string) {
	loaded, err := plugins.LoadEnabled(h.rc.GetResolvedPlugins())
	if err != nil {
		log.Printf("warning: failed to load plugin assets: %v", err)
	}
	data := loginData{
		AppName: h.rc.GetAppName(),
		Error:   msg,
		Plugins: loaded,
	}
	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, data); err != nil {
		log.Printf("template error: %v", err)
		statuspage.Write(w, http.StatusInternalServerError, h.rc.GetAppName())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		return
	}
}

func (h *Handler) writeTooManyAttempts(w http.ResponseWriter, retryAfter time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
	h.renderLogin(w, http.StatusTooManyRequests, "Too many login attempts. Try again later.")
}

func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.renderLogin(w, http.StatusOK, r.URL.Query().Get("error"))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		username := r.PostForm.Get("username")
		password := r.PostForm.Get("password")

		if len(password) > maxBcryptPasswordLen {
			http.Redirect(w, r, "/login?error=Invalid+credentials", http.StatusSeeOther)
			return
		}

		// Limits apply before bcrypt so blocked attempts cost no CPU, and to
		// unknown usernames so the response does not reveal which ones exist.
		ipKey := ipLimitKey(h.rc.ClientIP(r))
		if ok, retryAfter := h.ipLimit.Reserve(ipKey); !ok {
			h.writeTooManyAttempts(w, retryAfter)
			return
		}
		userKey := userLimitKey(username)
		if ok, retryAfter := h.userLimit.Reserve(userKey); !ok {
			h.ipLimit.Release(ipKey)
			h.writeTooManyAttempts(w, retryAfter)
			return
		}

		// find user in the resolved (merged) user list
		users := h.rc.GetResolvedUsers()
		var hash []byte
		for i := range users {
			if users[i].Username == username {
				hash = []byte(users[i].PasswordHash)
				break
			}
		}
		// Always call bcrypt to prevent timing-based username enumeration.
		if hash == nil {
			hash = dummyHash
		}
		if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
			http.Redirect(w, r, "/login?error=Invalid+credentials", http.StatusSeeOther)
			return
		}
		// Only failed attempts count towards the limits.
		h.ipLimit.Release(ipKey)
		h.userLimit.Release(userKey)

		sess, err := h.store.Create(username, config.CredentialFingerprint(string(hash)), h.rc.GetSessionTTL())
		if err != nil {
			statuspage.Write(w, http.StatusInternalServerError, h.rc.GetAppName())
			return
		}

		cookie := &http.Cookie{
			Name:     h.rc.GetCookieName(),
			Value:    sess.ID,
			Path:     "/",
			HttpOnly: true,
			Secure:   h.rc.GetSecureCookies(),
			SameSite: h.rc.GetSameSite(),
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
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c, err := r.Cookie(h.rc.GetCookieName())
	if err == nil {
		h.store.Delete(c.Value)
		// clear cookie
		http.SetCookie(w, &http.Cookie{
			Name:     h.rc.GetCookieName(),
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   h.rc.GetSecureCookies(),
			SameSite: h.rc.GetSameSite(),
		})
	}
	if r.Method == http.MethodGet {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

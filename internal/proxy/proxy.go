package proxy

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/velox0/gatekeeper/internal/statuspage"
)

func NewReverseProxy(target *url.URL, appName func() string) *httputil.ReverseProxy {
	rp := httputil.NewSingleHostReverseProxy(target)

	// configure transport with timeouts
	rp.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	originalDirector := rp.Director
	rp.Director = func(r *http.Request) {
		proto := "http"
		if r.TLS != nil {
			proto = "https"
		}
		originalDirector(r)
		// preserve protocol and add forwarded headers
		if r.Header.Get("X-Forwarded-Proto") == "" {
			r.Header.Set("X-Forwarded-Proto", proto)
		}
	}
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		status := http.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			status = http.StatusGatewayTimeout
		}
		log.Printf("proxy error for %s %s: %v", r.Method, r.URL.Path, err)
		statuspage.Write(w, status, appName())
	}

	return rp
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

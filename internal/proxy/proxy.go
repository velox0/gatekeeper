package proxy

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func NewReverseProxy(target *url.URL) *httputil.ReverseProxy {
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

	return rp
}

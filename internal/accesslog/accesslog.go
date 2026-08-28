// Package accesslog provides structured HTTP access logging.
package accesslog

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// EnvPath is the environment variable used to configure the access log.
// A value of "-" writes to stdout; an empty value disables access logging.
const EnvPath = "GATEKEEPER_ACCESS_LOG"

// Logger writes one JSON object per completed HTTP request.
type Logger struct {
	mu     sync.Mutex
	out    io.Writer
	closer io.Closer
}

type entry struct {
	Timestamp  time.Time `json:"timestamp"`
	RemoteAddr string    `json:"remote_addr"`
	Method     string    `json:"method"`
	Host       string    `json:"host"`
	Path       string    `json:"path"`
	Status     int       `json:"status"`
	Bytes      int64     `json:"bytes"`
	DurationMS float64   `json:"duration_ms"`
	User       string    `json:"user,omitempty"`
}

// Open creates a logger for path. A path of "-" writes to stdout.
func Open(path string) (*Logger, error) {
	if path == "-" {
		return &Logger{out: os.Stdout}, nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("open access log %q: %w", path, err)
	}
	return &Logger{out: f, closer: f}, nil
}

// OpenFromEnv creates the configured logger, or returns nil when disabled.
func OpenFromEnv() (*Logger, error) {
	path := os.Getenv(EnvPath)
	if path == "" {
		return nil, nil
	}
	return Open(path)
}

// Wrap records requests handled by next.
func (l *Logger) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)

		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		l.write(entry{
			Timestamp:  started.UTC(),
			RemoteAddr: host,
			Method:     r.Method,
			Host:       r.Host,
			Path:       r.URL.RequestURI(),
			Status:     rw.status,
			Bytes:      rw.bytes,
			DurationMS: float64(time.Since(started).Microseconds()) / 1000,
			User:       r.Header.Get("X-User"),
		})
	})
}

func (l *Logger) write(e entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := json.NewEncoder(l.out).Encode(e); err != nil {
		fmt.Fprintf(os.Stderr, "gatekeeper: write access log: %v\n", err)
	}
}

// Close closes the backing file, when one is in use.
func (l *Logger) Close() error {
	if l == nil || l.closer == nil {
		return nil
	}
	return l.closer.Close()
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	bytes       int64
	wroteHeader bool
}

func (w *responseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += int64(n)
	return n, err
}

// Unwrap lets http.ResponseController reach optional interfaces implemented by
// the underlying writer, including flushing and connection hijacking.
func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

package accesslog

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrapWritesStructuredEntry(t *testing.T) {
	var out bytes.Buffer
	logger := &Logger{out: &out}
	handler := logger.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-User", "ayush")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))

	req := httptest.NewRequest(http.MethodPost, "http://app.local/widgets?source=test", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var got entry
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}
	if got.RemoteAddr != "192.0.2.10" || got.Method != http.MethodPost || got.Host != "app.local" {
		t.Fatalf("unexpected request fields: %+v", got)
	}
	if got.Path != "/widgets?source=test" || got.Status != http.StatusCreated || got.Bytes != 7 {
		t.Fatalf("unexpected response fields: %+v", got)
	}
	if got.User != "ayush" {
		t.Fatalf("User = %q, want ayush", got.User)
	}
}

func TestWrapDefaultsStatusToOK(t *testing.T) {
	var out bytes.Buffer
	logger := &Logger{out: &out}
	logger.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "http://app.local/", nil),
	)

	var got entry
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}
	if got.Status != http.StatusOK {
		t.Fatalf("Status = %d, want %d", got.Status, http.StatusOK)
	}
}

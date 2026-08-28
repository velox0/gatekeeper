package statuspage

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteRendersStatusPage(t *testing.T) {
	tests := []struct {
		status int
		title  string
	}{
		{http.StatusInternalServerError, "Internal server error"},
		{http.StatusBadGateway, "Bad gateway"},
		{http.StatusServiceUnavailable, "Service unavailable"},
		{http.StatusGatewayTimeout, "Gateway timeout"},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			rec := httptest.NewRecorder()
			Write(rec, tt.status, "My App")

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Fatalf("Content-Type = %q", got)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control = %q, want no-store", got)
			}
			for _, want := range []string{tt.title, "My App"} {
				if !strings.Contains(rec.Body.String(), want) {
					t.Errorf("body does not contain %q", want)
				}
			}
		})
	}
}

func TestWriteEscapesAppName(t *testing.T) {
	rec := httptest.NewRecorder()
	Write(rec, http.StatusInternalServerError, `<script>alert("x")</script>`)

	if strings.Contains(rec.Body.String(), "<script>") {
		t.Fatal("app name was not HTML-escaped")
	}
}

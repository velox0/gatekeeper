// Package statuspage renders the built-in HTTP 5xx response pages.
package statuspage

import (
	_ "embed"
	"html/template"
	"log"
	"net/http"
)

//go:embed status.html
var pageHTML string

var pageTemplate = template.Must(template.New("status").Parse(pageHTML))

type pageData struct {
	AppName string
	Code    int
	Title   string
	Message string
}

// Write renders a built-in error page for an HTTP 5xx status code.
func Write(w http.ResponseWriter, status int, appName string) {
	title, message := copyFor(status)
	if appName == "" {
		appName = "Gatekeeper"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := pageTemplate.Execute(w, pageData{
		AppName: appName,
		Code:    status,
		Title:   title,
		Message: message,
	}); err != nil {
		log.Printf("failed to render %d status page: %v", status, err)
	}
}

func copyFor(status int) (string, string) {
	switch status {
	case http.StatusBadGateway:
		return "Bad gateway", "The upstream application returned an invalid response."
	case http.StatusServiceUnavailable:
		return "Service unavailable", "The application is temporarily unavailable."
	case http.StatusGatewayTimeout:
		return "Gateway timeout", "The upstream application took too long to respond."
	default:
		return "Internal server error", "Something went wrong while handling your request."
	}
}

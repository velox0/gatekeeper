// Package errorlog configures persistent output for the standard application logger.
package errorlog

import (
	"fmt"
	"io"
	"log"
	"os"
)

// EnvPath is the environment variable used to configure the error log.
// An empty value disables persistent error logging.
const EnvPath = "GATEKEEPER_ERROR_LOG"

// Sink owns an error-log file and restores the previous logger on close.
type Sink struct {
	file     *os.File
	previous io.Writer
}

// Open starts duplicating standard logger output to path and stderr.
func Open(path string) (*Sink, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("open error log %q: %w", path, err)
	}
	previous := log.Writer()
	log.SetOutput(io.MultiWriter(previous, f))
	return &Sink{file: f, previous: previous}, nil
}

// OpenFromEnv opens the configured error log, or returns nil when disabled.
func OpenFromEnv() (*Sink, error) {
	path := os.Getenv(EnvPath)
	if path == "" {
		return nil, nil
	}
	return Open(path)
}

// Close restores the previous logger output and closes the backing file.
func (s *Sink) Close() error {
	if s == nil {
		return nil
	}
	log.SetOutput(s.previous)
	return s.file.Close()
}

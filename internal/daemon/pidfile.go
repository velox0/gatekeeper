package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// DefaultPIDPath mirrors the nginx convention: /var/run/<name>.pid
const DefaultPIDPath = "/var/run/gatekeeper.pid"

// WritePID writes the current process ID to the given file path.
func WritePID(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0644)
}

// ReadPID reads the daemon PID from the given file path.
func ReadPID(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("cannot read PID file %s: %w", path, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in %s: %w", path, err)
	}
	return pid, nil
}

// RemovePID removes the PID file. Errors are silently ignored (best-effort cleanup).
func RemovePID(path string) {
	_ = os.Remove(path)
}

// SignalReload sends SIGHUP to the daemon identified by the PID file.
// Returns nil if the signal was sent successfully.
// Returns an error if the PID file is missing, stale, or the signal fails.
func SignalReload(pidPath string) error {
	pid, err := ReadPID(pidPath)
	if err != nil {
		return err
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("cannot find process %d: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("failed to send SIGHUP to PID %d: %w", pid, err)
	}

	return nil
}

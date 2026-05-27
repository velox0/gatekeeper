package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// DefaultPIDPath mirrors the nginx convention: /var/run/<name>.pid
const DefaultPIDPath = "/var/run/gatekeeper.pid"

// configPathSuffix stores the active config path alongside the PID file.
const configPathSuffix = ".config"

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

// ConfigPathFile returns the companion file that stores the active config path.
func ConfigPathFile(pidPath string) string {
	return pidPath + configPathSuffix
}

// WriteConfigPath writes the active config path next to the PID file.
func WriteConfigPath(pidPath, configPath string) error {
	// Persist an absolute path so CLI invocations from other working
	// directories can still resolve the daemon's active config reliably.
	if !filepath.IsAbs(configPath) {
		abs, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("cannot resolve absolute config path for %q: %w", configPath, err)
		}
		configPath = abs
	}

	// Match PID file readability; config path is not secret, and this allows
	// non-root CLI invocations to discover the active config.
	return os.WriteFile(ConfigPathFile(pidPath), []byte(configPath+"\n"), 0644)
}

// ReadConfigPath reads the active config path next to the PID file.
func ReadConfigPath(pidPath string) (string, error) {
	b, err := os.ReadFile(ConfigPathFile(pidPath))
	if err != nil {
		return "", fmt.Errorf("cannot read config path file %s: %w", ConfigPathFile(pidPath), err)
	}
	configPath := strings.TrimSpace(string(b))
	if configPath == "" {
		return "", fmt.Errorf("empty config path in %s", ConfigPathFile(pidPath))
	}
	return configPath, nil
}

// RemoveConfigPath removes the config-path companion file.
func RemoveConfigPath(pidPath string) {
	_ = os.Remove(ConfigPathFile(pidPath))
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

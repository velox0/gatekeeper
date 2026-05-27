package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadPID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")

	if err := WritePID(path); err != nil {
		t.Fatalf("WritePID error: %v", err)
	}

	pid, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID error: %v", err)
	}

	if pid != os.Getpid() {
		t.Fatalf("PID = %d, want %d", pid, os.Getpid())
	}
}

func TestReadPIDMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.pid")

	_, err := ReadPID(path)
	if err == nil {
		t.Fatal("ReadPID should fail for missing file")
	}
}

func TestReadPIDInvalidContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pid")
	if err := os.WriteFile(path, []byte("not-a-number\n"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	_, err := ReadPID(path)
	if err == nil {
		t.Fatal("ReadPID should fail for non-numeric content")
	}
}

func TestRemovePIDCleansUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.pid")
	if err := WritePID(path); err != nil {
		t.Fatalf("WritePID error: %v", err)
	}

	RemovePID(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("PID file should have been removed")
	}
}

func TestRemovePIDMissingFileNoError(t *testing.T) {
	// RemovePID should not panic on missing files.
	RemovePID(filepath.Join(t.TempDir(), "nonexistent.pid"))
}

func TestWriteReadAndRemoveConfigPath(t *testing.T) {
	pidPath := filepath.Join(t.TempDir(), "gatekeeper.pid")
	configPath := filepath.Join(t.TempDir(), "config.yml")

	if err := WriteConfigPath(pidPath, configPath); err != nil {
		t.Fatalf("WriteConfigPath error: %v", err)
	}

	got, err := ReadConfigPath(pidPath)
	if err != nil {
		t.Fatalf("ReadConfigPath error: %v", err)
	}
	if got != configPath {
		t.Fatalf("ReadConfigPath = %q, want %q", got, configPath)
	}

	RemoveConfigPath(pidPath)
	if _, err := os.Stat(ConfigPathFile(pidPath)); !os.IsNotExist(err) {
		t.Fatal("config path file should have been removed")
	}
}

package errorlog

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenDuplicatesAndPersistsLogs(t *testing.T) {
	previous := log.Writer()
	var console bytes.Buffer
	log.SetOutput(&console)
	t.Cleanup(func() { log.SetOutput(previous) })

	path := filepath.Join(t.TempDir(), "error.log")
	sink, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	log.Print("upstream failed")
	if err := sink.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !strings.Contains(string(data), "upstream failed") {
		t.Fatalf("file log = %q", data)
	}
	if !strings.Contains(console.String(), "upstream failed") {
		t.Fatalf("console log = %q", console.String())
	}
	if log.Writer() != &console {
		t.Fatal("Close did not restore the previous log writer")
	}
}

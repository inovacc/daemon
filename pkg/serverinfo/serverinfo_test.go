package serverinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	in := Info{Address: "localhost:9500", Port: 9500, PID: 4242, Version: "v1.2.3"}
	if err := s.Write(in); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := s.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got == nil {
		t.Fatal("Read returned nil after Write")
	}
	if got.Port != 9500 || got.Address != "localhost:9500" || got.PID != 4242 || got.Version != "v1.2.3" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	if got.StartedAt.IsZero() {
		t.Fatal("StartedAt should be stamped by Write")
	}
}

func TestReadMissingReturnsNilNil(t *testing.T) {
	s := NewStore(t.TempDir())
	got, err := s.Read()
	if err != nil {
		t.Fatalf("Read of missing file should not error, got %v", err)
	}
	if got != nil {
		t.Fatalf("Read of missing file should be nil, got %+v", got)
	}
}

func TestWriteCreatesServerJSONInDir(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Write(Info{PID: 1}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "server.json")); err != nil {
		t.Fatalf("server.json not created: %v", err)
	}
}

func TestRemoveIsIdempotent(t *testing.T) {
	s := NewStore(t.TempDir())
	// Remove on missing file must not error.
	if err := s.Remove(); err != nil {
		t.Fatalf("Remove missing: %v", err)
	}
	_ = s.Write(Info{PID: 1})
	if err := s.Remove(); err != nil {
		t.Fatalf("Remove existing: %v", err)
	}
	got, _ := s.Read()
	if got != nil {
		t.Fatal("file should be gone after Remove")
	}
}

func TestIsRunningReturnsInfoWhenPIDAlive(t *testing.T) {
	s := NewStore(t.TempDir())
	s.alive = func(int) bool { return true }
	_ = s.Write(Info{PID: 12345, Port: 9500})

	if got := s.IsRunning(); got == nil || got.Port != 9500 {
		t.Fatalf("IsRunning should report the live instance, got %+v", got)
	}
}

func TestIsRunningClearsStaleFileWhenPIDDead(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	s.alive = func(int) bool { return false }
	_ = s.Write(Info{PID: 999999})

	if got := s.IsRunning(); got != nil {
		t.Fatalf("IsRunning should be nil for a dead PID, got %+v", got)
	}
	// Stale file must be cleaned up.
	if _, err := os.Stat(filepath.Join(dir, "server.json")); !os.IsNotExist(err) {
		t.Fatal("stale server.json should be removed when PID is dead")
	}
}

func TestIsRunningNilWhenNoFile(t *testing.T) {
	s := NewStore(t.TempDir())
	if got := s.IsRunning(); got != nil {
		t.Fatalf("IsRunning with no file should be nil, got %+v", got)
	}
}

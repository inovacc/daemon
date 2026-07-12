package serverinfo

import (
	"errors"
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

func TestReadMissingReturnsErrNoRecord(t *testing.T) {
	s := NewStore(t.TempDir())

	got, err := s.Read()
	if !errors.Is(err, ErrNoRecord) {
		t.Fatalf("Read of missing file should report ErrNoRecord, got %v", err)
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

func TestReadCorruptJSONReturnsError(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	if err := os.WriteFile(s.Path(), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := s.Read()
	// A malformed file is a real error, NOT the ErrNoRecord "not running" sentinel.
	if err == nil || errors.Is(err, ErrNoRecord) {
		t.Fatalf("Read of corrupt JSON should return a decode error, got %v", err)
	}

	if got != nil {
		t.Fatalf("Read of corrupt JSON should return nil info, got %+v", got)
	}
}

func TestReadSurfacesNonNotExistError(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	// server.json is a directory → ReadFile fails with a non-ErrNotExist error,
	// which must propagate rather than be masked as ErrNoRecord.
	if err := os.Mkdir(s.Path(), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := s.Read()
	if err == nil || errors.Is(err, ErrNoRecord) {
		t.Fatalf("Read of a non-file path should surface the I/O error, got %v", err)
	}
}

func TestWriteSurfacesWriteError(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	// server.json is a directory → WriteFile cannot write to it.
	if err := os.Mkdir(s.Path(), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := s.Write(Info{PID: 1}); err == nil {
		t.Fatal("Write to an unwritable path must return an error")
	}
}

func TestRemoveSurfacesNonNotExistError(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	// server.json is a NON-EMPTY directory → os.Remove refuses it on every platform,
	// and that error (unlike a missing file) must propagate.
	if err := os.Mkdir(s.Path(), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := os.WriteFile(filepath.Join(s.Path(), "blocker"), []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := s.Remove(); err == nil {
		t.Fatal("Remove of a non-empty directory must return the underlying error")
	}
}

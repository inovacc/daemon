// Package serverinfo persists and reads the daemon's server.json PID file.
//
// The file records the MONITOR process (not the worker), so a `service stop` can
// kill the whole process tree from the root. IsRunning combines the on-disk record
// with a platform liveness check and self-heals a stale file left by a crashed monitor.
package serverinfo

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// FileName is the fixed name of the PID file inside the store directory.
const FileName = "server.json"

// ErrNoRecord reports that no PID file exists yet. It is an expected,
// non-fatal signal (the daemon is simply not recorded as running) and is
// returned by Read when the underlying file is absent.
var ErrNoRecord = errors.New("serverinfo: no record")

// Info is the persisted record of a running daemon instance.
type Info struct {
	Address   string    `json:"address,omitempty"`
	Port      int       `json:"port,omitempty"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	Version   string    `json:"version,omitempty"`
}

// Store reads and writes a single server.json within a directory.
type Store struct {
	dir   string
	alive func(pid int) bool
}

// NewStore returns a Store backed by dir/server.json.
func NewStore(dir string) *Store {
	return &Store{dir: dir, alive: processAlive}
}

// Path is the absolute path to the PID file.
func (s *Store) Path() string { return filepath.Join(s.dir, FileName) }

// Write persists info, stamping StartedAt with the current time when unset.
func (s *Store) Write(info Info) error {
	if info.StartedAt.IsZero() {
		info.StartedAt = time.Now()
	}

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.Path(), data, 0o644)
}

// Read returns the persisted info, or ErrNoRecord when the file does not exist.
// Callers should treat ErrNoRecord (via errors.Is) as an expected "not running"
// state rather than a failure.
func (s *Store) Read() (*Info, error) {
	data, err := os.ReadFile(s.Path())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// A missing PID file is an expected, non-error state; surface it as a
			// typed sentinel so callers can distinguish it from real I/O failures.
			return nil, ErrNoRecord
		}

		return nil, err
	}

	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// Remove deletes the PID file. Missing file is not an error.
func (s *Store) Remove() error {
	if err := os.Remove(s.Path()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return nil
}

// IsRunning reports the live instance, or nil. A record whose PID is no longer
// alive is treated as stale: the file is removed and nil is returned. A corrupt or
// otherwise unreadable file (any error other than the expected ErrNoRecord) is also
// self-healed — removed — so junk left by a crashed writer cannot block the next start.
func (s *Store) IsRunning() *Info {
	info, err := s.Read()
	if err != nil {
		if !errors.Is(err, ErrNoRecord) {
			_ = s.Remove()
		}

		return nil
	}

	if info == nil {
		return nil
	}

	if !s.alive(info.PID) {
		_ = s.Remove()
		return nil
	}

	return info
}

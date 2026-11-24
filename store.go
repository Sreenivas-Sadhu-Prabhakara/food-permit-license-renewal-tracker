package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/sreenivas-sadhu-prabhakara/food-permit-license-renewal-tracker/internal/permits"
)

// Store keeps the permit register in memory and on disk in one plain JSON file.
//
// There is no database and no server. The file sits wherever the owner points
// -data at, so it can live in a folder that is already being backed up or on a
// pen drive that goes home at night. Writes go to a temporary file in the same
// directory and are then renamed over the old one, so a crash, a full disk or a
// closed laptop leaves the previous good file intact rather than half of a new
// one.
type Store struct {
	mu       sync.RWMutex
	path     string
	register *permits.Register
}

// OpenStore loads the data file, or starts an empty register if there is not
// one yet. It does not create the file until something is saved, so running
// the tool to look at it leaves nothing behind.
func OpenStore(path string) (*Store, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("data file %s: %w", path, err)
	}
	s := &Store{path: absolute, register: permits.New()}

	raw, err := os.ReadFile(absolute)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not read the data file %s: %w", absolute, err)
	}
	if len(raw) == 0 {
		return s, nil
	}
	loaded := permits.New()
	if err := json.Unmarshal(raw, loaded); err != nil {
		return nil, fmt.Errorf("the data file %s is not valid JSON: %w", absolute, err)
	}
	loaded.Normalise()
	s.register = loaded
	return s, nil
}

// Path is where the data file lives, printed at startup and shown in the
// interface so the owner always knows which file they are editing.
func (s *Store) Path() string { return s.path }

// Read runs fn against the register under a read lock. fn must not keep the
// pointer or change anything behind it.
func (s *Store) Read(fn func(r *permits.Register)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(s.register)
}

// Write runs fn against the register under a write lock and saves the file if
// fn succeeds. Nothing is saved if fn fails, and the domain validates every
// edit before it touches a row, so a rejected edit leaves memory untouched too.
func (s *Store) Write(fn func(r *permits.Register) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(s.register); err != nil {
		return err
	}
	return s.save()
}

// ReplacePermits swaps every permit for the ones given, used by an import that
// replaces the register rather than merging into it. The owner's own settings —
// the business details, the reminder tiers, the late-payment rates — are kept,
// because a CSV of permits says nothing about any of them.
func (s *Store) ReplacePermits(replacement []permits.Permit) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fresh := permits.New()
	fresh.Business = s.register.Business
	fresh.Tiers = s.register.Tiers
	fresh.LateRates = s.register.LateRates
	fresh.Permits = replacement
	fresh.Normalise()
	s.register = fresh
	return s.save()
}

// save writes the data file atomically: a temporary file in the same
// directory, flushed to disk, then renamed over the target.
func (s *Store) save() error {
	raw, err := json.MarshalIndent(s.register, "", "  ")
	if err != nil {
		return fmt.Errorf("could not encode the register: %w", err)
	}
	raw = append(raw, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("could not write beside %s: %w", s.path, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // harmless once the rename has happened

	// The register holds the licence numbers of somebody's business. It is
	// readable by its owner and nobody else.
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("could not set permissions on %s: %w", tmpName, err)
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return fmt.Errorf("could not write %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("could not flush %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("could not close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("could not replace %s: %w", s.path, err)
	}
	return nil
}

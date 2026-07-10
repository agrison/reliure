// Package settings persists user preferences as JSON in the OS config
// directory. It is deliberately small and additive: new preferences (server
// ports, hooks…) get new fields with defaults, and old files keep loading.
// A Store guards concurrent access and writes atomically.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/agrison/reliure/internal/library"
)

// Settings is the persisted preference set.
type Settings struct {
	// ImportMode selects copy (managed library) vs. reference (index in place).
	ImportMode library.Mode `json:"importMode"`
	// LibraryDir is the managed library root, used only in copy mode.
	LibraryDir string `json:"libraryDir"`
}

// Store loads, exposes and persists Settings. Safe for concurrent use.
type Store struct {
	path           string
	defaultLibrary string
	mu             sync.RWMutex
	cur            Settings
}

// Open loads settings from path, creating the file with defaults if absent.
// defaultLibraryDir seeds LibraryDir when the stored value is empty (e.g. a
// fresh install or an older settings file).
func Open(path, defaultLibraryDir string) (*Store, error) {
	s := &Store{path: path, defaultLibrary: defaultLibraryDir}

	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		s.cur = s.normalize(Settings{})
		return s, s.save()
	case err != nil:
		return nil, err
	}

	var loaded Settings
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, err
	}
	s.cur = s.normalize(loaded)
	return s, nil
}

// Get returns a copy of the current settings.
func (s *Store) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cur
}

// Update validates, applies and persists next, returning the effective (post-
// normalization) settings.
func (s *Store) Update(next Settings) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cur = s.normalize(next)
	if err := s.save(); err != nil {
		return s.cur, err
	}
	return s.cur, nil
}

// normalize fills in defaults for missing/invalid fields.
func (s *Store) normalize(in Settings) Settings {
	if !in.ImportMode.Valid() {
		in.ImportMode = library.ModeCopy
	}
	if in.LibraryDir == "" {
		in.LibraryDir = s.defaultLibrary
	}
	return in
}

// save writes the current settings atomically (temp file + rename). Callers
// hold s.mu.
func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.cur, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

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
	// RemotePathTemplate controls where files are placed on KOReader sends.
	RemotePathTemplate string `json:"remotePathTemplate"`
	// OPDSEnabled controls whether the pull catalog starts with the app.
	OPDSEnabled bool `json:"opdsEnabled"`
	// OPDSPort is the TCP port used by the local OPDS catalog.
	OPDSPort int `json:"opdsPort"`
	// CalibreEnabled controls whether the Calibre wireless (push) server starts
	// with the app, so KOReader can connect to send/receive books.
	CalibreEnabled bool `json:"calibreEnabled"`
	// WriteMetadataToFile, when true, writes edited metadata back into the ebook
	// file (rewrites the EPUB's OPF) on save, so readers that read the file
	// directly (e.g. KOReader's file browser) see Reliure's values. Off by
	// default: it modifies files on disk, including referenced originals.
	WriteMetadataToFile bool `json:"writeMetadataToFile"`
	// Theme is the UI appearance: "system" (follow the OS), "light" or "dark".
	Theme string `json:"theme"`
	// KoreaderSyncDir is the last folder scanned for KOReader `.sdr` sidecars
	// (a mounted device or a synced copy of the reader's library), remembered so
	// re-syncing reading progress is one click.
	KoreaderSyncDir string `json:"koreaderSyncDir"`
	// FeatureDiscover controls whether the Project Gutenberg discovery view is
	// shown in the UI.
	FeatureDiscover bool `json:"featureDiscover"`
	// FeatureSmartShelves controls whether dynamic rule-based shelves are shown
	// in the UI.
	FeatureSmartShelves bool `json:"featureSmartShelves"`
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
		s.cur = s.normalize(Settings{FeatureDiscover: true, FeatureSmartShelves: true})
		return s, s.save()
	case err != nil:
		return nil, err
	}

	var loaded Settings
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		if _, ok := raw["featureDiscover"]; !ok {
			loaded.FeatureDiscover = true
		}
		if _, ok := raw["featureSmartShelves"]; !ok {
			loaded.FeatureSmartShelves = true
		}
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
	if in.RemotePathTemplate == "" {
		in.RemotePathTemplate = "{authors}/{series}/{series_index} {title}"
	}
	if in.OPDSPort <= 0 || in.OPDSPort > 65535 {
		in.OPDSPort = 8088
	}
	switch in.Theme {
	case "system", "light", "dark":
	default:
		in.Theme = "system"
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

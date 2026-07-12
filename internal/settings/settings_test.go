package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agrison/reliure/internal/library"
)

func TestOpenCreatesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path, "/default/lib")
	if err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	if got.ImportMode != library.ModeCopy {
		t.Errorf("default mode = %q, want copy", got.ImportMode)
	}
	if got.LibraryDir != "/default/lib" {
		t.Errorf("default library dir = %q", got.LibraryDir)
	}
	// The file was written.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("settings file not created: %v", err)
	}
}

func TestUpdatePersistsAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path, "/default/lib")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(Settings{ImportMode: library.ModeReference, LibraryDir: "/custom"}); err != nil {
		t.Fatal(err)
	}

	// Reopen from disk: the change survives.
	s2, err := Open(path, "/default/lib")
	if err != nil {
		t.Fatal(err)
	}
	got := s2.Get()
	if got.ImportMode != library.ModeReference || got.LibraryDir != "/custom" {
		t.Errorf("reloaded settings = %+v", got)
	}
}

func TestNormalizeInvalidMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	// Write a file with a bogus mode and empty dir.
	os.WriteFile(path, []byte(`{"importMode":"nonsense","libraryDir":""}`), 0o644)

	s, err := Open(path, "/fallback")
	if err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	if got.ImportMode != library.ModeCopy {
		t.Errorf("invalid mode should normalize to copy, got %q", got.ImportMode)
	}
	if got.LibraryDir != "/fallback" {
		t.Errorf("empty dir should fall back, got %q", got.LibraryDir)
	}
}

func TestUpdateEmptyDirFallsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, _ := Open(path, "/fallback")
	got, err := s.Update(Settings{ImportMode: library.ModeReference, LibraryDir: ""})
	if err != nil {
		t.Fatal(err)
	}
	if got.LibraryDir != "/fallback" {
		t.Errorf("empty library dir should fall back to default, got %q", got.LibraryDir)
	}
}

func TestLanguageDefaultsAndNormalization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path, "/default/lib")
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Get().Language; got != "fr" {
		t.Fatalf("default language = %q, want fr", got)
	}
	next := s.Get()
	next.Language = "de"
	got, err := s.Update(next)
	if err != nil {
		t.Fatal(err)
	}
	if got.Language != "de" {
		t.Fatalf("language = %q, want de", got.Language)
	}
	next.Language = "xx"
	got, err = s.Update(next)
	if err != nil {
		t.Fatal(err)
	}
	if got.Language != "fr" {
		t.Fatalf("invalid language should normalize to fr, got %q", got.Language)
	}
}

func TestWatchFolderDefaultsAndDelayNormalization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path, "/default/lib")
	if err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	if got.WatchFolderEnabled || got.WatchFolderDir != "" || got.WatchFolderDeleteSource {
		t.Fatalf("watch folder should be disabled by default: %+v", got)
	}
	if got.WatchFolderDelaySeconds != 10 {
		t.Fatalf("default watch delay = %d, want 10", got.WatchFolderDelaySeconds)
	}

	got.WatchFolderDelaySeconds = -1
	got.WatchFolderEnabled = true
	got.WatchFolderDir = "/drop"
	got.WatchFolderDeleteSource = true
	next, err := s.Update(got)
	if err != nil {
		t.Fatal(err)
	}
	if next.WatchFolderDelaySeconds != 10 {
		t.Fatalf("normalized watch delay = %d, want 10", next.WatchFolderDelaySeconds)
	}
	if !next.WatchFolderEnabled || next.WatchFolderDir != "/drop" || !next.WatchFolderDeleteSource {
		t.Fatalf("watch folder settings not persisted through normalization: %+v", next)
	}
}

func TestFeatureTogglesDefaultVisibleButCanBeDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte(`{"importMode":"copy","libraryDir":"/lib"}`), 0o644)

	s, err := Open(path, "/fallback")
	if err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	if !got.FeatureDiscover || !got.FeatureSmartShelves {
		t.Fatalf("missing feature fields should default visible: %+v", got)
	}
	got.FeatureDiscover = false
	got.FeatureSmartShelves = false
	if _, err := s.Update(got); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path, "/fallback")
	if err != nil {
		t.Fatal(err)
	}
	got = s2.Get()
	if got.FeatureDiscover || got.FeatureSmartShelves {
		t.Fatalf("disabled feature toggles should persist: %+v", got)
	}
}

func TestOpenRejectsCorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte("{not json"), 0o644)
	if _, err := Open(path, "/x"); err == nil {
		t.Error("expected error on corrupt settings file")
	}
}

func TestSavedJSONShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, _ := Open(path, "/default/lib")
	s.Update(Settings{ImportMode: library.ModeReference, LibraryDir: "/custom"})

	data, _ := os.ReadFile(path)
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	if m["importMode"] != "reference" || m["libraryDir"] != "/custom" {
		t.Errorf("unexpected JSON: %s", data)
	}
}

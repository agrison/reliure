package core

import (
	"os"
	"path/filepath"
)

// appDir is the application's subdirectory name inside the OS config dir.
const appDir = "reliure"

// ConfigDir returns the application's configuration directory inside the OS
// standard config location (~/Library/Application Support on macOS, %AppData%
// on Windows, ~/.config on Linux), creating it if needed.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, appDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// DefaultDBPath returns the SQLite file path inside the config directory. The
// library directory (where EPUB files are copied) is a separate concept the
// user configures independently.
func DefaultDBPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "library.db"), nil
}

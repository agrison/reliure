package library

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// Scan walks root recursively and returns the paths of every file a registered
// format handler can import, sorted for deterministic ordering. It is tolerant:
// unreadable directories are skipped rather than aborting the walk, and hidden
// directories (dot-prefixed) are pruned.
func (imp *Importer) Scan(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip what we can't read; keep going.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if _, ok := imp.reg.HandlerFor(path); ok {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

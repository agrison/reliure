package library

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxNameLen caps a single path component (in runes) to stay well under file
// system limits once combined into a full path.
const maxNameLen = 150

// hashFile computes the SHA-256 (hex) and size of a file in a single pass.
func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// sanitize turns an arbitrary author/title into a safe path component: reserved
// and control characters become '_', surrounding spaces/dots are trimmed (they
// trip up Windows), and the result is length-capped on rune boundaries.
func sanitize(name string) string {
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		if r < 0x20 {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" {
		return "_"
	}
	if r := []rune(name); len(r) > maxNameLen {
		name = strings.TrimRight(string(r[:maxNameLen]), " .")
	}
	return name
}

// destPath computes where a book's file is copied: LibraryDir/Author/Title/Title.ext.
func (imp *Importer) destPath(author, title, srcPath, format string) string {
	if author == "" {
		author = "Unknown Author"
	}
	if title == "" {
		title = "Untitled"
	}
	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext == "" {
		ext = "." + format
	}
	dir := filepath.Join(imp.cfg.LibraryDir, sanitize(author), sanitize(title))
	return filepath.Join(dir, sanitize(title)+ext)
}

// placeFile copies the source file to its destination, creating directories and
// avoiding clobbering an unrelated existing file (a numeric suffix is added on
// collision). Returns the final path. The copy is atomic within the directory
// (write to a temp file, then rename).
func placeFile(srcPath, destPath string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return "", err
	}
	final := uniquePath(destPath)
	if err := copyContents(srcPath, final); err != nil {
		return "", err
	}
	return final, nil
}

// uniquePath returns path unchanged if free, otherwise inserts " (N)" before the
// extension until it finds a free name. Deduplication has already ruled out an
// identical file, so a collision here means a genuinely different file.
func uniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s (%d)%s", stem, n, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// copyContents copies src to dst via a temp file + rename for atomicity.
func copyContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	// Clean up the temp file if anything below fails.
	defer func() {
		if err != nil {
			os.Remove(tmp)
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err = out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

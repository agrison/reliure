package library

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/agrison/reliure/internal/core"
	"github.com/agrison/reliure/internal/formats"
)

// DefaultThumbnailMax is the default largest side (px) of generated thumbnails.
const DefaultThumbnailMax = 400

// GenerateCover extracts a cover from the first of a book's files that yields
// one, writes a cached JPEG thumbnail named "<bookID>.jpg" into coverDir, and
// returns that relative name — or "" if no cover could be produced (e.g. a
// text-only PDF). It does not touch the database; the caller stores the name.
func GenerateCover(reg *formats.Registry, coverDir string, thumbMax int, b *core.Book) (string, error) {
	if coverDir == "" {
		return "", nil
	}
	if thumbMax <= 0 {
		thumbMax = DefaultThumbnailMax
	}
	for _, f := range b.Files {
		h, ok := reg.HandlerFor(f.Path)
		if !ok {
			continue
		}
		raw, err := h.Cover(f.Path)
		if err != nil || len(raw) == 0 {
			continue
		}
		thumb, err := formats.Thumbnail(raw, thumbMax)
		if err != nil {
			continue
		}
		if err := os.MkdirAll(coverDir, 0o755); err != nil {
			return "", err
		}
		name := strconv.FormatInt(b.ID, 10) + ".jpg"
		if err := os.WriteFile(filepath.Join(coverDir, name), thumb, 0o644); err != nil {
			return "", err
		}
		return name, nil
	}
	return "", nil
}

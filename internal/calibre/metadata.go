package calibre

import (
	"strconv"

	"github.com/agrison/reliure/internal/core"
)

// bookMetadata builds the calibre metadata dictionary sent with SEND_BOOK.
// KOReader stores a subset (title, authors, series, tags, lpath, uuid, size);
// we send a reasonable superset. lpath is the on-device path (may contain
// subfolders) and size is the file's byte length.
func bookMetadata(b *core.Book, lpath string, size int64) map[string]any {
	md := map[string]any{
		"title":        b.Title,
		"authors":      authorsOrUnknown(b),
		"lpath":        lpath,
		"uuid":         bookUUID(b),
		"size":         size,
		"tags":         tagNames(b),
		"languages":    languages(b),
		"comments":     b.Description,
		"publisher":    "",
		"series":       seriesName(b),
		"series_index": seriesIndex(b),
		"title_sort":   b.TitleSort,
		"author_sort":  authorSort(b),
	}
	if !b.UpdatedAt.IsZero() {
		md["last_modified"] = b.UpdatedAt.UTC().Format("2006-01-02T15:04:05+00:00")
	}
	if b.PublishedAt != "" {
		md["pubdate"] = b.PublishedAt
	}
	return md
}

// fileMetadata builds a minimal metadata dictionary for non-book files such as
// Reliure's device inventory manifest.
func fileMetadata(title, lpath string, size int64) map[string]any {
	return map[string]any{
		"title":     title,
		"authors":   []string{"Reliure"},
		"lpath":     lpath,
		"uuid":      "reliure-inventory",
		"size":      size,
		"tags":      []string{"Reliure"},
		"languages": []string{},
		"comments":  "Reliure device inventory",
	}
}

func authorsOrUnknown(b *core.Book) []string {
	names := b.AuthorNames()
	if len(names) == 0 {
		return []string{"Unknown"}
	}
	return names
}

func authorSort(b *core.Book) string {
	for _, c := range b.Authors {
		if c.Author.SortName != "" {
			return c.Author.SortName
		}
	}
	if names := b.AuthorNames(); len(names) > 0 {
		return names[0]
	}
	return ""
}

func tagNames(b *core.Book) []string {
	names := make([]string, 0, len(b.Tags))
	for _, t := range b.Tags {
		names = append(names, t.Name)
	}
	return names
}

func languages(b *core.Book) []string {
	if b.Language == "" {
		return []string{}
	}
	return []string{b.Language}
}

func seriesName(b *core.Book) string {
	if b.Series == nil {
		return ""
	}
	return b.Series.Name
}

func seriesIndex(b *core.Book) any {
	if b.SeriesIndex == nil {
		return nil
	}
	return *b.SeriesIndex
}

// bookUUID returns a stable identifier for the book: its ISBN when available,
// otherwise a synthetic one derived from the local id.
func bookUUID(b *core.Book) string {
	if b.ISBN != "" {
		return b.ISBN
	}
	return "reliure-" + strconv.FormatInt(b.ID, 10)
}

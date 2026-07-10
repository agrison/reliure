package library

import (
	"github.com/agrison/reliure/internal/core"
	"github.com/agrison/reliure/internal/formats"
)

// WriteFileMetadata writes a book's current metadata into each of its files
// whose format handler supports writing (via formats.MetadataWriter). Formats
// that can't write are skipped. Best-effort: the first write error is returned,
// but earlier files may already have been updated.
//
// This modifies files on disk — in reference mode it edits the user's own
// originals — so callers gate it behind an explicit user preference.
func WriteFileMetadata(reg *formats.Registry, b *core.Book) error {
	md := bookToMetadata(b)
	for _, f := range b.Files {
		h, ok := reg.HandlerFor(f.Path)
		if !ok {
			continue
		}
		writer, ok := h.(formats.MetadataWriter)
		if !ok {
			continue
		}
		if err := writer.WriteMetadata(f.Path, md); err != nil {
			return err
		}
	}
	return nil
}

// bookToMetadata maps a core.Book onto the format-neutral BookMetadata used by
// the writers (the inverse of metadataToBook).
func bookToMetadata(b *core.Book) formats.BookMetadata {
	md := formats.BookMetadata{
		Title:       b.Title,
		TitleSort:   b.TitleSort,
		Language:    b.Language,
		Description: b.Description,
		ISBN:        b.ISBN,
		Published:   b.PublishedAt,
		SeriesIndex: b.SeriesIndex,
	}
	if b.Series != nil {
		md.Series = b.Series.Name
	}
	for _, c := range b.Authors {
		md.Contributors = append(md.Contributors, formats.Contributor{
			Name:     c.Author.Name,
			SortName: c.Author.SortName,
			Role:     c.Role,
		})
	}
	for _, t := range b.Tags {
		md.Tags = append(md.Tags, t.Name)
	}
	return md
}

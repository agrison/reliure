package library

import (
	"testing"

	"github.com/agrison/reliure/internal/core"
	"github.com/agrison/reliure/internal/formats"
)

func TestWriteFileMetadataUpdatesEpub(t *testing.T) {
	// A real EPUB carrying the original metadata.
	path := makeEPUB(t, "", "book.epub", "Old Title", "Old Author", "Old Series", "x", nil)

	idx := 3.0
	b := &core.Book{
		Title:     "New Title",
		TitleSort: "Title, New",
		Language:  "en",
		Authors: []core.Contribution{
			{Author: core.Author{Name: "New Author", SortName: "Author, New"}, Role: "aut"},
		},
		Series:      &core.Series{Name: "New Series"},
		SeriesIndex: &idx,
		Tags:        []core.Tag{{Name: "sci-fi"}},
		Files:       []core.File{{Path: path, Format: "epub"}},
	}

	if err := WriteFileMetadata(formats.Default, b); err != nil {
		t.Fatalf("WriteFileMetadata: %v", err)
	}

	// Re-read the file through the handler: the on-disk metadata is updated.
	h, ok := formats.Default.HandlerFor(path)
	if !ok {
		t.Fatal("no handler for epub")
	}
	md, err := h.Metadata(path)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if md.Title != "New Title" || md.TitleSort != "Title, New" {
		t.Errorf("title/sort not written: %+v", md)
	}
	if len(md.Contributors) != 1 || md.Contributors[0].Name != "New Author" ||
		md.Contributors[0].SortName != "Author, New" {
		t.Errorf("author not written: %+v", md.Contributors)
	}
	if md.Series != "New Series" || md.SeriesIndex == nil || *md.SeriesIndex != 3 {
		t.Errorf("series not written: %q %v", md.Series, md.SeriesIndex)
	}
	if len(md.Tags) != 1 || md.Tags[0] != "sci-fi" {
		t.Errorf("tags not written: %v", md.Tags)
	}
}

func TestWriteFileMetadataSkipsUnknownFormat(t *testing.T) {
	// A file with no registered handler must be skipped, not error.
	b := &core.Book{
		Title: "X",
		Files: []core.File{{Path: "/tmp/whatever.xyz", Format: "xyz"}},
	}
	if err := WriteFileMetadata(formats.Default, b); err != nil {
		t.Errorf("unknown format should be skipped, got %v", err)
	}
}

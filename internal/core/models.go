// Package core holds the domain models, SQLite access (migrations,
// repositories) and search. It depends on no UI framework: this is the
// reusable heart of the application (the Wails binary and a future headless
// cmd/cli are just entry points on top of it).
package core

import "time"

// Author is a contributor (author, editor, translator…). The actual role of a
// contribution is carried by the BookAuthor link, not by the author itself.
type Author struct {
	ID       int64
	Name     string
	SortName string // "Last, First" for sorting; empty → derived from Name
}

// Series is a series/saga. The per-book position lives on Book.SeriesIndex.
type Series struct {
	ID       int64
	Name     string
	SortName string
}

// Tag is a free-form label.
type Tag struct {
	ID   int64
	Name string
}

// File is a concrete file backing a book (one book → many formats).
type File struct {
	ID      int64
	BookID  int64
	Path    string
	Format  string // "epub", "pdf"…
	Size    int64
	SHA256  string
	AddedAt time.Time
}

// Contribution links an author to a book with a role and display position.
type Contribution struct {
	Author   Author
	Role     string // MARC relator code: "aut", "edt", "trl"…
	Position int
}

// Book is the central entity. Relational fields (Authors, Tags, Files, Series)
// are loaded on demand by the repositories; they may be nil on a Book returned
// by a "light" method.
type Book struct {
	ID          int64
	Title       string
	TitleSort   string
	Description string
	Language    string
	ISBN        string
	PublishedAt string // free-form text (ISO 8601 when known), not always parseable
	SeriesIndex *float64
	CoverPath   string
	// RemotePathOverride is used for KOReader sends only when
	// RemotePathOverrideEnabled is true.
	RemotePathOverrideEnabled bool
	RemotePathOverride        string
	AddedAt                   time.Time
	UpdatedAt                 time.Time

	Series  *Series
	Authors []Contribution
	Tags    []Tag
	Files   []File
}

// AuthorNames returns the author names in position order.
func (b *Book) AuthorNames() []string {
	names := make([]string, 0, len(b.Authors))
	for _, c := range b.Authors {
		names = append(names, c.Author.Name)
	}
	return names
}

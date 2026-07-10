package main

import (
	"strings"

	"github.com/agrison/reliure/internal/core"
)

// coverURLPrefix is where the custom asset handler serves cached thumbnails.
const coverURLPrefix = "/covers/"

// BookCard is the compact shape shown in the grid/list. Covers are referenced
// by URL (served by the asset handler), never inlined as base64.
type BookCard struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Authors     string   `json:"authors"`
	Series      string   `json:"series"`
	SeriesIndex float64  `json:"seriesIndex"`
	Cover       string   `json:"cover"`
	Formats     []string `json:"formats"`
}

// Contributor is a named author/role pair for the detail view.
type Contributor struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

// FileInfo describes one file backing a book.
type FileInfo struct {
	Format string `json:"format"`
	Size   int64  `json:"size"`
	Path   string `json:"path"`
}

// BookDetail is the full metadata for the detail view.
type BookDetail struct {
	ID          int64         `json:"id"`
	Title       string        `json:"title"`
	Authors     []Contributor `json:"authors"`
	Series      string        `json:"series"`
	SeriesIndex float64       `json:"seriesIndex"`
	Description string        `json:"description"`
	Language    string        `json:"language"`
	ISBN        string        `json:"isbn"`
	Published   string        `json:"published"`
	Tags        []string      `json:"tags"`
	Cover       string        `json:"cover"`
	Files       []FileInfo    `json:"files"`
}

// SidebarItem is a named entry (author/series/tag) with a book count.
type SidebarItem struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Books returns all books in the given sort order ("title", "author", "added").
func (s *LibraryService) Books(sort string) ([]BookCard, error) {
	books, err := s.db.Books.Browse(sort, 0, 0)
	if err != nil {
		return nil, err
	}
	return cards(books), nil
}

// Search returns books matching a full-text query, ranked by relevance.
func (s *LibraryService) Search(query string) ([]BookCard, error) {
	books, err := s.db.Books.Search(query, 200)
	if err != nil {
		return nil, err
	}
	return cards(books), nil
}

// BooksByAuthor returns an author's books.
func (s *LibraryService) BooksByAuthor(id int64) ([]BookCard, error) {
	books, err := s.db.Books.ListByAuthor(id)
	if err != nil {
		return nil, err
	}
	return cards(books), nil
}

// BooksBySeries returns a series' books, in reading order.
func (s *LibraryService) BooksBySeries(id int64) ([]BookCard, error) {
	books, err := s.db.Books.ListBySeries(id)
	if err != nil {
		return nil, err
	}
	return cards(books), nil
}

// BooksByTag returns the books carrying a tag.
func (s *LibraryService) BooksByTag(id int64) ([]BookCard, error) {
	books, err := s.db.Books.ListByTag(id)
	if err != nil {
		return nil, err
	}
	return cards(books), nil
}

// Authors returns the sidebar's author list with counts.
func (s *LibraryService) Authors() ([]SidebarItem, error) {
	return sidebar(s.db.Authors.Counts())
}

// SeriesList returns the sidebar's series list with counts.
func (s *LibraryService) SeriesList() ([]SidebarItem, error) {
	return sidebar(s.db.Series.Counts())
}

// Tags returns the sidebar's tag list with counts.
func (s *LibraryService) Tags() ([]SidebarItem, error) {
	return sidebar(s.db.Tags.Counts())
}

// Book returns the full detail for one book.
func (s *LibraryService) Book(id int64) (BookDetail, error) {
	b, err := s.db.Books.ByID(id)
	if err != nil {
		return BookDetail{}, err
	}
	d := BookDetail{
		ID:          b.ID,
		Title:       b.Title,
		Series:      seriesName(b),
		SeriesIndex: seriesIndex(b),
		Description: b.Description,
		Language:    b.Language,
		ISBN:        b.ISBN,
		Published:   b.PublishedAt,
		Cover:       coverURL(b),
	}
	for _, c := range b.Authors {
		d.Authors = append(d.Authors, Contributor{Name: c.Author.Name, Role: c.Role})
	}
	for _, t := range b.Tags {
		d.Tags = append(d.Tags, t.Name)
	}
	for _, f := range b.Files {
		d.Files = append(d.Files, FileInfo{Format: f.Format, Size: f.Size, Path: f.Path})
	}
	return d, nil
}

// --- mapping helpers ---

func cards(books []*core.Book) []BookCard {
	out := make([]BookCard, 0, len(books))
	for _, b := range books {
		formats := make([]string, 0, len(b.Files))
		for _, f := range b.Files {
			formats = append(formats, f.Format)
		}
		out = append(out, BookCard{
			ID:          b.ID,
			Title:       b.Title,
			Authors:     strings.Join(b.AuthorNames(), ", "),
			Series:      seriesName(b),
			SeriesIndex: seriesIndex(b),
			Cover:       coverURL(b),
			Formats:     formats,
		})
	}
	return out
}

func sidebar(items []core.NamedCount, err error) ([]SidebarItem, error) {
	if err != nil {
		return nil, err
	}
	out := make([]SidebarItem, 0, len(items))
	for _, it := range items {
		out = append(out, SidebarItem{ID: it.ID, Name: it.Name, Count: it.Count})
	}
	return out, nil
}

func coverURL(b *core.Book) string {
	if b.CoverPath == "" {
		return ""
	}
	return coverURLPrefix + b.CoverPath
}

func seriesName(b *core.Book) string {
	if b.Series == nil {
		return ""
	}
	return b.Series.Name
}

func seriesIndex(b *core.Book) float64 {
	if b.SeriesIndex == nil {
		return 0
	}
	return *b.SeriesIndex
}

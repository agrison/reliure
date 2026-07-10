package opds

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agrison/reliure/internal/core"
)

type fakeCatalog struct {
	books []*core.Book
}

func (c fakeCatalog) Recent(context.Context, int) ([]*core.Book, error) {
	return c.books, nil
}

func (c fakeCatalog) Search(context.Context, string, int) ([]*core.Book, error) {
	return c.books, nil
}

func (c fakeCatalog) Book(_ context.Context, id int64) (*core.Book, error) {
	for _, b := range c.books {
		if b.ID == id {
			return b, nil
		}
	}
	return nil, os.ErrNotExist
}

func (c fakeCatalog) Authors(context.Context) ([]NamedCount, error) {
	return []NamedCount{{ID: 10, Name: "Ursula K. Le Guin", Count: 1}}, nil
}

func (c fakeCatalog) BooksByAuthor(context.Context, int64) ([]*core.Book, error) {
	return c.books, nil
}

func (c fakeCatalog) Series(context.Context) ([]NamedCount, error) {
	return []NamedCount{{ID: 20, Name: "Ekumen", Count: 1}}, nil
}

func (c fakeCatalog) BooksBySeries(context.Context, int64) ([]*core.Book, error) {
	return c.books, nil
}

func TestRootNavigationFeed(t *testing.T) {
	h := testHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://reliure.local/", nil)

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{">Ajouts récents<", "href=\"http://reliure.local/authors\"", "opensearch.xml"} {
		if !strings.Contains(body, want) {
			t.Fatalf("root feed missing %q:\n%s", want, body)
		}
	}
}

func TestRecentFeedExposesAcquisitionAndCover(t *testing.T) {
	h := testHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://reliure.local/recent", nil)

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		">La Main gauche de la nuit<",
		"type=\"application/epub+zip\"",
		"href=\"http://reliure.local/books/1/files/7\"",
		"href=\"http://reliure.local/covers/1.jpg\"",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("recent feed missing %q:\n%s", want, body)
		}
	}
}

func TestDownloadServesEPUB(t *testing.T) {
	h := testHandler(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://reliure.local/books/1/files/7", nil)

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, epubType) {
		t.Fatalf("content-type = %q", ct)
	}
	if got := rec.Body.String(); got != "epub bytes" {
		t.Fatalf("body = %q", got)
	}
}

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	bookPath := filepath.Join(dir, "book.epub")
	if err := os.WriteFile(bookPath, []byte("epub bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	coverDir := filepath.Join(dir, "covers")
	if err := os.MkdirAll(coverDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coverDir, "1.jpg"), []byte("jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := 4.0
	book := &core.Book{
		ID:          1,
		Title:       "La Main gauche de la nuit",
		Description: "Roman de science-fiction.",
		UpdatedAt:   time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
		CoverPath:   "1.jpg",
		Series:      &core.Series{ID: 20, Name: "Ekumen"},
		SeriesIndex: &idx,
		Authors: []core.Contribution{{
			Author: core.Author{ID: 10, Name: "Ursula K. Le Guin"},
			Role:   "aut",
		}},
		Files: []core.File{{
			ID:     7,
			BookID: 1,
			Path:   bookPath,
			Format: "epub",
			Size:   10,
		}},
	}
	return NewHandler(HandlerConfig{
		Catalog:  fakeCatalog{books: []*core.Book{book}},
		CoverDir: coverDir,
		Title:    "Reliure",
		Now:      func() time.Time { return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC) },
	})
}

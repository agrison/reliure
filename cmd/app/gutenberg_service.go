package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/agrison/reliure/internal/gutenberg"
	"github.com/agrison/reliure/internal/library"
)

// maxGutenbergEPUB caps a downloaded EPUB so a misbehaving mirror can't fill the
// disk; real Gutenberg EPUBs are a few MB at most.
const maxGutenbergEPUB = 100 << 20

// GutenbergBook is the JSON-friendly shape of a catalogue entry for the
// "Découvrir" view.
type GutenbergBook struct {
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	Authors   []string `json:"authors"`
	Languages []string `json:"languages"`
	Subjects  []string `json:"subjects"`
	Cover     string   `json:"cover"`
	HasEpub   bool     `json:"hasEpub"`
}

// GutenbergResult is a page of catalogue results.
type GutenbergResult struct {
	Count       int             `json:"count"`
	HasNext     bool            `json:"hasNext"`
	HasPrevious bool            `json:"hasPrevious"`
	Page        int             `json:"page"`
	Books       []GutenbergBook `json:"books"`
}

// SearchGutenberg browses the Project Gutenberg catalogue (via Gutendex).
// languages is a comma-separated list of ISO 639-1 codes (e.g. "fr" or "fr,en").
func (s *LibraryService) SearchGutenberg(search, languages string, page int) (GutenbergResult, error) {
	// Generous deadline only to cover the one-time catalogue CSV download on the
	// first search (~21 MB); every subsequent search is served from memory and
	// returns instantly.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	res, err := s.gutenberg.Search(ctx, gutenberg.Query{
		Search:    search,
		Languages: splitQuickList(languages),
		Page:      page,
	})
	if err != nil {
		return GutenbergResult{}, err
	}
	out := GutenbergResult{
		Count:       res.Count,
		HasNext:     res.HasNext,
		HasPrevious: res.HasPrev,
		Page:        res.Page,
	}
	for _, b := range res.Books {
		out.Books = append(out.Books, GutenbergBook{
			ID:        b.ID,
			Title:     b.Title,
			Authors:   b.Authors,
			Languages: b.Languages,
			Subjects:  b.Subjects,
			Cover:     b.CoverURL,
			HasEpub:   b.EPUBURL != "",
		})
	}
	slog.Info("gutenberg search", "query", search, "languages", languages, "page", out.Page, "results", len(out.Books))
	return out, nil
}

// ImportGutenbergBook downloads a catalogue book's EPUB and imports it into the
// library. It re-resolves the download URL from Gutendex by id (rather than
// trusting a client-supplied URL) and always copies into the managed library,
// since the download lives in a throwaway temp file. Progress and the summary
// flow through the normal import events, so the UI refreshes as usual.
func (s *LibraryService) ImportGutenbergBook(id int) (ImportSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	book, ok, err := s.gutenberg.Book(ctx, id)
	if err != nil {
		return ImportSummary{}, err
	}
	if !ok {
		return ImportSummary{}, fmt.Errorf("livre Gutenberg %d introuvable", id)
	}

	path, err := s.downloadToTemp(ctx, book)
	if err != nil {
		return ImportSummary{}, err
	}
	defer os.Remove(path) // copied into the library by ModeCopy import

	slog.Info("gutenberg import", "id", id, "title", book.Title, "from", book.EPUBURL)
	return s.importPathsMode([]string{path}, library.ModeCopy)
}

// downloadToTemp streams a book's EPUB into a temporary .epub file and returns
// its path.
func (s *LibraryService) downloadToTemp(ctx context.Context, book gutenberg.Book) (string, error) {
	body, err := s.gutenberg.Download(ctx, book.ID)
	if err != nil {
		return "", err
	}
	defer body.Close()

	f, err := os.CreateTemp("", fmt.Sprintf("gutenberg-%d-*.epub", book.ID))
	if err != nil {
		return "", err
	}
	path := f.Name()
	n, err := io.Copy(f, io.LimitReader(body, maxGutenbergEPUB))
	closeErr := f.Close()
	if err != nil {
		os.Remove(path)
		return "", err
	}
	if closeErr != nil {
		os.Remove(path)
		return "", closeErr
	}
	if n == 0 {
		os.Remove(path)
		return "", fmt.Errorf("téléchargement vide pour « %s »", strings.TrimSpace(book.Title))
	}
	return path, nil
}

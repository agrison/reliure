package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agrison/reliure/internal/gutenberg"
	"github.com/agrison/reliure/internal/library"
	"github.com/agrison/reliure/internal/standardebooks"
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

// DiscoverBook is the source-agnostic shape used by the "Découvrir" view.
type DiscoverBook struct {
	Source    string   `json:"source"`
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Authors   []string `json:"authors"`
	Languages []string `json:"languages"`
	Subjects  []string `json:"subjects"`
	Cover     string   `json:"cover"`
	HasEpub   bool     `json:"hasEpub"`
}

// DiscoverResult is a page of discovery results across one or more providers.
type DiscoverResult struct {
	Count       int            `json:"count"`
	HasNext     bool           `json:"hasNext"`
	HasPrevious bool           `json:"hasPrevious"`
	Page        int            `json:"page"`
	Books       []DiscoverBook `json:"books"`
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

// SearchDiscover browses legal ebook providers from the single discovery view.
// source may be "all", "gutenberg" or "standardebooks".
func (s *LibraryService) SearchDiscover(source, search, languages string, page int) (DiscoverResult, error) {
	if page < 1 {
		page = 1
	}
	source = normalizeDiscoverSource(source)
	langs := splitQuickList(languages)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var out DiscoverResult
	out.Page = page
	if source == "all" || source == "gutenberg" {
		res, err := s.gutenberg.Search(ctx, gutenberg.Query{
			Search:    search,
			Languages: langs,
			Page:      page,
		})
		if err != nil {
			return DiscoverResult{}, err
		}
		out.Count += res.Count
		out.HasNext = out.HasNext || res.HasNext
		out.HasPrevious = out.HasPrevious || res.HasPrev
		for _, b := range res.Books {
			out.Books = append(out.Books, discoverFromGutenberg(b))
		}
	}
	if source == "all" || source == "standardebooks" {
		res, err := s.standard.Search(ctx, standardebooks.Query{
			Search:    search,
			Languages: langs,
			Page:      page,
		})
		if err != nil {
			return DiscoverResult{}, err
		}
		out.Count += res.Count
		out.HasNext = out.HasNext || res.HasNext
		out.HasPrevious = out.HasPrevious || res.HasPrev
		for _, b := range res.Books {
			out.Books = append(out.Books, discoverFromStandard(b))
		}
	}
	slog.Info("discover search", "source", source, "query", search, "languages", languages, "page", out.Page, "results", len(out.Books))
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

	body, err := s.gutenberg.Download(ctx, book.ID)
	if err != nil {
		return ImportSummary{}, err
	}
	path, err := downloadEPUBToTemp(body, fmt.Sprintf("gutenberg-%d-*.epub", book.ID), book.Title)
	if err != nil {
		return ImportSummary{}, err
	}
	defer os.Remove(path) // copied into the library by ModeCopy import

	slog.Info("gutenberg import", "id", id, "title", book.Title, "from", book.EPUBURL)
	return s.importPathsMode([]string{path}, library.ModeCopy)
}

// ImportDiscoverBook downloads a book from a supported discovery provider and
// imports the EPUB through the normal library pipeline.
func (s *LibraryService) ImportDiscoverBook(source, id string) (ImportSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	switch normalizeDiscoverSource(source) {
	case "gutenberg":
		n, err := parsePositiveInt(id)
		if err != nil {
			return ImportSummary{}, err
		}
		return s.ImportGutenbergBook(n)
	case "standardebooks":
		book, ok, err := s.standard.Book(ctx, id)
		if err != nil {
			return ImportSummary{}, err
		}
		if !ok {
			return ImportSummary{}, fmt.Errorf("livre Standard Ebooks %q introuvable", id)
		}
		body, err := s.standard.Download(ctx, id)
		if err != nil {
			return ImportSummary{}, err
		}
		path, err := downloadEPUBToTemp(body, "standardebooks-*.epub", book.Title)
		if err != nil {
			return ImportSummary{}, err
		}
		defer os.Remove(path)
		slog.Info("standardebooks import", "id", id, "title", book.Title, "from", book.EPUBURL)
		return s.importPathsMode([]string{path}, library.ModeCopy)
	default:
		return ImportSummary{}, fmt.Errorf("source de découverte inconnue: %s", source)
	}
}

// downloadEPUBToTemp streams an EPUB into a temporary file and returns its path.
func downloadEPUBToTemp(body io.ReadCloser, pattern, title string) (string, error) {
	defer body.Close()
	f, err := os.CreateTemp("", pattern)
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
		return "", fmt.Errorf("téléchargement vide pour « %s »", strings.TrimSpace(title))
	}
	return path, nil
}

func discoverFromGutenberg(b gutenberg.Book) DiscoverBook {
	return DiscoverBook{
		Source:    "gutenberg",
		ID:        fmt.Sprintf("%d", b.ID),
		Title:     b.Title,
		Authors:   b.Authors,
		Languages: b.Languages,
		Subjects:  b.Subjects,
		Cover:     b.CoverURL,
		HasEpub:   b.EPUBURL != "",
	}
}

func discoverFromStandard(b standardebooks.Book) DiscoverBook {
	return DiscoverBook{
		Source:    "standardebooks",
		ID:        b.ID,
		Title:     b.Title,
		Authors:   b.Authors,
		Languages: b.Languages,
		Subjects:  b.Subjects,
		Cover:     b.CoverURL,
		HasEpub:   b.EPUBURL != "",
	}
}

func normalizeDiscoverSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", "all":
		return "all"
	case "gutenberg":
		return "gutenberg"
	case "standard", "standardebooks", "standard-ebooks":
		return "standardebooks"
	default:
		return source
	}
}

func parsePositiveInt(raw string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("identifiant Gutenberg invalide: %s", raw)
	}
	return n, nil
}

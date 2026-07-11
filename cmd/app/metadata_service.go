package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agrison/reliure/internal/formats"
	"github.com/agrison/reliure/internal/library"
	"github.com/agrison/reliure/internal/metadata"
)

// OnlineCandidate is one online edition offered to the user, JSON-friendly. It
// carries every field so the frontend can let the user cherry-pick and edit
// each one before applying (e.g. keep the French description, drop an author in
// "Last, First" form).
type OnlineCandidate struct {
	ID          string   `json:"id"`
	Source      string   `json:"source"`
	Title       string   `json:"title"`
	Subtitle    string   `json:"subtitle"`
	Authors     []string `json:"authors"`
	Publisher   string   `json:"publisher"`
	Published   string   `json:"published"`
	Description string   `json:"description"`
	Language    string   `json:"language"`
	ISBN        string   `json:"isbn"`
	Series      string   `json:"series"`
	SeriesIndex string   `json:"seriesIndex"`
	Tags        []string `json:"tags"`
	CoverURL    string   `json:"coverUrl"`
	PageCount   int      `json:"pageCount"`
}

// OnlineSearchResult wraps the ranked candidates plus the effective query, so
// the UI can show what it actually searched for.
type OnlineSearchResult struct {
	Query      string            `json:"query"`
	Candidates []OnlineCandidate `json:"candidates"`
}

// ApplyMetadataInput carries the user's field-by-field choices as a ready-made
// BookUpdate, plus an optional cover URL to download and apply.
type ApplyMetadataInput struct {
	Book     BookUpdate `json:"book"`
	CoverURL string     `json:"coverUrl"`
}

// SearchOnlineMetadata queries the online providers for editions matching the
// given hints. Empty hints fall back to the book's own metadata, so the UI can
// open and search immediately. The language hint only ranks results (preferred
// language first); it never hides other editions, so the user keeps the choice.
func (s *LibraryService) SearchOnlineMetadata(bookID int64, title, authors, isbn, language string) (OnlineSearchResult, error) {
	q := metadata.Query{
		Title:    strings.TrimSpace(title),
		Authors:  splitQuickList(authors),
		ISBN:     strings.TrimSpace(isbn),
		Language: strings.TrimSpace(language),
		Max:      12,
	}
	// Fall back to the stored book when the caller gave nothing to search on.
	if q.Title == "" && q.ISBN == "" && bookID > 0 {
		if b, err := s.db.Books.ByID(bookID); err == nil {
			q.Title = b.Title
			q.Authors = b.AuthorNames()
			q.ISBN = b.ISBN
			if q.Language == "" {
				q.Language = b.Language
			}
		}
	}
	if q.Title == "" && q.ISBN == "" {
		return OnlineSearchResult{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cands, err := s.meta.Search(ctx, q)
	if err != nil {
		return OnlineSearchResult{}, err
	}

	out := OnlineSearchResult{Query: onlineQueryLabel(q)}
	for _, c := range cands {
		out.Candidates = append(out.Candidates, toOnlineCandidate(c))
	}
	slog.Info("online metadata search", "book", bookID, "query", out.Query, "results", len(out.Candidates))
	return out, nil
}

// ApplyOnlineMetadata persists the user's chosen fields (already merged into a
// BookUpdate by the frontend) and, if a cover URL was chosen, downloads and
// installs that cover. Cover failure is non-fatal: the metadata is the point,
// so it is logged and the (metadata-updated) detail is still returned.
func (s *LibraryService) ApplyOnlineMetadata(in ApplyMetadataInput) (BookDetail, error) {
	detail, err := s.UpdateBook(in.Book)
	if err != nil {
		return BookDetail{}, err
	}
	if url := strings.TrimSpace(in.CoverURL); url != "" {
		if err := s.applyCoverFromURL(in.Book.ID, url); err != nil {
			slog.Warn("apply online cover failed", "book", in.Book.ID, "err", err)
		} else if fresh, err := s.Book(in.Book.ID); err == nil {
			detail = fresh
		}
	}
	return detail, nil
}

// applyCoverFromURL downloads an image, thumbnails it like an imported cover and
// stores it as the book's cover, replacing any existing one.
func (s *LibraryService) applyCoverFromURL(bookID int64, url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	raw, err := s.meta.FetchImage(ctx, url, 12<<20)
	if err != nil {
		return err
	}
	thumb, err := formats.Thumbnail(raw, library.DefaultThumbnailMax)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.coverDir, 0o755); err != nil {
		return err
	}
	name := strconv.FormatInt(bookID, 10) + ".jpg"
	if err := os.WriteFile(filepath.Join(s.coverDir, name), thumb, 0o644); err != nil {
		return err
	}
	return s.db.Books.SetCover(bookID, name)
}

func toOnlineCandidate(c metadata.Candidate) OnlineCandidate {
	idx := ""
	if c.SeriesIndex != nil {
		idx = strconv.FormatFloat(*c.SeriesIndex, 'f', -1, 64)
	}
	return OnlineCandidate{
		ID:          c.Source + ":" + c.SourceID,
		Source:      c.Source,
		Title:       c.Title,
		Subtitle:    c.Subtitle,
		Authors:     c.Authors,
		Publisher:   c.Publisher,
		Published:   c.Published,
		Description: c.Description,
		Language:    c.Language,
		ISBN:        c.ISBN,
		Series:      c.Series,
		SeriesIndex: idx,
		Tags:        c.Tags,
		CoverURL:    c.CoverURL,
		PageCount:   c.PageCount,
	}
}

func onlineQueryLabel(q metadata.Query) string {
	if q.ISBN != "" {
		return "ISBN " + q.ISBN
	}
	label := q.Title
	if len(q.Authors) > 0 && q.Authors[0] != "" {
		label += " — " + q.Authors[0]
	}
	return label
}

package main

import (
	"errors"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agrison/reliure/internal/core"
	"github.com/agrison/reliure/internal/koreader"
	"github.com/agrison/reliure/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// KOReaderService mirrors reading progress and annotations from a KOReader
// library into Reliure. It reads the reader's `.sdr` sidecars from a folder the
// user points at — a USB-mounted device, or a synced copy of its library.
type KOReaderService struct {
	db    *core.DB
	store *settings.Store
}

// ReadingCard is the compact per-book reading state for grid overlays.
type ReadingCard struct {
	BookID      int64   `json:"bookId"`
	Percent     float64 `json:"percent"`
	Pages       int     `json:"pages"`
	Status      string  `json:"status"`
	Rating      int     `json:"rating"` // 1..5 star rating (0 = unrated)
	Annotations int     `json:"annotations"`
}

// ReadingStatusCounts is the number of books per reading status, for the sidebar.
type ReadingStatusCounts struct {
	Reading   int `json:"reading"`
	Complete  int `json:"complete"`
	Abandoned int `json:"abandoned"`
}

// AnnotatedBook groups a book's annotations for the "Annotations" view.
type AnnotatedBook struct {
	BookID      int64        `json:"bookId"`
	Title       string       `json:"title"`
	Authors     string       `json:"authors"`
	Cover       string       `json:"cover"`
	Annotations []Annotation `json:"annotations"`
}

// KoreaderSyncResult summarizes a sync run.
type KoreaderSyncResult struct {
	Dir          string `json:"dir"`
	Scanned      int    `json:"scanned"`
	Matched      int    `json:"matched"`
	Unmatched    int    `json:"unmatched"`
	WithProgress int    `json:"withProgress"`
	Annotations  int    `json:"annotations"`
}

// ChooseFolderAndSync opens a native directory picker (rooted at the last folder
// used), remembers the choice and syncs from it.
func (s *KOReaderService) ChooseFolderAndSync() (KoreaderSyncResult, error) {
	dialog := application.Get().Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		SetTitle("Choisir la bibliothèque KOReader (dossier contenant les .sdr)")
	if dir := s.store.Get().KoreaderSyncDir; dir != "" {
		dialog.SetDirectory(dir)
	}
	dir, err := dialog.PromptForSingleSelection()
	if err != nil || dir == "" {
		return KoreaderSyncResult{}, err
	}
	cur := s.store.Get()
	cur.KoreaderSyncDir = dir
	if _, err := s.store.Update(cur); err != nil {
		return KoreaderSyncResult{}, err
	}
	return s.sync(dir)
}

// Sync re-scans the previously chosen folder.
func (s *KOReaderService) Sync() (KoreaderSyncResult, error) {
	return s.sync(s.store.Get().KoreaderSyncDir)
}

// ReadingStates returns every book's reading state for grid badges/progress.
func (s *KOReaderService) ReadingStates() ([]ReadingCard, error) {
	states, err := s.db.Reading.AllStates()
	if err != nil {
		return nil, err
	}
	counts, err := s.db.Reading.AnnotationCounts()
	if err != nil {
		return nil, err
	}
	out := make([]ReadingCard, 0, len(states))
	for id, st := range states {
		out = append(out, ReadingCard{BookID: id, Percent: st.Percent, Pages: st.Pages, Status: st.Status, Rating: st.Rating, Annotations: counts[id]})
	}
	// Books with annotations but no progress row still deserve a badge.
	for id, n := range counts {
		if _, ok := states[id]; !ok {
			out = append(out, ReadingCard{BookID: id, Annotations: n})
		}
	}
	return out, nil
}

// StatusCounts returns how many books sit in each reading status, for the
// sidebar's reading filters.
func (s *KOReaderService) StatusCounts() (ReadingStatusCounts, error) {
	counts, err := s.db.Reading.StatusCounts()
	if err != nil {
		return ReadingStatusCounts{}, err
	}
	return ReadingStatusCounts{
		Reading:   counts["reading"],
		Complete:  counts["complete"],
		Abandoned: counts["abandoned"],
	}, nil
}

// AnnotatedBooks returns every book that has annotations, with its annotations,
// for the dedicated "Annotations" view. Books are ordered by title.
func (s *KOReaderService) AnnotatedBooks() ([]AnnotatedBook, error) {
	counts, err := s.db.Reading.AnnotationCounts()
	if err != nil {
		return nil, err
	}
	out := make([]AnnotatedBook, 0, len(counts))
	for bookID := range counts {
		b, err := s.db.Books.ByID(bookID)
		if err != nil {
			continue // book removed since sync; skip
		}
		anns, err := s.db.Reading.Annotations(bookID)
		if err != nil || len(anns) == 0 {
			continue
		}
		ab := AnnotatedBook{
			BookID:  bookID,
			Title:   b.Title,
			Authors: strings.Join(b.AuthorNames(), ", "),
			Cover:   coverURL(b),
		}
		for _, a := range anns {
			ab.Annotations = append(ab.Annotations, Annotation{
				Text: a.Text, Note: a.Note, Chapter: a.Chapter, Drawer: a.Drawer, CreatedAt: a.CreatedAt,
			})
		}
		out = append(out, ab)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
	})
	return out, nil
}

// sync scans a folder for KOReader sidecars, matches them to library books and
// persists progress + annotations. Sidecars that don't match any book are
// counted but skipped.
func (s *KOReaderService) sync(dir string) (KoreaderSyncResult, error) {
	res := KoreaderSyncResult{Dir: dir}
	if strings.TrimSpace(dir) == "" {
		return res, errors.New("aucun dossier KOReader défini")
	}
	sidecars, errs := koreader.Scan(dir)
	for _, e := range errs {
		slog.Warn("koreader: sidecar parse failed", "err", e)
	}
	res.Scanned = len(sidecars)

	books, err := s.db.Books.List(0, 0)
	if err != nil {
		return res, err
	}
	idx := buildMatchIndex(books)

	// Aggregate per book: several sidecars (e.g. epub + pdf) can map to one book.
	type agg struct {
		percent  float64
		pages    int
		status   string
		modified string
		rating   int
		anns     []core.Annotation
	}
	byBook := map[int64]*agg{}
	for _, sc := range sidecars {
		bookID, ok := idx.match(sc)
		if !ok {
			res.Unmatched++
			continue
		}
		a := byBook[bookID]
		if a == nil {
			a = &agg{}
			byBook[bookID] = a
		}
		if sc.PercentFinished > a.percent {
			a.percent = sc.PercentFinished
		}
		if sc.TotalPages > a.pages {
			a.pages = sc.TotalPages
		}
		a.status = bestStatus(a.status, string(sc.Status))
		if sc.ModifiedAt > a.modified {
			a.modified = sc.ModifiedAt
		}
		if sc.Rating > 0 {
			a.rating = sc.Rating
		}
		for _, an := range sc.Annotations {
			a.anns = append(a.anns, core.Annotation{
				BookID: bookID, Text: an.Text, Note: an.Note,
				Chapter: an.Chapter, Drawer: an.Drawer, CreatedAt: an.Datetime,
			})
		}
	}
	res.Matched = len(byBook)

	for bookID, a := range byBook {
		if err := s.db.Reading.MergeDeviceState(core.ReadingState{
			BookID: bookID, Percent: a.percent, Pages: a.pages, Status: a.status, LastReadAt: a.modified, Rating: a.rating,
		}); err != nil {
			return res, err
		}
		if a.percent > 0 || a.status != "" {
			res.WithProgress++
		}
		if err := s.db.Reading.ReplaceAnnotations(bookID, a.anns); err != nil {
			return res, err
		}
		res.Annotations += len(a.anns)
	}
	slog.Info("koreader sync", "dir", dir, "scanned", res.Scanned, "matched", res.Matched,
		"unmatched", res.Unmatched, "annotations", res.Annotations)
	return res, nil
}

// matchIndex resolves a sidecar to a library book by title+author, then by file
// basename, then by a unique title.
type matchIndex struct {
	byTitleAuthor map[string]int64
	byTitle       map[string][]int64
	byBasename    map[string]int64
}

func buildMatchIndex(books []*core.Book) matchIndex {
	idx := matchIndex{
		byTitleAuthor: map[string]int64{},
		byTitle:       map[string][]int64{},
		byBasename:    map[string]int64{},
	}
	for _, b := range books {
		title := normalizeMatch(b.Title)
		if title != "" {
			author := normalizeMatch(firstAuthorName(b))
			if key := title + "|" + author; author != "" {
				if _, exists := idx.byTitleAuthor[key]; !exists {
					idx.byTitleAuthor[key] = b.ID
				}
			}
			idx.byTitle[title] = append(idx.byTitle[title], b.ID)
		}
		for _, f := range b.Files {
			base := strings.ToLower(filepath.Base(f.Path))
			if base != "" {
				idx.byBasename[base] = b.ID
			}
		}
	}
	return idx
}

func (m matchIndex) match(sc *koreader.Sidecar) (int64, bool) {
	title := normalizeMatch(sc.Title)
	if title != "" {
		if author := normalizeMatch(firstString(sc.Authors)); author != "" {
			if id, ok := m.byTitleAuthor[title+"|"+author]; ok {
				return id, true
			}
		}
	}
	if sc.DocBasename != "" {
		if id, ok := m.byBasename[strings.ToLower(sc.DocBasename)]; ok {
			return id, true
		}
	}
	if title != "" {
		if ids := m.byTitle[title]; len(ids) == 1 {
			return ids[0], true
		}
	}
	return 0, false
}

func firstAuthorName(b *core.Book) string {
	if len(b.Authors) > 0 {
		return b.Authors[0].Author.Name
	}
	return ""
}

func firstString(ss []string) string {
	if len(ss) > 0 {
		return ss[0]
	}
	return ""
}

// normalizeMatch lower-cases and collapses whitespace for tolerant matching.
func normalizeMatch(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// bestStatus keeps the most advanced of two reading statuses.
func bestStatus(a, b string) string {
	if statusRank(b) > statusRank(a) {
		return b
	}
	return a
}

func statusRank(s string) int {
	switch s {
	case "complete":
		return 4
	case "abandoned":
		return 3
	case "reading":
		return 2
	case "new":
		return 1
	default:
		return 0
	}
}

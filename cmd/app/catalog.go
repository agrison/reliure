package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agrison/reliure/internal/core"
	"github.com/agrison/reliure/internal/formats"
	"github.com/agrison/reliure/internal/library"
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
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	SortName string `json:"sortName"`
	Role     string `json:"role"`
}

// FileInfo describes one file backing a book.
type FileInfo struct {
	Format  string `json:"format"`
	Size    int64  `json:"size"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	AddedAt string `json:"addedAt"`
}

// BookDetail is the full metadata for the detail view.
type BookDetail struct {
	ID                        int64         `json:"id"`
	Title                     string        `json:"title"`
	TitleSort                 string        `json:"titleSort"`
	Authors                   []Contributor `json:"authors"`
	Series                    string        `json:"series"`
	SeriesIndex               float64       `json:"seriesIndex"`
	Description               string        `json:"description"`
	Language                  string        `json:"language"`
	ISBN                      string        `json:"isbn"`
	Published                 string        `json:"published"`
	Tags                      []string      `json:"tags"`
	Cover                     string        `json:"cover"`
	Files                     []FileInfo    `json:"files"`
	AddedAt                   string        `json:"addedAt"`
	UpdatedAt                 string        `json:"updatedAt"`
	RemotePathOverrideEnabled bool          `json:"remotePathOverrideEnabled"`
	RemotePathOverride        string        `json:"remotePathOverride"`
	RemotePath                string        `json:"remotePath"`
}

// BookUpdate is the editable metadata payload sent by the frontend. Files are
// intentionally excluded; they are moved only as a consequence of title/author
// changes for managed-library books.
type BookUpdate struct {
	ID                        int64    `json:"id"`
	Title                     string   `json:"title"`
	TitleSort                 string   `json:"titleSort"`
	Authors                   []string `json:"authors"`
	Series                    string   `json:"series"`
	SeriesIndex               string   `json:"seriesIndex"`
	Description               string   `json:"description"`
	Language                  string   `json:"language"`
	ISBN                      string   `json:"isbn"`
	Published                 string   `json:"published"`
	Tags                      []string `json:"tags"`
	RemotePathOverrideEnabled bool     `json:"remotePathOverrideEnabled"`
	RemotePathOverride        string   `json:"remotePathOverride"`
}

// BatchSeriesUpdate assigns one series to several books. When SeriesIndexStart
// is set, books receive consecutive indices in the order provided by IDs.
type BatchSeriesUpdate struct {
	IDs              []int64 `json:"ids"`
	Series           string  `json:"series"`
	SeriesIndexStart string  `json:"seriesIndexStart"`
}

// BatchUpdateResult summarizes a metadata batch operation.
type BatchUpdateResult struct {
	Updated int `json:"updated"`
}

// QuickEditRow is one editable row in the spreadsheet-like metadata editor.
// Multi-value fields are comma- or semicolon-separated strings for ergonomic
// paste/edit workflows.
type QuickEditRow struct {
	ID                        int64  `json:"id"`
	Title                     string `json:"title"`
	TitleSort                 string `json:"titleSort"`
	Authors                   string `json:"authors"`
	Series                    string `json:"series"`
	SeriesIndex               string `json:"seriesIndex"`
	Tags                      string `json:"tags"`
	Language                  string `json:"language"`
	Published                 string `json:"published"`
	ISBN                      string `json:"isbn"`
	RemotePathOverrideEnabled bool   `json:"remotePathOverrideEnabled"`
	RemotePathOverride        string `json:"remotePathOverride"`
	UpdatedAt                 string `json:"updatedAt"`
}

// QuickEditSaveResult summarizes a quick-edit save.
type QuickEditSaveResult struct {
	Updated int                  `json:"updated"`
	Failed  int                  `json:"failed"`
	Errors  []QuickEditRowError  `json:"errors"`
	Rows    []QuickEditSavedBook `json:"rows"`
}

// QuickEditRowError reports a failed row without aborting the whole batch.
type QuickEditRowError struct {
	ID    int64  `json:"id"`
	Error string `json:"error"`
}

// QuickEditSavedBook carries fresh timestamps for rows saved successfully.
type QuickEditSavedBook struct {
	ID        int64  `json:"id"`
	UpdatedAt string `json:"updatedAt"`
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

// BooksWithoutAuthor returns books that have no author link.
func (s *LibraryService) BooksWithoutAuthor() ([]BookCard, error) {
	books, err := s.db.Books.ListWithoutAuthors()
	if err != nil {
		return nil, err
	}
	return cards(books), nil
}

// BooksWithoutSeries returns books that are not attached to a series.
func (s *LibraryService) BooksWithoutSeries() ([]BookCard, error) {
	books, err := s.db.Books.ListWithoutSeries()
	if err != nil {
		return nil, err
	}
	return cards(books), nil
}

// BooksWithoutTag returns books that carry no tag.
func (s *LibraryService) BooksWithoutTag() ([]BookCard, error) {
	books, err := s.db.Books.ListWithoutTags()
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

// AuthorGroups returns author tiles for the grouped library view, including a
// synthetic fallback tile for books without authors.
func (s *LibraryService) AuthorGroups() ([]SidebarItem, error) {
	items, err := sidebar(s.db.Authors.Counts())
	if err != nil {
		return nil, err
	}
	count, err := s.db.Authors.CountMissing()
	return appendMissingGroup(items, "Sans auteur", count, err)
}

// SeriesGroups returns series tiles for the grouped library view, including a
// synthetic fallback tile for books without a series.
func (s *LibraryService) SeriesGroups() ([]SidebarItem, error) {
	items, err := sidebar(s.db.Series.Counts())
	if err != nil {
		return nil, err
	}
	count, err := s.db.Series.CountMissing()
	return appendMissingGroup(items, "Sans série", count, err)
}

// TagGroups returns tag tiles for the grouped library view, including a
// synthetic fallback tile for books without tags.
func (s *LibraryService) TagGroups() ([]SidebarItem, error) {
	items, err := sidebar(s.db.Tags.Counts())
	if err != nil {
		return nil, err
	}
	count, err := s.db.Tags.CountMissing()
	return appendMissingGroup(items, "Sans tag", count, err)
}

// Book returns the full detail for one book.
func (s *LibraryService) Book(id int64) (BookDetail, error) {
	b, err := s.db.Books.ByID(id)
	if err != nil {
		return BookDetail{}, err
	}
	d := BookDetail{
		ID:                        b.ID,
		Title:                     b.Title,
		TitleSort:                 b.TitleSort,
		Series:                    seriesName(b),
		SeriesIndex:               seriesIndex(b),
		Description:               b.Description,
		Language:                  b.Language,
		ISBN:                      b.ISBN,
		Published:                 b.PublishedAt,
		Cover:                     coverURL(b),
		AddedAt:                   formatTime(b.AddedAt),
		UpdatedAt:                 formatTime(b.UpdatedAt),
		RemotePathOverrideEnabled: b.RemotePathOverrideEnabled,
		RemotePathOverride:        b.RemotePathOverride,
		RemotePath:                remotePath(s.settings.Get().RemotePathTemplate, b),
	}
	for _, c := range b.Authors {
		d.Authors = append(d.Authors, Contributor{ID: c.Author.ID, Name: c.Author.Name, SortName: c.Author.SortName, Role: c.Role})
	}
	for _, t := range b.Tags {
		d.Tags = append(d.Tags, t.Name)
	}
	for _, f := range b.Files {
		d.Files = append(d.Files, FileInfo{
			Format:  f.Format,
			Size:    f.Size,
			Path:    f.Path,
			SHA256:  f.SHA256,
			AddedAt: formatTime(f.AddedAt),
		})
	}
	return d, nil
}

// QuickEditRows returns every book as a flat row for the spreadsheet-like
// editor. It intentionally loads full books so the row contains relations.
func (s *LibraryService) QuickEditRows() ([]QuickEditRow, error) {
	books, err := s.db.Books.Browse("title", 0, 0)
	if err != nil {
		return nil, err
	}
	rows := make([]QuickEditRow, 0, len(books))
	for _, b := range books {
		rows = append(rows, quickEditRow(b))
	}
	return rows, nil
}

// SaveQuickEdits applies changed quick-edit rows. Rows are independent: a
// validation/file-move failure is reported for that row and does not prevent
// later rows from saving.
func (s *LibraryService) SaveQuickEdits(rows []QuickEditRow) (QuickEditSaveResult, error) {
	var res QuickEditSaveResult
	for _, row := range rows {
		before, err := s.db.Books.ByID(row.ID)
		if err != nil {
			res.Failed++
			res.Errors = append(res.Errors, QuickEditRowError{ID: row.ID, Error: err.Error()})
			continue
		}
		if row.UpdatedAt != "" && formatTime(before.UpdatedAt) != row.UpdatedAt {
			res.Failed++
			res.Errors = append(res.Errors, QuickEditRowError{ID: row.ID, Error: "livre modifié depuis le chargement"})
			continue
		}
		_, err = s.UpdateBook(BookUpdate{
			ID:                        row.ID,
			Title:                     row.Title,
			TitleSort:                 row.TitleSort,
			Authors:                   splitQuickList(row.Authors),
			Series:                    row.Series,
			SeriesIndex:               row.SeriesIndex,
			Description:               before.Description,
			Language:                  row.Language,
			ISBN:                      row.ISBN,
			Published:                 row.Published,
			Tags:                      splitQuickList(row.Tags),
			RemotePathOverrideEnabled: row.RemotePathOverrideEnabled,
			RemotePathOverride:        row.RemotePathOverride,
		})
		if err != nil {
			res.Failed++
			res.Errors = append(res.Errors, QuickEditRowError{ID: row.ID, Error: err.Error()})
			continue
		}
		fresh, err := s.db.Books.ByID(row.ID)
		if err != nil {
			res.Failed++
			res.Errors = append(res.Errors, QuickEditRowError{ID: row.ID, Error: err.Error()})
			continue
		}
		res.Updated++
		res.Rows = append(res.Rows, QuickEditSavedBook{ID: row.ID, UpdatedAt: formatTime(fresh.UpdatedAt)})
	}
	return res, nil
}

// UpdateBook persists editable metadata and moves managed files when the
// primary author or title changes. The database remains the source of truth:
// if a file move fails, moved files and metadata are rolled back best-effort.
func (s *LibraryService) UpdateBook(in BookUpdate) (BookDetail, error) {
	before, err := s.db.Books.ByID(in.ID)
	if err != nil {
		return BookDetail{}, err
	}
	next, err := bookFromUpdate(in, before)
	if err != nil {
		return BookDetail{}, err
	}
	if err := s.db.Books.Update(next); err != nil {
		return BookDetail{}, err
	}

	moved, err := s.moveManagedFiles(before, next)
	if err != nil {
		rollbackMoves(moved)
		_ = s.db.Books.Update(before)
		return BookDetail{}, err
	}
	for _, mv := range moved {
		if err := s.db.Books.UpdateFilePath(mv.fileID, mv.to); err != nil {
			rollbackMoves(moved)
			_ = s.db.Books.Update(before)
			return BookDetail{}, err
		}
	}

	s.maybeWriteMetadata(in.ID)
	return s.Book(in.ID)
}

// SetTitleSort updates just a book's sort title (empty clears it) and, if
// enabled, mirrors it into the ebook file.
func (s *LibraryService) SetTitleSort(bookID int64, sort string) (BookDetail, error) {
	if err := s.db.Books.SetTitleSort(bookID, sort); err != nil {
		return BookDetail{}, err
	}
	s.maybeWriteMetadata(bookID)
	return s.Book(bookID)
}

// SetAuthorSort updates an author's sort name (empty clears it, so sends fall
// back to the display name) and mirrors the change into the given book's file
// if enabled. Note the author's sort name is shared across all their books.
func (s *LibraryService) SetAuthorSort(bookID, authorID int64, sort string) (BookDetail, error) {
	if err := s.db.Authors.SetSortName(authorID, sort); err != nil {
		return BookDetail{}, err
	}
	s.maybeWriteMetadata(bookID)
	return s.Book(bookID)
}

// CoverResult summarizes a cover-regeneration run.
type CoverResult struct {
	Scanned int `json:"scanned"`
	Updated int `json:"updated"`
}

// RegenerateCovers builds a cached thumbnail for every book that lacks one
// (e.g. PDFs imported before cover extraction existed). Books whose file yields
// no image are left without a cover.
func (s *LibraryService) RegenerateCovers() (CoverResult, error) {
	books, err := s.db.Books.List(0, 0)
	if err != nil {
		return CoverResult{}, err
	}
	var res CoverResult
	for _, b := range books {
		if b.CoverPath != "" {
			continue
		}
		res.Scanned++
		name, err := library.GenerateCover(formats.Default, s.coverDir, library.DefaultThumbnailMax, b)
		if err != nil {
			slog.Warn("regenerate cover failed", "book", b.ID, "err", err)
			continue
		}
		if name == "" {
			continue
		}
		if err := s.db.Books.SetCover(b.ID, name); err == nil {
			res.Updated++
		}
	}
	slog.Info("regenerate covers done", "scanned", res.Scanned, "updated", res.Updated)
	return res, nil
}

// maybeWriteMetadata writes a book's current metadata into its ebook files when
// the user has opted in. Best-effort: a failure is logged, not fatal (the DB is
// the source of truth and is already updated).
func (s *LibraryService) maybeWriteMetadata(bookID int64) {
	if !s.settings.Get().WriteMetadataToFile {
		return
	}
	fresh, err := s.db.Books.ByID(bookID)
	if err != nil {
		return
	}
	if err := library.WriteFileMetadata(formats.Default, fresh); err != nil {
		slog.Warn("write metadata to file failed", "book", bookID, "err", err)
	}
}

// BatchSetSeries assigns or clears a series for selected books. It does not
// move files: the managed path currently depends on title/primary author only.
func (s *LibraryService) BatchSetSeries(in BatchSeriesUpdate) (BatchUpdateResult, error) {
	if len(in.IDs) == 0 {
		return BatchUpdateResult{}, errors.New("no selected books")
	}
	series := strings.TrimSpace(in.Series)
	var (
		start    float64
		hasStart bool
	)
	if raw := strings.TrimSpace(in.SeriesIndexStart); raw != "" {
		v, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64)
		if err != nil {
			return BatchUpdateResult{}, fmt.Errorf("invalid series index %q", raw)
		}
		start, hasStart = v, true
	}

	var res BatchUpdateResult
	for i, id := range in.IDs {
		b, err := s.db.Books.ByID(id)
		if err != nil {
			return res, err
		}
		if series == "" {
			b.Series = nil
			b.SeriesIndex = nil
		} else {
			b.Series = &core.Series{Name: series}
			if hasStart {
				idx := start + float64(i)
				b.SeriesIndex = &idx
			}
		}
		if err := s.db.Books.Update(b); err != nil {
			return res, err
		}
		res.Updated++
	}
	return res, nil
}

// --- mapping helpers ---

func bookFromUpdate(in BookUpdate, before *core.Book) (*core.Book, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, errors.New("empty title")
	}
	b := &core.Book{
		ID:                        in.ID,
		Title:                     title,
		TitleSort:                 strings.TrimSpace(in.TitleSort),
		Description:               strings.TrimSpace(in.Description),
		Language:                  strings.TrimSpace(in.Language),
		ISBN:                      strings.TrimSpace(in.ISBN),
		PublishedAt:               strings.TrimSpace(in.Published),
		CoverPath:                 before.CoverPath,
		RemotePathOverrideEnabled: in.RemotePathOverrideEnabled,
		RemotePathOverride:        strings.TrimSpace(in.RemotePathOverride),
		Files:                     before.Files,
	}
	if series := strings.TrimSpace(in.Series); series != "" {
		b.Series = &core.Series{Name: series}
		if idx := strings.TrimSpace(in.SeriesIndex); idx != "" {
			v, err := strconv.ParseFloat(strings.ReplaceAll(idx, ",", "."), 64)
			if err != nil {
				return nil, fmt.Errorf("invalid series index %q", idx)
			}
			b.SeriesIndex = &v
		}
	}
	for i, name := range cleanStrings(in.Authors) {
		b.Authors = append(b.Authors, core.Contribution{
			Author:   core.Author{Name: name},
			Role:     "aut",
			Position: i,
		})
	}
	for _, name := range cleanStrings(in.Tags) {
		b.Tags = append(b.Tags, core.Tag{Name: name})
	}
	return b, nil
}

func quickEditRow(b *core.Book) QuickEditRow {
	tags := make([]string, 0, len(b.Tags))
	for _, t := range b.Tags {
		tags = append(tags, t.Name)
	}
	return QuickEditRow{
		ID:                        b.ID,
		Title:                     b.Title,
		TitleSort:                 b.TitleSort,
		Authors:                   strings.Join(b.AuthorNames(), ", "),
		Series:                    seriesName(b),
		SeriesIndex:               quickSeriesIndex(b),
		Tags:                      strings.Join(tags, ", "),
		Language:                  b.Language,
		Published:                 b.PublishedAt,
		ISBN:                      b.ISBN,
		RemotePathOverrideEnabled: b.RemotePathOverrideEnabled,
		RemotePathOverride:        b.RemotePathOverride,
		UpdatedAt:                 formatTime(b.UpdatedAt),
	}
}

func quickSeriesIndex(b *core.Book) string {
	if b.SeriesIndex == nil {
		return ""
	}
	return strconv.FormatFloat(*b.SeriesIndex, 'f', -1, 64)
}

func splitQuickList(raw string) []string {
	return cleanStrings(strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	}))
}

func cleanStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		key := strings.ToLower(s)
		if s == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

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

func appendMissingGroup(items []SidebarItem, label string, count int, err error) ([]SidebarItem, error) {
	if err != nil {
		return nil, err
	}
	if count > 0 {
		items = append(items, SidebarItem{ID: 0, Name: label, Count: count})
	}
	return items, nil
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

func remotePath(tmpl string, b *core.Book) string {
	if b.RemotePathOverrideEnabled {
		return strings.TrimSpace(b.RemotePathOverride)
	}
	return library.RenderRemotePath(tmpl, b)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

type fileMove struct {
	fileID int64
	from   string
	to     string
}

func (s *LibraryService) moveManagedFiles(before, after *core.Book) ([]fileMove, error) {
	cfg := s.settings.Get()
	author := "Unknown Author"
	if names := after.AuthorNames(); len(names) > 0 {
		author = names[0]
	}
	var moved []fileMove
	for _, f := range before.Files {
		if !managedPath(f.Path, cfg.LibraryDir) {
			continue
		}
		dst := uniquePath(managedDest(cfg.LibraryDir, author, after.Title, f.Path, f.Format), f.Path)
		if samePath(f.Path, dst) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return moved, err
		}
		if err := os.Rename(f.Path, dst); err != nil {
			return moved, err
		}
		moved = append(moved, fileMove{fileID: f.ID, from: f.Path, to: dst})
		cleanupEmptyParents(filepath.Dir(f.Path), cfg.LibraryDir)
	}
	return moved, nil
}

func rollbackMoves(moved []fileMove) {
	for i := len(moved) - 1; i >= 0; i-- {
		_ = os.MkdirAll(filepath.Dir(moved[i].from), 0o755)
		_ = os.Rename(moved[i].to, moved[i].from)
	}
}

func managedDest(root, author, title, srcPath, format string) string {
	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext == "" {
		ext = "." + format
	}
	return filepath.Join(root, sanitizePathPart(author), sanitizePathPart(title), sanitizePathPart(title)+ext)
}

func sanitizePathPart(name string) string {
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		if r < 0x20 {
			return '_'
		}
		return r
	}, strings.TrimSpace(name))
	name = strings.Trim(name, " .")
	if name == "" {
		return "_"
	}
	const max = 150
	if r := []rune(name); len(r) > max {
		name = strings.TrimRight(string(r[:max]), " .")
	}
	return name
}

func uniquePath(path, current string) string {
	if samePath(path, current) {
		return path
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s (%d)%s", stem, n, ext)
		if samePath(candidate, current) {
			return candidate
		}
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA == nil && errB == nil {
		a, b = aa, bb
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func cleanupEmptyParents(dir, root string) {
	for managedPath(dir, root) {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

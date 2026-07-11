// Package gutenberg lets Reliure discover Project Gutenberg books and hand their
// EPUB to the normal import pipeline.
//
// It works off Gutenberg's official catalogue CSV (~21 MB, ~90k rows), which it
// downloads once and caches locally. Searching is then done in memory: instant,
// offline, with real language filtering — unlike the Gutendex proxy, whose
// uncached queries can take tens of seconds. Cover and EPUB URLs are built by
// Gutenberg's fixed naming convention, so no per-book API call is needed.
//
// The package is independent of internal/core and internal/library: it returns
// neutral Book values and streams files, leaving import to the app layer.
package gutenberg

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultCatalogURL = "https://www.gutenberg.org/cache/epub/feeds/pg_catalog.csv"
	userAgent         = "Reliure/1.0 (+https://github.com/agrison/reliure)"
	catalogMaxAge     = 30 * 24 * time.Hour // refresh the cached CSV after a month
	pageSize          = 36
)

// Book is one catalogue entry reduced to what the UI and importer need.
type Book struct {
	ID        int
	Title     string
	Authors   []string
	Languages []string
	Subjects  []string
	CoverURL  string
	EPUBURL   string
}

// Query parameterises a catalogue search. All fields are optional.
type Query struct {
	Search    string   // free text over title and author
	Languages []string // ISO 639-1 codes, e.g. {"fr"}; empty means all
	Topic     string   // subject substring filter
	Page      int      // 1-based; 0 means the first page
}

// SearchResult is a page of results plus paging flags.
type SearchResult struct {
	Count   int
	HasNext bool
	HasPrev bool
	Page    int
	Books   []Book
}

// entry is the in-memory catalogue row. authors/subjects are pre-split; key is a
// lower-cased "title authors" blob used for fast substring matching.
type entry struct {
	id       int
	title    string
	authors  []string
	langs    []string
	subjects []string
	key      string
}

// Catalog holds the parsed catalogue and refreshes it from Gutenberg on demand.
type Catalog struct {
	url       string
	cachePath string
	http      *http.Client

	mu      sync.Mutex
	entries []entry
	byID    map[int]entry
}

// NewCatalog returns a Catalog caching the CSV at cachePath.
func NewCatalog(cachePath string) *Catalog {
	return &Catalog{
		url:       defaultCatalogURL,
		cachePath: cachePath,
		http: &http.Client{
			Timeout:   3 * time.Minute, // one-time ~21 MB download on slow links
			Transport: uaTransport{rt: http.DefaultTransport},
		},
	}
}

// uaTransport stamps a descriptive User-Agent on outgoing requests.
type uaTransport struct{ rt http.RoundTripper }

func (t uaTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Header.Get("User-Agent") == "" {
		r.Header.Set("User-Agent", userAgent)
	}
	return t.rt.RoundTrip(r)
}

// Search returns a page of catalogue results, loading/refreshing the catalogue
// first if needed (the only step that may touch the network).
func (c *Catalog) Search(ctx context.Context, q Query) (SearchResult, error) {
	if err := c.ensureLoaded(ctx); err != nil {
		return SearchResult{}, err
	}

	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(q.Search)))
	langs := lowerSet(q.Languages)
	topic := strings.ToLower(strings.TrimSpace(q.Topic))

	c.mu.Lock()
	entries := c.entries
	c.mu.Unlock()

	var matched []entry
	for _, e := range entries {
		if !matchTokens(e.key, tokens) {
			continue
		}
		if len(langs) > 0 && !hasAnyLang(e.langs, langs) {
			continue
		}
		if topic != "" && !subjectsContain(e.subjects, topic) {
			continue
		}
		matched = append(matched, e)
	}

	page := q.Page
	if page < 1 {
		page = 1
	}
	start := (page - 1) * pageSize
	if start > len(matched) {
		start = len(matched)
	}
	end := start + pageSize
	if end > len(matched) {
		end = len(matched)
	}
	res := SearchResult{
		Count:   len(matched),
		HasPrev: page > 1,
		HasNext: end < len(matched),
		Page:    page,
	}
	for _, e := range matched[start:end] {
		res.Books = append(res.Books, e.toBook())
	}
	return res, nil
}

// Book resolves a single catalogue entry by id.
func (c *Catalog) Book(ctx context.Context, id int) (Book, bool, error) {
	if err := c.ensureLoaded(ctx); err != nil {
		return Book{}, false, err
	}
	c.mu.Lock()
	e, ok := c.byID[id]
	c.mu.Unlock()
	if !ok {
		return Book{}, false, nil
	}
	return e.toBook(), true, nil
}

// Download streams a book's EPUB, trying Gutenberg's URL variants in order (the
// images build first, then the plain one). The caller must close the reader.
func (c *Catalog) Download(ctx context.Context, id int) (io.ReadCloser, error) {
	var lastErr error
	for _, u := range epubURLs(id) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return resp.Body, nil
		}
		resp.Body.Close()
		lastErr = fmt.Errorf("download %s: status %d", u, resp.StatusCode)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no EPUB URL for book %d", id)
	}
	return nil, lastErr
}

// ensureLoaded parses the cached CSV, (re)downloading it if missing or stale.
func (c *Catalog) ensureLoaded(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries != nil {
		return nil
	}
	if c.stale() {
		if err := c.download(ctx); err != nil && !c.cacheExists() {
			return err // no usable cache to fall back on
		}
	}
	f, err := os.Open(c.cachePath)
	if err != nil {
		return err
	}
	defer f.Close()
	entries, byID, err := parseCatalog(f)
	if err != nil {
		return err
	}
	c.entries = entries
	c.byID = byID
	return nil
}

func (c *Catalog) cacheExists() bool {
	_, err := os.Stat(c.cachePath)
	return err == nil
}

func (c *Catalog) stale() bool {
	info, err := os.Stat(c.cachePath)
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) > catalogMaxAge
}

// download fetches the CSV to a temp file and renames it into place atomically.
func (c *Catalog) download(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("catalog download: status %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(c.cachePath), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(c.cachePath), "pg_catalog-*.csv")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, c.cachePath)
}

// parseCatalog reads the CSV, keeping text books only.
func parseCatalog(r io.Reader) ([]entry, map[int]entry, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // tolerate irregular rows
	cr.ReuseRecord = true

	header, err := cr.Read()
	if err != nil {
		return nil, nil, err
	}
	col := columnIndex(header)

	entries := make([]entry, 0, 80000)
	byID := make(map[int]entry, 80000)
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip a malformed row rather than abort the whole catalogue
		}
		if get(rec, col["Type"]) != "Text" {
			continue
		}
		id, err := strconv.Atoi(strings.TrimSpace(get(rec, col["Text#"])))
		if err != nil {
			continue
		}
		title := strings.TrimSpace(get(rec, col["Title"]))
		if title == "" {
			continue
		}
		authors := cleanAuthors(get(rec, col["Authors"]))
		e := entry{
			id:       id,
			title:    title,
			authors:  authors,
			langs:    splitList(get(rec, col["Language"])),
			subjects: trimTo(splitList(get(rec, col["Subjects"])), 6),
			key:      strings.ToLower(title + " " + strings.Join(authors, " ")),
		}
		entries = append(entries, e)
		byID[id] = e
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	return entries, byID, nil
}

func (e entry) toBook() Book {
	return Book{
		ID:        e.id,
		Title:     e.title,
		Authors:   e.authors,
		Languages: e.langs,
		Subjects:  e.subjects,
		CoverURL:  coverURL(e.id),
		EPUBURL:   epubURLs(e.id)[0],
	}
}

// --- URL conventions ---

func coverURL(id int) string {
	s := strconv.Itoa(id)
	return "https://www.gutenberg.org/cache/epub/" + s + "/pg" + s + ".cover.medium.jpg"
}

func epubURLs(id int) []string {
	s := strconv.Itoa(id)
	return []string{
		"https://www.gutenberg.org/ebooks/" + s + ".epub3.images",
		"https://www.gutenberg.org/ebooks/" + s + ".epub.images",
		"https://www.gutenberg.org/ebooks/" + s + ".epub.noimages",
	}
}

// --- parsing/matching helpers ---

func columnIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[strings.TrimSpace(h)] = i
	}
	return idx
}

func get(rec []string, i int) string {
	if i >= 0 && i < len(rec) {
		return rec[i]
	}
	return ""
}

// authorDates matches the trailing life-dates Gutenberg appends to an author,
// e.g. "Verne, Jules, 1828-1905" → the ", 1828-1905" part.
var authorDates = regexp.MustCompile(`,\s*\d{3,4}[0-9?bcBC.\-\s]*$`)

func cleanAuthors(raw string) []string {
	parts := strings.Split(raw, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(authorDates.ReplaceAllString(strings.TrimSpace(p), ""))
		p = strings.Trim(p, ",; ")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitList(raw string) []string {
	parts := strings.Split(raw, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func trimTo(ss []string, n int) []string {
	if len(ss) > n {
		return ss[:n]
	}
	return ss
}

func matchTokens(key string, tokens []string) bool {
	for _, t := range tokens {
		if !strings.Contains(key, t) {
			return false
		}
	}
	return true
}

func lowerSet(vals []string) map[string]bool {
	if len(vals) == 0 {
		return nil
	}
	m := make(map[string]bool, len(vals))
	for _, v := range vals {
		if v = strings.ToLower(strings.TrimSpace(v)); v != "" {
			m[v] = true
		}
	}
	return m
}

func hasAnyLang(langs []string, want map[string]bool) bool {
	for _, l := range langs {
		if want[strings.ToLower(l)] {
			return true
		}
	}
	return false
}

func subjectsContain(subjects []string, needle string) bool {
	for _, s := range subjects {
		if strings.Contains(strings.ToLower(s), needle) {
			return true
		}
	}
	return false
}

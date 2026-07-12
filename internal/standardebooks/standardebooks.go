// Package standardebooks discovers Standard Ebooks titles from public metadata
// and streams EPUB downloads to the app layer.
package standardebooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultCatalogURL = "https://api.github.com/orgs/standardebooks/repos"
	siteBaseURL       = "https://standardebooks.org/ebooks/"
	userAgent         = "Reliure/1.0 (+https://github.com/agrison/reliure)"
	catalogMaxAge     = 7 * 24 * time.Hour
	pageSize          = 36
)

// Book is one Standard Ebooks catalogue entry reduced to what Reliure needs.
type Book struct {
	ID        string
	Title     string
	Authors   []string
	Languages []string
	Subjects  []string
	CoverURL  string
	EPUBURL   string
}

type Query struct {
	Search    string
	Languages []string
	Page      int
}

type SearchResult struct {
	Count   int
	HasNext bool
	HasPrev bool
	Page    int
	Books   []Book
}

type entry struct {
	id       string
	title    string
	authors  []string
	langs    []string
	subjects []string
	coverURL string
	epubURL  string
	key      string
}

type repo struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Fork        bool   `json:"fork"`
	Archived    bool   `json:"archived"`
}

// Catalog caches and searches Standard Ebooks' public GitHub repositories.
type Catalog struct {
	url       string
	cachePath string
	http      *http.Client

	mu      sync.Mutex
	entries []entry
	byID    map[string]entry
}

func NewCatalog(cachePath string) *Catalog {
	return &Catalog{
		url:       defaultCatalogURL,
		cachePath: cachePath,
		http: &http.Client{
			Timeout:   90 * time.Second,
			Transport: uaTransport{rt: http.DefaultTransport},
		},
	}
}

type uaTransport struct{ rt http.RoundTripper }

func (t uaTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Header.Get("User-Agent") == "" {
		r.Header.Set("User-Agent", userAgent)
	}
	if r.Header.Get("Accept") == "" {
		r.Header.Set("Accept", "application/vnd.github+json")
	}
	return t.rt.RoundTrip(r)
}

func (c *Catalog) Search(ctx context.Context, q Query) (SearchResult, error) {
	if err := c.ensureLoaded(ctx); err != nil {
		return SearchResult{}, err
	}

	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(q.Search)))
	langs := lowerSet(q.Languages)

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

func (c *Catalog) Book(ctx context.Context, id string) (Book, bool, error) {
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

func (c *Catalog) Download(ctx context.Context, id string) (io.ReadCloser, error) {
	book, ok, err := c.Book(ctx, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("standard ebooks book %q not found", id)
	}
	if book.EPUBURL == "" {
		return nil, fmt.Errorf("no EPUB URL for %q", book.Title)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, book.EPUBURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download %s: status %d", book.EPUBURL, resp.StatusCode)
	}
	return resp.Body, nil
}

func (c *Catalog) ensureLoaded(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries != nil {
		return nil
	}
	if c.stale() {
		if err := c.download(ctx); err != nil && !c.cacheExists() {
			return err
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

func (c *Catalog) download(ctx context.Context) error {
	repos, err := c.fetchRepos(ctx)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.cachePath), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(c.cachePath), "standardebooks-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	enc := json.NewEncoder(tmp)
	if err := enc.Encode(repos); err != nil {
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

func (c *Catalog) fetchRepos(ctx context.Context) ([]repo, error) {
	var repos []repo
	for page := 1; ; page++ {
		u := fmt.Sprintf("%s?per_page=100&type=public&sort=full_name&page=%d", c.url, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("standard ebooks catalogue download: status %d", resp.StatusCode)
		}
		var pageRepos []repo
		err = json.NewDecoder(resp.Body).Decode(&pageRepos)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if len(pageRepos) == 0 {
			break
		}
		repos = append(repos, pageRepos...)
		if len(pageRepos) < 100 {
			break
		}
	}
	return repos, nil
}

func parseCatalog(r io.Reader) ([]entry, map[string]entry, error) {
	var repos []repo
	if err := json.NewDecoder(r).Decode(&repos); err != nil {
		return nil, nil, err
	}
	entries := make([]entry, 0, len(repos))
	byID := make(map[string]entry, len(repos))
	for _, raw := range repos {
		e := normalizeRepo(raw)
		if e.id == "" || e.title == "" {
			continue
		}
		entries = append(entries, e)
		byID[e.id] = e
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].title) < strings.ToLower(entries[j].title)
	})
	return entries, byID, nil
}

func normalizeRepo(raw repo) entry {
	if raw.Fork || raw.Archived || !strings.HasPrefix(raw.FullName, "standardebooks/") {
		return entry{}
	}
	name := strings.TrimSpace(raw.Name)
	parts := strings.Split(name, "_")
	if name == "" || len(parts) < 2 {
		return entry{}
	}
	authors := splitPeople(parts[0])
	title := titleFromSlug(parts[1])
	if title == "" {
		return entry{}
	}
	sitePath := strings.ReplaceAll(name, "_", "/")
	return entry{
		id:       name,
		title:    title,
		authors:  authors,
		langs:    []string{"en"},
		subjects: subjectsFromDescription(raw.Description),
		coverURL: siteBaseURL + sitePath + "/downloads/cover.jpg",
		epubURL:  siteBaseURL + sitePath + "/downloads/" + name + ".epub",
		key:      strings.ToLower(name + " " + title + " " + strings.Join(authors, " ") + " " + raw.Description),
	}
}

func (e entry) toBook() Book {
	return Book{
		ID:        e.id,
		Title:     e.title,
		Authors:   e.authors,
		Languages: e.langs,
		Subjects:  e.subjects,
		CoverURL:  e.coverURL,
		EPUBURL:   e.epubURL,
	}
}

func splitPeople(slug string) []string {
	parts := strings.Split(slug, "_")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = titleFromSlug(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func titleFromSlug(slug string) string {
	words := strings.Split(strings.ReplaceAll(slug, "-", " "), " ")
	for i, w := range words {
		words[i] = capitalizeSmallWord(w)
	}
	return strings.TrimSpace(strings.Join(words, " "))
}

func capitalizeSmallWord(s string) string {
	if s == "" {
		return ""
	}
	switch s {
	case "a", "an", "and", "as", "at", "but", "by", "for", "from", "in", "nor", "of", "on", "or", "the", "to", "with":
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func subjectsFromDescription(description string) []string {
	description = strings.TrimSpace(description)
	if description == "" {
		return nil
	}
	return []string{description}
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

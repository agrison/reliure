// Package metadata fetches book metadata from online catalogues so a user can
// enrich a locally-imported book (fix authors, add a description, a cover, a
// series…). It is provider-based and dependency-free: each source implements
// Provider, and a Client fans a query out to all of them, then merges and ranks
// the results.
//
// The package deliberately does not import internal/core: it produces neutral
// Candidate values that the app layer maps onto the domain and lets the user
// pick field by field. Ranking favours the caller's preferred language so, for
// a French book, French editions (descriptions, covers) surface first while
// every other edition stays visible to choose from.
package metadata

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Query describes the book to look up. All fields are optional; providers use
// whatever they get (ISBN is the most precise, then title+author).
type Query struct {
	Title    string
	Authors  []string
	ISBN     string
	Language string // preferred language hint (ISO 639-1, e.g. "fr"); ranking only
	Max      int    // soft cap on results per provider (0 → provider default)
}

// Candidate is one edition returned by a provider. Fields are best-effort: a
// provider fills what it has and leaves the rest zero.
type Candidate struct {
	Source      string // provider name ("googlebooks", "openlibrary")
	SourceID    string // provider-local id, for a stable UI key
	Title       string
	Subtitle    string
	Authors     []string
	Publisher   string
	Published   string // free-form date, ISO 8601 when known
	Description string
	Language    string // normalised to ISO 639-1 when possible
	ISBN        string // preferred ISBN-13, else ISBN-10
	Series      string
	SeriesIndex *float64
	Tags        []string
	CoverURL    string
	PageCount   int
}

// Provider is a single online source. Implementations must be safe for
// concurrent use and must never panic on malformed responses; on a transport or
// decode error they return the error so the Client can degrade gracefully.
type Provider interface {
	Name() string
	Search(ctx context.Context, q Query, hc *http.Client) ([]Candidate, error)
}

// Client aggregates providers behind one Search call.
type Client struct {
	providers []Provider
	http      *http.Client
}

// userAgent identifies Reliure to the public APIs. OpenLibrary asks callers to
// send a descriptive UA, and a real UA also lightens Google Books' anonymous
// rate-limiting (HTTP 429).
const userAgent = "Reliure/1.0 (+https://github.com/agrison/reliure)"

// NewClient returns a Client wired with the default providers (Google Books,
// OpenLibrary and the BnF), all free and key-less.
func NewClient() *Client {
	return &Client{
		providers: []Provider{NewGoogleBooks(""), NewOpenLibrary(""), NewBnF("")},
		http: &http.Client{
			Timeout:   12 * time.Second,
			Transport: uaTransport{rt: http.DefaultTransport},
		},
	}
}

// uaTransport stamps a User-Agent on every outgoing request that lacks one.
type uaTransport struct{ rt http.RoundTripper }

func (t uaTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Header.Get("User-Agent") == "" {
		r.Header.Set("User-Agent", userAgent)
	}
	return t.rt.RoundTrip(r)
}

// NewClientWith builds a Client from an explicit provider list and HTTP client;
// used by tests to inject fakes.
func NewClientWith(hc *http.Client, providers ...Provider) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 12 * time.Second}
	}
	return &Client{providers: providers, http: hc}
}

// Search queries every provider concurrently, merges duplicate editions and
// ranks the result. A provider that fails is skipped (its error is not fatal):
// partial results beat no results. An error is returned only if every provider
// failed.
func (c *Client) Search(ctx context.Context, q Query) ([]Candidate, error) {
	type out struct {
		cands []Candidate
		err   error
	}
	results := make([]out, len(c.providers))
	done := make(chan int, len(c.providers))
	for i, p := range c.providers {
		go func(i int, p Provider) {
			cands, err := p.Search(ctx, q, c.http)
			results[i] = out{cands: cands, err: err}
			done <- i
		}(i, p)
	}
	for range c.providers {
		<-done
	}

	var (
		all      []Candidate
		failures int
		lastErr  error
	)
	for _, r := range results {
		if r.err != nil {
			failures++
			lastErr = r.err
			continue
		}
		all = append(all, r.cands...)
	}
	if failures == len(c.providers) && len(c.providers) > 0 {
		return nil, fmt.Errorf("all metadata providers failed: %w", lastErr)
	}

	merged := mergeCandidates(all)
	rankCandidates(merged, normalizeLang(q.Language))
	return merged, nil
}

// FetchImage downloads a cover image, capping the body at maxBytes so a
// misbehaving host can't exhaust memory.
func (c *Client) FetchImage(ctx context.Context, url string, maxBytes int64) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("empty image url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image fetch: status %d", resp.StatusCode)
	}
	if maxBytes <= 0 {
		maxBytes = 12 << 20
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// mergeCandidates removes duplicate editions (same ISBN-13, or same
// title+first-author+language) that different providers returned, keeping the
// first occurrence and filling its empty fields from the duplicates so the
// merged entry is as complete as possible.
func mergeCandidates(in []Candidate) []Candidate {
	var out []Candidate
	index := map[string]int{}
	for _, c := range in {
		key := dedupKey(c)
		if key == "" {
			out = append(out, c)
			continue
		}
		if i, ok := index[key]; ok {
			out[i] = fillMissing(out[i], c)
			continue
		}
		index[key] = len(out)
		out = append(out, c)
	}
	return out
}

func dedupKey(c Candidate) string {
	if isbn := digitsOnly(c.ISBN); len(isbn) == 13 {
		return "isbn:" + isbn
	}
	title := strings.ToLower(strings.TrimSpace(c.Title))
	if title == "" {
		return ""
	}
	author := ""
	if len(c.Authors) > 0 {
		author = strings.ToLower(strings.TrimSpace(c.Authors[0]))
	}
	return "ta:" + title + "|" + author + "|" + normalizeLang(c.Language)
}

// fillMissing copies fields set on b into a where a's are empty.
func fillMissing(a, b Candidate) Candidate {
	if a.Subtitle == "" {
		a.Subtitle = b.Subtitle
	}
	if len(a.Authors) == 0 {
		a.Authors = b.Authors
	}
	if a.Publisher == "" {
		a.Publisher = b.Publisher
	}
	if a.Published == "" {
		a.Published = b.Published
	}
	if a.Description == "" {
		a.Description = b.Description
	}
	if a.Language == "" {
		a.Language = b.Language
	}
	if a.ISBN == "" {
		a.ISBN = b.ISBN
	}
	if a.Series == "" {
		a.Series = b.Series
		a.SeriesIndex = b.SeriesIndex
	}
	if len(a.Tags) == 0 {
		a.Tags = b.Tags
	}
	if a.CoverURL == "" {
		a.CoverURL = b.CoverURL
	}
	if a.PageCount == 0 {
		a.PageCount = b.PageCount
	}
	return a
}

// rankCandidates orders editions by usefulness for the given preferred language:
// language match first, then richer entries (cover, description, ISBN). It is a
// stable sort so provider order breaks ties.
func rankCandidates(cands []Candidate, prefLang string) {
	sort.SliceStable(cands, func(i, j int) bool {
		return score(cands[i], prefLang) > score(cands[j], prefLang)
	})
}

func score(c Candidate, prefLang string) int {
	s := 0
	if prefLang != "" && normalizeLang(c.Language) == prefLang {
		s += 100
	}
	if c.CoverURL != "" {
		s += 20
	}
	if c.Description != "" {
		s += 15
	}
	if c.ISBN != "" {
		s += 8
	}
	if c.Series != "" {
		s += 5
	}
	if len(c.Tags) > 0 {
		s += 3
	}
	return s
}

package metadata

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// htmlTag matches a single HTML tag; Google Books descriptions often embed
// <p>/<br>/<b> which we don't want ending up in the OPF or the UI.
var htmlTag = regexp.MustCompile(`<[^>]*>`)

// stripHTML turns a small HTML snippet into readable plain text: block tags
// become newlines/spaces, remaining tags are removed and entities unescaped.
func stripHTML(s string) string {
	if s == "" || !strings.Contains(s, "<") {
		return strings.TrimSpace(html.UnescapeString(s))
	}
	s = regexp.MustCompile(`(?i)<br\s*/?>`).ReplaceAllString(s, "\n")
	s = regexp.MustCompile(`(?i)</p>`).ReplaceAllString(s, "\n\n")
	s = htmlTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	// Normalise non-breaking spaces to plain ones and collapse the runs of blank
	// lines the substitutions may create.
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// defaultGoogleBooksURL is the public, key-less volumes endpoint. A key raises
// the rate limit but is not required for the modest volume Reliure needs.
const defaultGoogleBooksURL = "https://www.googleapis.com/books/v1/volumes"

// GoogleBooks looks books up in the Google Books API. It is the richest single
// source (descriptions, categories, covers, language), so it is queried first.
type GoogleBooks struct {
	baseURL string
}

// NewGoogleBooks returns a provider; baseURL defaults to the public endpoint and
// is overridable for tests.
func NewGoogleBooks(baseURL string) *GoogleBooks {
	if baseURL == "" {
		baseURL = defaultGoogleBooksURL
	}
	return &GoogleBooks{baseURL: baseURL}
}

func (g *GoogleBooks) Name() string { return "googlebooks" }

// gbVolumes mirrors the subset of the Google Books response we consume.
type gbVolumes struct {
	Items []struct {
		ID         string `json:"id"`
		VolumeInfo struct {
			Title               string   `json:"title"`
			Subtitle            string   `json:"subtitle"`
			Authors             []string `json:"authors"`
			Publisher           string   `json:"publisher"`
			PublishedDate       string   `json:"publishedDate"`
			Description         string   `json:"description"`
			Language            string   `json:"language"`
			PageCount           int      `json:"pageCount"`
			Categories          []string `json:"categories"`
			IndustryIdentifiers []struct {
				Type       string `json:"type"`
				Identifier string `json:"identifier"`
			} `json:"industryIdentifiers"`
			ImageLinks struct {
				Thumbnail      string `json:"thumbnail"`
				SmallThumbnail string `json:"smallThumbnail"`
			} `json:"imageLinks"`
		} `json:"volumeInfo"`
	} `json:"items"`
}

func (g *GoogleBooks) Search(ctx context.Context, q Query, hc *http.Client) ([]Candidate, error) {
	expr := googleQueryExpr(q)
	if expr == "" {
		return nil, nil
	}
	params := url.Values{}
	params.Set("q", expr)
	params.Set("printType", "books")
	max := q.Max
	if max <= 0 || max > 40 {
		max = 10
	}
	params.Set("maxResults", strconv.Itoa(max))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &httpError{provider: g.Name(), status: resp.StatusCode}
	}
	var body gbVolumes
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	out := make([]Candidate, 0, len(body.Items))
	for _, it := range body.Items {
		vi := it.VolumeInfo
		if strings.TrimSpace(vi.Title) == "" {
			continue
		}
		c := Candidate{
			Source:      g.Name(),
			SourceID:    it.ID,
			Title:       vi.Title,
			Subtitle:    vi.Subtitle,
			Authors:     vi.Authors,
			Publisher:   vi.Publisher,
			Published:   vi.PublishedDate,
			Description: stripHTML(vi.Description),
			Language:    normalizeLang(vi.Language),
			Tags:        vi.Categories,
			PageCount:   vi.PageCount,
			ISBN:        gbISBN(vi.IndustryIdentifiers),
			CoverURL:    cleanGoogleImage(vi.ImageLinks.Thumbnail, vi.ImageLinks.SmallThumbnail),
		}
		out = append(out, c)
	}
	return out, nil
}

// googleQueryExpr builds a structured Google Books query: ISBN is the most
// precise, otherwise intitle/inauthor. Returns "" when there is nothing to
// search on.
func googleQueryExpr(q Query) string {
	if isbn := digitsOnly(q.ISBN); len(isbn) >= 10 {
		return "isbn:" + isbn
	}
	var parts []string
	if t := strings.TrimSpace(q.Title); t != "" {
		parts = append(parts, "intitle:"+t)
	}
	for _, a := range q.Authors {
		if a = strings.TrimSpace(a); a != "" {
			parts = append(parts, "inauthor:"+a)
			break // first author is enough to disambiguate
		}
	}
	return strings.Join(parts, " ")
}

// gbISBN prefers the ISBN-13 among the identifiers, falling back to ISBN-10.
func gbISBN(ids []struct {
	Type       string `json:"type"`
	Identifier string `json:"identifier"`
}) string {
	var isbn10 string
	for _, id := range ids {
		switch id.Type {
		case "ISBN_13":
			return id.Identifier
		case "ISBN_10":
			isbn10 = id.Identifier
		}
	}
	return isbn10
}

// cleanGoogleImage normalises a Google Books cover link: force https and drop
// the page-curl overlay so the thumbnail is a clean cover.
func cleanGoogleImage(thumb, small string) string {
	u := thumb
	if u == "" {
		u = small
	}
	if u == "" {
		return ""
	}
	u = strings.Replace(u, "http://", "https://", 1)
	u = strings.Replace(u, "&edge=curl", "", 1)
	return u
}

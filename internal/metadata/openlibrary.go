package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// defaultOpenLibraryBase is the OpenLibrary host; the provider builds the
// search and edition endpoints from it (no key, negligible quota for light use).
const defaultOpenLibraryBase = "https://openlibrary.org"

// OpenLibrary broadens coverage beyond Google Books, especially for editions
// and non-English titles. An ISBN lookup uses the edition-precise Books API so
// the cover and identifiers match the exact edition asked for; a title/author
// lookup uses the fuzzy search endpoint.
type OpenLibrary struct {
	base string
}

// NewOpenLibrary returns a provider; base defaults to the public host and is
// overridable for tests (which route on the request path).
func NewOpenLibrary(base string) *OpenLibrary {
	if base == "" {
		base = defaultOpenLibraryBase
	}
	return &OpenLibrary{base: strings.TrimRight(base, "/")}
}

func (o *OpenLibrary) Name() string { return "openlibrary" }

func (o *OpenLibrary) Search(ctx context.Context, q Query, hc *http.Client) ([]Candidate, error) {
	if isbn := digitsOnly(q.ISBN); len(isbn) >= 10 {
		return o.searchByISBN(ctx, isbn, hc)
	}
	if strings.TrimSpace(q.Title) != "" {
		return o.searchByText(ctx, q, hc)
	}
	return nil, nil
}

// --- ISBN lookup: the edition-precise Books API (jscmd=data) ---

type olBook struct {
	Key      string `json:"key"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Authors  []struct {
		Name string `json:"name"`
	} `json:"authors"`
	Publishers []struct {
		Name string `json:"name"`
	} `json:"publishers"`
	PublishDate string `json:"publish_date"`
	Cover       struct {
		Large  string `json:"large"`
		Medium string `json:"medium"`
	} `json:"cover"`
	Identifiers struct {
		ISBN13 []string `json:"isbn_13"`
		ISBN10 []string `json:"isbn_10"`
	} `json:"identifiers"`
	Subjects []struct {
		Name string `json:"name"`
	} `json:"subjects"`
	NumberOfPages int `json:"number_of_pages"`
}

func (o *OpenLibrary) searchByISBN(ctx context.Context, isbn string, hc *http.Client) ([]Candidate, error) {
	params := url.Values{}
	params.Set("bibkeys", "ISBN:"+isbn)
	params.Set("format", "json")
	params.Set("jscmd", "data")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.base+"/api/books?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &httpError{provider: o.Name(), status: resp.StatusCode}
	}
	// The response is keyed by "ISBN:<isbn>"; there is at most one entry.
	var byKey map[string]olBook
	if err := json.NewDecoder(resp.Body).Decode(&byKey); err != nil {
		return nil, err
	}
	out := make([]Candidate, 0, 1)
	for _, b := range byKey {
		if strings.TrimSpace(b.Title) == "" {
			continue
		}
		out = append(out, Candidate{
			Source:    o.Name(),
			SourceID:  b.Key,
			Title:     b.Title,
			Subtitle:  b.Subtitle,
			Authors:   olBookAuthors(b),
			Publisher: olBookPublisher(b),
			Published: b.PublishDate,
			ISBN:      olBookISBN(b, isbn),
			CoverURL:  firstNonEmpty(b.Cover.Large, b.Cover.Medium),
			Tags:      olBookSubjects(b),
			PageCount: b.NumberOfPages,
		})
	}
	return out, nil
}

func olBookAuthors(b olBook) []string {
	out := make([]string, 0, len(b.Authors))
	for _, a := range b.Authors {
		if a.Name != "" {
			out = append(out, a.Name)
		}
	}
	return out
}

func olBookPublisher(b olBook) string {
	if len(b.Publishers) > 0 {
		return b.Publishers[0].Name
	}
	return ""
}

func olBookISBN(b olBook, queried string) string {
	if len(b.Identifiers.ISBN13) > 0 {
		return b.Identifiers.ISBN13[0]
	}
	if len(b.Identifiers.ISBN10) > 0 {
		return b.Identifiers.ISBN10[0]
	}
	return queried
}

func olBookSubjects(b olBook) []string {
	out := make([]string, 0, len(b.Subjects))
	for _, s := range b.Subjects {
		if s.Name != "" {
			out = append(out, s.Name)
		}
	}
	return trimSubjects(out)
}

// --- text lookup: the fuzzy search endpoint ---

type olResponse struct {
	Docs []struct {
		Key          string   `json:"key"`
		Title        string   `json:"title"`
		AuthorName   []string `json:"author_name"`
		FirstPublish int      `json:"first_publish_year"`
		ISBN         []string `json:"isbn"`
		Language     []string `json:"language"`
		CoverI       int      `json:"cover_i"`
		Publisher    []string `json:"publisher"`
		Subject      []string `json:"subject"`
	} `json:"docs"`
}

func (o *OpenLibrary) searchByText(ctx context.Context, q Query, hc *http.Client) ([]Candidate, error) {
	params := url.Values{}
	params.Set("title", strings.TrimSpace(q.Title))
	if len(q.Authors) > 0 && strings.TrimSpace(q.Authors[0]) != "" {
		params.Set("author", strings.TrimSpace(q.Authors[0]))
	}
	max := q.Max
	if max <= 0 || max > 40 {
		max = 10
	}
	params.Set("limit", strconv.Itoa(max))
	params.Set("fields", "key,title,author_name,first_publish_year,isbn,language,cover_i,publisher,subject")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.base+"/search.json?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &httpError{provider: o.Name(), status: resp.StatusCode}
	}
	var body olResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	out := make([]Candidate, 0, len(body.Docs))
	for _, d := range body.Docs {
		if strings.TrimSpace(d.Title) == "" {
			continue
		}
		lang := ""
		if len(d.Language) > 0 {
			lang = normalizeLang(d.Language[0])
		}
		published := ""
		if d.FirstPublish > 0 {
			published = strconv.Itoa(d.FirstPublish)
		}
		publisher := ""
		if len(d.Publisher) > 0 {
			publisher = d.Publisher[0]
		}
		out = append(out, Candidate{
			Source:    o.Name(),
			SourceID:  d.Key,
			Title:     d.Title,
			Authors:   d.AuthorName,
			Publisher: publisher,
			Published: published,
			Language:  lang,
			ISBN:      olISBN(d.ISBN),
			Tags:      trimSubjects(d.Subject),
			CoverURL:  olCoverURL(d.CoverI),
		})
	}
	return out, nil
}

// olISBN prefers a 13-digit ISBN among the (often many) listed, else the first.
func olISBN(isbns []string) string {
	for _, s := range isbns {
		if len(digitsOnly(s)) == 13 {
			return s
		}
	}
	if len(isbns) > 0 {
		return isbns[0]
	}
	return ""
}

func olCoverURL(coverI int) string {
	if coverI <= 0 {
		return ""
	}
	return "https://covers.openlibrary.org/b/id/" + strconv.Itoa(coverI) + "-L.jpg"
}

// trimSubjects caps OpenLibrary's frequently huge subject list to a handful of
// usable tags.
func trimSubjects(subjects []string) []string {
	const max = 8
	if len(subjects) > max {
		return subjects[:max]
	}
	return subjects
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

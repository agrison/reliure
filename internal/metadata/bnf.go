package metadata

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// defaultBnFURL is the Bibliothèque nationale de France SRU endpoint. It is
// public and key-less; we ask for Dublin Core records, which are far simpler to
// parse than the alternative UNIMARC and carry what we need.
const defaultBnFURL = "https://catalogue.bnf.fr/api/SRU"

// BnF queries the French national library. It is the authority for French
// editions (correct French titles, publishers, language) and complements the
// others, which supply the cover and ISBN. It returns neither cover nor ISBN.
type BnF struct {
	baseURL string
}

// NewBnF returns a provider; baseURL defaults to the public endpoint and is
// overridable for tests.
func NewBnF(baseURL string) *BnF {
	if baseURL == "" {
		baseURL = defaultBnFURL
	}
	return &BnF{baseURL: baseURL}
}

func (b *BnF) Name() string { return "bnf" }

// bnfResponse maps the SRU response down to the Dublin Core fields we read.
// Element names are matched by local name, ignoring the SRU/DC namespaces.
type bnfResponse struct {
	Records []struct {
		Data struct {
			DC struct {
				Titles      []string `xml:"title"`
				Creators    []string `xml:"creator"`
				Publishers  []string `xml:"publisher"`
				Dates       []string `xml:"date"`
				Languages   []string `xml:"language"`
				Identifiers []string `xml:"identifier"`
			} `xml:"dc"`
		} `xml:"recordData"`
	} `xml:"records>record"`
}

func (b *BnF) Search(ctx context.Context, q Query, hc *http.Client) ([]Candidate, error) {
	cql := bnfCQL(q)
	if cql == "" {
		return nil, nil
	}
	max := q.Max
	if max <= 0 || max > 20 {
		max = 10
	}
	params := url.Values{}
	params.Set("version", "1.2")
	params.Set("operation", "searchRetrieve")
	params.Set("query", cql)
	params.Set("recordSchema", "dublincore")
	params.Set("maximumRecords", strconv.Itoa(max))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &httpError{provider: b.Name(), status: resp.StatusCode}
	}
	var body bnfResponse
	if err := xml.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	out := make([]Candidate, 0, len(body.Records))
	for _, r := range body.Records {
		dc := r.Data.DC
		title := bnfTitle(first(dc.Titles))
		if title == "" {
			continue
		}
		out = append(out, Candidate{
			Source:    b.Name(),
			SourceID:  bnfArk(dc.Identifiers),
			Title:     title,
			Authors:   bnfCreators(dc.Creators),
			Publisher: bnfPublisher(first(dc.Publishers)),
			Published: bnfYear(first(dc.Dates)),
			Language:  normalizeLang(first(dc.Languages)),
		})
	}
	return out, nil
}

// bnfCQL builds the CQL query: ISBN is most precise, otherwise title (+author).
func bnfCQL(q Query) string {
	if isbn := digitsOnly(q.ISBN); len(isbn) >= 10 {
		return `bib.isbn all "` + isbn + `"`
	}
	title := strings.TrimSpace(q.Title)
	if title == "" {
		return ""
	}
	clause := `bib.title all "` + cqlEscape(title) + `"`
	if len(q.Authors) > 0 && strings.TrimSpace(q.Authors[0]) != "" {
		clause += ` and bib.author all "` + cqlEscape(strings.TrimSpace(q.Authors[0])) + `"`
	}
	return clause
}

func cqlEscape(s string) string {
	return strings.ReplaceAll(s, `"`, " ")
}

// bnfTitle drops the statement of responsibility BnF appends to titles, e.g.
// "L'assassin royal / Robin Hobb" → "L'assassin royal".
func bnfTitle(s string) string {
	if i := strings.Index(s, " / "); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// bnfCreators cleans BnF author strings, which carry life dates and a role, e.g.
// "Hobb, Robin (1952-....). Auteur du texte" → "Hobb, Robin". The sort ("Last,
// First") form is preserved on purpose: the UI offers a one-click flip.
func bnfCreators(creators []string) []string {
	out := make([]string, 0, len(creators))
	seen := map[string]bool{}
	for _, c := range creators {
		name := bnfCleanCreator(c)
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		out = append(out, name)
	}
	return out
}

func bnfCleanCreator(s string) string {
	if i := strings.Index(s, "("); i >= 0 { // strip "(1952-....)"
		s = s[:i]
	}
	if i := strings.Index(s, ". "); i >= 0 { // strip ". Auteur du texte"
		s = s[:i]
	}
	return strings.Trim(strings.TrimSpace(s), ".,; ")
}

// bnfPublisher drops the trailing place BnF appends, e.g. "J'ai lu (Paris)".
func bnfPublisher(s string) string {
	if i := strings.LastIndex(s, " ("); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// bnfYear keeps the four-digit year out of a possibly noisy date string.
func bnfYear(s string) string {
	for i := 0; i+4 <= len(s); i++ {
		sub := s[i : i+4]
		if isFourDigits(sub) {
			return sub
		}
	}
	return strings.TrimSpace(s)
}

func isFourDigits(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// bnfArk returns the ark identifier if present, for a stable UI key.
func bnfArk(ids []string) string {
	for _, id := range ids {
		if strings.Contains(id, "ark:") {
			return id
		}
	}
	return first(ids)
}

func first(ss []string) string {
	if len(ss) > 0 {
		return ss[0]
	}
	return ""
}

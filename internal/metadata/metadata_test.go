package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeLang(t *testing.T) {
	cases := map[string]string{
		"fr":    "fr",
		"FR":    "fr",
		"fr-FR": "fr",
		"fr_CA": "fr",
		"fre":   "fr",
		"fra":   "fr",
		"eng":   "en",
		"ger":   "de",
		"":      "",
		"xyz":   "xyz",
	}
	for in, want := range cases {
		if got := normalizeLang(in); got != want {
			t.Errorf("normalizeLang(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStripHTML(t *testing.T) {
	cases := map[string]string{
		"plain text":                          "plain text",
		"<p>Un&nbsp;r&eacute;sum&eacute;</p>": "Un résumé",
		"line<br/>break":                      "line\nbreak",
		"<b>bold</b> &amp; more":              "bold & more",
	}
	for in, want := range cases {
		if got := stripHTML(in); got != want {
			t.Errorf("stripHTML(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGoogleBooksSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "intitle:L'assassin royal inauthor:Robin Hobb" {
			t.Errorf("unexpected q=%q", got)
		}
		w.Write([]byte(`{"items":[
			{"id":"g1","volumeInfo":{
				"title":"L'assassin royal","authors":["Robin Hobb"],
				"publisher":"Pygmalion","publishedDate":"1998","language":"fr",
				"description":"Fitz...","categories":["Fantasy"],
				"industryIdentifiers":[{"type":"ISBN_10","identifier":"2857046782"},{"type":"ISBN_13","identifier":"9782857046783"}],
				"imageLinks":{"thumbnail":"http://books.google.com/books/content?id=x&zoom=1&edge=curl"}
			}}
		]}`))
	}))
	defer srv.Close()

	g := NewGoogleBooks(srv.URL)
	cands, err := g.Search(context.Background(), Query{Title: "L'assassin royal", Authors: []string{"Robin Hobb"}}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 {
		t.Fatalf("got %d candidates, want 1", len(cands))
	}
	c := cands[0]
	if c.ISBN != "9782857046783" {
		t.Errorf("ISBN = %q, want the ISBN-13", c.ISBN)
	}
	if c.Language != "fr" {
		t.Errorf("Language = %q, want fr", c.Language)
	}
	if c.CoverURL != "https://books.google.com/books/content?id=x&zoom=1" {
		t.Errorf("CoverURL not cleaned: %q", c.CoverURL)
	}
}

func TestOpenLibraryTextSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search.json" {
			t.Errorf("text search hit %q, want /search.json", r.URL.Path)
		}
		w.Write([]byte(`{"docs":[
			{"key":"/works/OL1W","title":"Assassin's Apprentice","author_name":["Robin Hobb"],
			 "first_publish_year":1995,"isbn":["0553573403","9780553573404"],
			 "language":["eng"],"cover_i":42,"publisher":["Bantam"],"subject":["Fantasy","Magic"]}
		]}`))
	}))
	defer srv.Close()

	o := NewOpenLibrary(srv.URL)
	cands, err := o.Search(context.Background(), Query{Title: "Assassin's Apprentice"}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 {
		t.Fatalf("got %d candidates, want 1", len(cands))
	}
	c := cands[0]
	if c.ISBN != "9780553573404" {
		t.Errorf("ISBN = %q, want the ISBN-13", c.ISBN)
	}
	if c.Language != "en" {
		t.Errorf("Language = %q, want en", c.Language)
	}
	if c.CoverURL != "https://covers.openlibrary.org/b/id/42-L.jpg" {
		t.Errorf("CoverURL = %q", c.CoverURL)
	}
}

// TestOpenLibraryISBNUsesEditionAPI guards the fix for the bug where an ISBN
// search returned the wrong edition's ISBN/cover: an ISBN query must hit the
// edition-precise /api/books endpoint and echo the queried edition.
func TestOpenLibraryISBNUsesEditionAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/books" {
			t.Errorf("ISBN search hit %q, want /api/books", r.URL.Path)
		}
		if got := r.URL.Query().Get("bibkeys"); got != "ISBN:9780141439600" {
			t.Errorf("bibkeys = %q", got)
		}
		w.Write([]byte(`{"ISBN:9780141439600":{
			"key":"/books/OL3703019M","title":"A Tale of Two Cities",
			"authors":[{"name":"Charles Dickens"}],
			"publishers":[{"name":"Penguin Books"}],"publish_date":"2003",
			"cover":{"large":"https://covers.openlibrary.org/b/id/8493695-L.jpg"},
			"identifiers":{"isbn_13":["9780141439600"],"isbn_10":["0141439602"]},
			"subjects":[{"name":"English fiction"}],"number_of_pages":488
		}}`))
	}))
	defer srv.Close()

	o := NewOpenLibrary(srv.URL)
	cands, err := o.Search(context.Background(), Query{ISBN: "978-0-14-143960-0"}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 {
		t.Fatalf("got %d candidates, want 1", len(cands))
	}
	c := cands[0]
	if c.ISBN != "9780141439600" {
		t.Errorf("ISBN = %q, want the queried edition's ISBN", c.ISBN)
	}
	if c.CoverURL != "https://covers.openlibrary.org/b/id/8493695-L.jpg" {
		t.Errorf("CoverURL = %q, want the edition cover", c.CoverURL)
	}
	if c.Publisher != "Penguin Books" || c.PageCount != 488 {
		t.Errorf("unexpected edition data: %+v", c)
	}
}

func TestBnFSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("recordSchema"); got != "dublincore" {
			t.Errorf("recordSchema = %q", got)
		}
		w.Write([]byte(`<srw:searchRetrieveResponse xmlns:srw="http://www.loc.gov/zing/srw/">
			<srw:records><srw:record><srw:recordData>
				<oai_dc:dc xmlns:oai_dc="http://www.openarchives.org/OAI/2.0/oai_dc/" xmlns:dc="http://purl.org/dc/elements/1.1/">
					<dc:identifier>http://catalogue.bnf.fr/ark:/12148/cb412481349</dc:identifier>
					<dc:title>L'assassin royal / Robin Hobb</dc:title>
					<dc:creator>Hobb, Robin (1952-....). Auteur du texte</dc:creator>
					<dc:publisher>J'ai lu (Paris)</dc:publisher>
					<dc:date>2008</dc:date>
					<dc:language>fre</dc:language>
				</oai_dc:dc>
			</srw:recordData></srw:record></srw:records>
		</srw:searchRetrieveResponse>`))
	}))
	defer srv.Close()

	b := NewBnF(srv.URL)
	cands, err := b.Search(context.Background(), Query{Title: "L'assassin royal", Authors: []string{"Robin Hobb"}}, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 {
		t.Fatalf("got %d candidates, want 1", len(cands))
	}
	c := cands[0]
	if c.Title != "L'assassin royal" {
		t.Errorf("Title = %q, want the responsibility statement stripped", c.Title)
	}
	if len(c.Authors) != 1 || c.Authors[0] != "Hobb, Robin" {
		t.Errorf("Authors = %v, want cleaned [Hobb, Robin]", c.Authors)
	}
	if c.Publisher != "J'ai lu" {
		t.Errorf("Publisher = %q, want place stripped", c.Publisher)
	}
	if c.Published != "2008" || c.Language != "fr" {
		t.Errorf("Published/Language = %q/%q", c.Published, c.Language)
	}
}

func TestClientMergeAndRank(t *testing.T) {
	// Two providers return the same edition (same ISBN-13) in different
	// languages plus an English-only one; the FR-preferred ranking must float
	// the French edition first and the duplicate must be merged.
	frThenEn := fakeProvider{name: "a", cands: []Candidate{
		{Source: "a", Title: "Le livre", Language: "fr", ISBN: "9782857046783", CoverURL: "u"},
		{Source: "a", Title: "The Book", Language: "en", ISBN: "9780553573404"},
	}}
	dupEnrich := fakeProvider{name: "b", cands: []Candidate{
		{Source: "b", Title: "Le livre", Language: "fr", ISBN: "9782857046783", Description: "desc"},
	}}
	c := NewClientWith(nil, dupEnrich, frThenEn)

	cands, err := c.Search(context.Background(), Query{Language: "fr"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 2 {
		t.Fatalf("got %d candidates, want 2 after merge", len(cands))
	}
	if cands[0].Language != "fr" {
		t.Errorf("first candidate language = %q, want fr (preferred)", cands[0].Language)
	}
	if cands[0].Description != "desc" || cands[0].CoverURL != "u" {
		t.Errorf("merge did not combine fields: %+v", cands[0])
	}
}

func TestClientAllProvidersFail(t *testing.T) {
	c := NewClientWith(nil, fakeProvider{name: "x", err: &httpError{provider: "x", status: 500}})
	if _, err := c.Search(context.Background(), Query{Title: "t"}); err == nil {
		t.Fatal("expected error when every provider fails")
	}
}

func TestClientPartialFailure(t *testing.T) {
	ok := fakeProvider{name: "ok", cands: []Candidate{{Source: "ok", Title: "t"}}}
	bad := fakeProvider{name: "bad", err: &httpError{provider: "bad", status: 500}}
	c := NewClientWith(nil, bad, ok)
	cands, err := c.Search(context.Background(), Query{Title: "t"})
	if err != nil {
		t.Fatalf("partial failure should not error: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("got %d candidates, want 1", len(cands))
	}
}

type fakeProvider struct {
	name  string
	cands []Candidate
	err   error
}

func (f fakeProvider) Name() string { return f.name }
func (f fakeProvider) Search(context.Context, Query, *http.Client) ([]Candidate, error) {
	return f.cands, f.err
}

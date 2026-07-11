package gutenberg

import (
	"context"
	"strings"
	"testing"
)

const sampleCSV = `Text#,Type,Issued,Title,Language,Authors,Subjects,LoCC,Bookshelves
796,Text,1998-05-01,La Chartreuse de Parme,fr,"Stendhal, 1783-1842","Love stories; Italy -- Fiction",PQ,
164,Text,1994-09-01,Twenty Thousand Leagues under the Sea,en,"Verne, Jules, 1828-1905","Science fiction; Adventure",PQ,
14287,Text,2004-12-01,L'île mystérieuse,fr,"Verne, Jules, 1828-1905","Adventure stories",PQ,
9999,Sound,2010-01-01,An Audiobook,en,"Reader, A.","Spoken",PQ,
2000,Text,2000-01-01,Don Quijote,es,"Cervantes Saavedra, Miguel de, 1547-1616","Fiction",PQ,`

func loadSample(t *testing.T) *Catalog {
	t.Helper()
	entries, byID, err := parseCatalog(strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatal(err)
	}
	return &Catalog{entries: entries, byID: byID}
}

func TestParseSkipsNonTextAndCleansAuthors(t *testing.T) {
	c := loadSample(t)
	// The audiobook row must be dropped; 4 text books remain.
	if len(c.entries) != 4 {
		t.Fatalf("got %d entries, want 4 (audiobook excluded)", len(c.entries))
	}
	e, ok := c.byID[164]
	if !ok {
		t.Fatal("book 164 missing")
	}
	if len(e.authors) != 1 || e.authors[0] != "Verne, Jules" {
		t.Errorf("authors = %v, want [Verne, Jules] (life dates stripped)", e.authors)
	}
}

func TestSearchLanguageFilter(t *testing.T) {
	c := loadSample(t)
	res, err := c.Search(context.Background(), Query{Search: "verne", Languages: []string{"fr"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 1 || len(res.Books) != 1 {
		t.Fatalf("got %d results, want 1 (French Verne only)", res.Count)
	}
	if res.Books[0].ID != 14287 {
		t.Errorf("id = %d, want 14287", res.Books[0].ID)
	}
	if res.Books[0].CoverURL == "" || res.Books[0].EPUBURL == "" {
		t.Error("expected constructed cover and epub URLs")
	}
}

func TestSearchTokensMatchTitleAndAuthor(t *testing.T) {
	c := loadSample(t)
	// Two Verne books across languages, no language filter.
	res, _ := c.Search(context.Background(), Query{Search: "verne"})
	if res.Count != 2 {
		t.Fatalf("got %d, want 2 Verne books", res.Count)
	}
	// A token that only appears in a title.
	res, _ = c.Search(context.Background(), Query{Search: "chartreuse"})
	if res.Count != 1 || res.Books[0].ID != 796 {
		t.Fatalf("chartreuse search wrong: %+v", res)
	}
}

func TestSearchPagination(t *testing.T) {
	c := loadSample(t)
	// Force a tiny page by asking for page 2 of an all-match query; with 4
	// results and pageSize=36 there is no page 2.
	res, _ := c.Search(context.Background(), Query{})
	if res.Count != 4 || res.HasNext || res.HasPrev {
		t.Fatalf("unexpected paging: %+v", res)
	}
}

func TestBookByID(t *testing.T) {
	c := loadSample(t)
	b, ok, err := c.Book(context.Background(), 796)
	if err != nil || !ok {
		t.Fatalf("Book(796): ok=%v err=%v", ok, err)
	}
	if b.Title != "La Chartreuse de Parme" || b.Languages[0] != "fr" {
		t.Errorf("book = %+v", b)
	}
	if _, ok, _ := c.Book(context.Background(), 111111); ok {
		t.Error("expected missing book to return ok=false")
	}
}

func TestURLConventions(t *testing.T) {
	if coverURL(796) != "https://www.gutenberg.org/cache/epub/796/pg796.cover.medium.jpg" {
		t.Errorf("coverURL = %q", coverURL(796))
	}
	got := epubURLs(796)
	if got[0] != "https://www.gutenberg.org/ebooks/796.epub3.images" {
		t.Errorf("epubURLs[0] = %q", got[0])
	}
	if len(got) < 2 {
		t.Error("expected fallback EPUB URLs")
	}
}

func TestCleanAuthors(t *testing.T) {
	cases := map[string][]string{
		"Verne, Jules, 1828-1905":                             {"Verne, Jules"},
		"Stendhal, 1783-1842":                                 {"Stendhal"},
		"Dickens, Charles, 1812-1870; Leech, John, 1817-1864": {"Dickens, Charles", "Leech, John"},
		"Anonymous": {"Anonymous"},
	}
	for in, want := range cases {
		got := cleanAuthors(in)
		if len(got) != len(want) {
			t.Errorf("cleanAuthors(%q) = %v, want %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("cleanAuthors(%q)[%d] = %q, want %q", in, i, got[i], want[i])
			}
		}
	}
}

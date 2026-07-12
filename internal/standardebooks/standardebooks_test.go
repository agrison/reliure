package standardebooks

import (
	"context"
	"strings"
	"testing"
)

const sampleCatalog = `[
  {
    "name": "jane-austen_pride-and-prejudice",
    "full_name": "standardebooks/jane-austen_pride-and-prejudice",
    "description": "Standard Ebooks edition of Pride and Prejudice, by Jane Austen.",
    "fork": false,
    "archived": false
  },
  {
    "name": "jules-verne_twenty-thousand-leagues-under-the-sea_f-p-walter",
    "full_name": "standardebooks/jules-verne_twenty-thousand-leagues-under-the-sea_f-p-walter",
    "description": "Standard Ebooks edition of Twenty Thousand Leagues Under the Sea, by Jules Verne, translated by F. P. Walter.",
    "fork": false,
    "archived": false
  },
  {
    "name": "website",
    "full_name": "standardebooks/website",
    "description": "The Standard Ebooks website.",
    "fork": false,
    "archived": false
  }
]`

func loadSample(t *testing.T) *Catalog {
	t.Helper()
	entries, byID, err := parseCatalog(strings.NewReader(sampleCatalog))
	if err != nil {
		t.Fatal(err)
	}
	return &Catalog{entries: entries, byID: byID}
}

func TestParseCatalog(t *testing.T) {
	c := loadSample(t)
	if len(c.entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(c.entries))
	}
	b, ok := c.byID["jane-austen_pride-and-prejudice"]
	if !ok {
		t.Fatal("missing Pride and Prejudice")
	}
	if b.title != "Pride and Prejudice" {
		t.Errorf("title = %q", b.title)
	}
	if len(b.authors) != 1 || b.authors[0] != "Jane Austen" {
		t.Errorf("authors = %v, want [Jane Austen]", b.authors)
	}
	if b.coverURL != "https://standardebooks.org/ebooks/jane-austen/pride-and-prejudice/downloads/cover.jpg" {
		t.Errorf("coverURL = %q", b.coverURL)
	}
	if b.epubURL != "https://standardebooks.org/ebooks/jane-austen/pride-and-prejudice/downloads/jane-austen_pride-and-prejudice.epub" {
		t.Errorf("epubURL = %q", b.epubURL)
	}
}

func TestSearch(t *testing.T) {
	c := loadSample(t)
	res, err := c.Search(context.Background(), Query{Search: "verne walter", Languages: []string{"en"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 1 || len(res.Books) != 1 {
		t.Fatalf("got %+v, want one Verne result", res)
	}
	if res.Books[0].ID != "jules-verne_twenty-thousand-leagues-under-the-sea_f-p-walter" {
		t.Errorf("id = %q", res.Books[0].ID)
	}
}

func TestBookByID(t *testing.T) {
	c := loadSample(t)
	b, ok, err := c.Book(context.Background(), "jane-austen_pride-and-prejudice")
	if err != nil || !ok {
		t.Fatalf("Book: ok=%v err=%v", ok, err)
	}
	if b.Title != "Pride and Prejudice" || b.EPUBURL == "" {
		t.Errorf("book = %+v", b)
	}
}

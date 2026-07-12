package core

import (
	"strings"
	"testing"
)

func TestContentSnippetsReturnHighlightedFragments(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	b := &Book{
		Title: "Livre source",
		Files: []File{{Path: "/tmp/source.epub", Format: "epub"}},
	}
	if err := db.Books.Create(b); err != nil {
		t.Fatal(err)
	}
	if err := db.Content.Upsert(b.ID, b.Files[0].ID, []ContentFragment{
		{Page: 2, Text: "Premier passage avec une aiguille dans la botte."},
		{Page: 4, Text: "Deuxieme passage avec une autre aiguille visible."},
	}); err != nil {
		t.Fatal(err)
	}

	hits, err := db.Content.Snippets("aiguille", SearchScope{}, 10, 2, "minimal")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2: %+v", len(hits), hits)
	}
	if hits[0].BookID != b.ID || hits[0].Page != 2 {
		t.Fatalf("first hit = %+v", hits[0])
	}
	if !strings.Contains(hits[0].Snippet, "[[[") || !strings.Contains(hits[0].Snippet, "]]]") {
		t.Fatalf("snippet is not highlighted: %q", hits[0].Snippet)
	}
}

func TestContentOccurrencesExpandMultipleMentionsInOneFragment(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	b := &Book{
		Title: "Fragment dense",
		Files: []File{{Path: "/tmp/dense.epub", Format: "epub"}},
	}
	if err := db.Books.Create(b); err != nil {
		t.Fatal(err)
	}
	if err := db.Content.Upsert(b.ID, b.Files[0].ID, []ContentFragment{
		{Page: 1, Text: "Burrich arrive. Puis Burrich repart. Enfin Burrich revient."},
	}); err != nil {
		t.Fatal(err)
	}

	page, err := db.Content.Occurrences("Burrich", SearchScope{}, 1, 10, "minimal")
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Hits) != 3 {
		t.Fatalf("occurrences total=%d hits=%d, want 3/3: %+v", page.Total, len(page.Hits), page.Hits)
	}
	for _, h := range page.Hits {
		if !strings.Contains(h.Snippet, "[[[Burrich]]]") {
			t.Fatalf("snippet is not highlighted: %q", h.Snippet)
		}
	}
}

func TestContentOccurrencesOrderSeriesByIndex(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	idx2 := 2.0
	idx1 := 1.0
	second := &Book{
		Title:       "Deux",
		Series:      &Series{Name: "Cycle"},
		SeriesIndex: &idx2,
		Files:       []File{{Path: "/tmp/deux.epub", Format: "epub"}},
	}
	first := &Book{
		Title:       "Un",
		Series:      &Series{Name: "Cycle"},
		SeriesIndex: &idx1,
		Files:       []File{{Path: "/tmp/un.epub", Format: "epub"}},
	}
	for _, b := range []*Book{second, first} {
		if err := db.Books.Create(b); err != nil {
			t.Fatal(err)
		}
		if err := db.Content.Upsert(b.ID, b.Files[0].ID, []ContentFragment{{Page: 1, Text: "motif commun"}}); err != nil {
			t.Fatal(err)
		}
	}

	page, err := db.Content.Occurrences("motif", SearchScope{}, 1, 10, "minimal")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Hits) != 2 {
		t.Fatalf("hits = %+v", page.Hits)
	}
	if page.Hits[0].Title != "Un" || page.Hits[1].Title != "Deux" {
		t.Fatalf("series order = %q then %q, want Un then Deux", page.Hits[0].Title, page.Hits[1].Title)
	}
	if page.Hits[0].Series != "Cycle" || page.Hits[0].SeriesIndex != 1 {
		t.Fatalf("series fields missing: %+v", page.Hits[0])
	}
}

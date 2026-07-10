package core

import "testing"

func TestSetTitleSortAndClear(t *testing.T) {
	db := newTestDB(t)
	b := &Book{Title: "Foundation", TitleSort: "Foundation"}
	if err := db.Books.Create(b); err != nil {
		t.Fatal(err)
	}

	if err := db.Books.SetTitleSort(b.ID, "  Fondation, La  "); err != nil {
		t.Fatal(err)
	}
	got, _ := db.Books.ByID(b.ID)
	if got.TitleSort != "Fondation, La" {
		t.Errorf("title sort = %q (should be trimmed)", got.TitleSort)
	}

	// Clearing is the key case: it must actually empty the field.
	if err := db.Books.SetTitleSort(b.ID, ""); err != nil {
		t.Fatal(err)
	}
	got, _ = db.Books.ByID(b.ID)
	if got.TitleSort != "" {
		t.Errorf("title sort = %q, want empty after clear", got.TitleSort)
	}
}

func TestSetAuthorSortAndClear(t *testing.T) {
	db := newTestDB(t)
	b := &Book{Title: "X", Authors: []Contribution{{Author: Author{Name: "Ursula K. Le Guin"}}}}
	if err := db.Books.Create(b); err != nil {
		t.Fatal(err)
	}
	id := b.Authors[0].Author.ID

	if err := db.Authors.SetSortName(id, "Le Guin, Ursula K."); err != nil {
		t.Fatal(err)
	}
	a, _ := db.Authors.ByID(id)
	if a.SortName != "Le Guin, Ursula K." {
		t.Errorf("sort name = %q", a.SortName)
	}

	if err := db.Authors.SetSortName(id, ""); err != nil {
		t.Fatal(err)
	}
	a, _ = db.Authors.ByID(id)
	if a.SortName != "" {
		t.Errorf("sort name = %q, want empty after clear", a.SortName)
	}
}

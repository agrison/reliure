package core

import "testing"

func TestBrowseSortByAdded(t *testing.T) {
	db := newTestDB(t)
	for _, ti := range []string{"First", "Second", "Third"} {
		if err := db.Books.Create(&Book{Title: ti}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := db.Books.Browse("added", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Most recently added first.
	if len(got) != 3 || got[0].Title != "Third" || got[2].Title != "First" {
		t.Fatalf("added order = %v", titles(got))
	}
}

func TestBrowseSortByAuthor(t *testing.T) {
	db := newTestDB(t)
	mk := func(title, author string) {
		b := &Book{Title: title, Authors: []Contribution{{Author: Author{Name: author}}}}
		if err := db.Books.Create(b); err != nil {
			t.Fatal(err)
		}
	}
	mk("Book Z", "Zelazny, Roger") // sort_name derived: "Zelazny, Roger" → last word "Roger"? see below
	mk("Book A", "Isaac Asimov")

	got, err := db.Books.Browse("author", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Asimov (sort "Asimov, Isaac") sorts before "Roger, Zelazny," derived key.
	if len(got) != 2 || got[0].Authors[0].Author.Name != "Isaac Asimov" {
		t.Fatalf("author order = %v", titles(got))
	}
}

func TestListByTag(t *testing.T) {
	db := newTestDB(t)
	if err := db.Books.Create(&Book{Title: "Tagged", Tags: []Tag{{Name: "sci-fi"}}}); err != nil {
		t.Fatal(err)
	}
	if err := db.Books.Create(&Book{Title: "Untagged"}); err != nil {
		t.Fatal(err)
	}
	tag, err := db.Tags.ByName("sci-fi")
	if err != nil {
		t.Fatal(err)
	}
	got, err := db.Books.ListByTag(tag.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "Tagged" {
		t.Fatalf("by tag = %v", titles(got))
	}
}

func TestCounts(t *testing.T) {
	db := newTestDB(t)
	mk := func(title, author, series, tag string) {
		b := &Book{
			Title:   title,
			Authors: []Contribution{{Author: Author{Name: author}}},
			Series:  &Series{Name: series},
			Tags:    []Tag{{Name: tag}},
		}
		if err := db.Books.Create(b); err != nil {
			t.Fatal(err)
		}
	}
	mk("A", "Asimov", "Foundation", "classic")
	mk("B", "Asimov", "Foundation", "classic")
	mk("C", "Herbert", "Dune", "epic")

	authors, err := db.Authors.Counts()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]int{}
	for _, a := range authors {
		byName[a.Name] = a.Count
	}
	if byName["Asimov"] != 2 || byName["Herbert"] != 1 {
		t.Errorf("author counts = %v", byName)
	}

	series, _ := db.Series.Counts()
	sName := map[string]int{}
	for _, s := range series {
		sName[s.Name] = s.Count
	}
	if sName["Foundation"] != 2 || sName["Dune"] != 1 {
		t.Errorf("series counts = %v", sName)
	}

	tags, _ := db.Tags.Counts()
	tName := map[string]int{}
	for _, tg := range tags {
		tName[tg.Name] = tg.Count
	}
	if tName["classic"] != 2 || tName["epic"] != 1 {
		t.Errorf("tag counts = %v", tName)
	}
}

package main

import (
	"testing"

	"github.com/agrison/reliure/internal/core"
)

func TestSmartShelfFiltersBooks(t *testing.T) {
	db := openAppTestDB(t)
	sf := appBook(t, db, "Dune", []string{"Frank Herbert"}, []string{"SF"})
	appBook(t, db, "Poésie", []string{"Someone"}, []string{"Poetry"})
	if err := db.Reading.UpsertState(core.ReadingState{BookID: sf.ID, Status: "reading", Percent: 0.4}); err != nil {
		t.Fatal(err)
	}
	svc := &LibraryService{db: db}

	sh, err := svc.SaveSmartShelf(SmartShelfInput{
		Name:  "SF en cours",
		Match: "all",
		Rules: []SmartShelfRule{
			{Field: "tag", Operator: "is", Value: "SF"},
			{Field: "reading_status", Operator: "is", Value: "reading"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sh.Count != 1 {
		t.Fatalf("shelf count = %d, want 1", sh.Count)
	}
	books, err := svc.BooksBySmartShelf(sh.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 || books[0].Title != "Dune" {
		t.Fatalf("books = %+v", books)
	}
}

func openAppTestDB(t *testing.T) *core.DB {
	t.Helper()
	db, err := core.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSmartShelfUnreadAndRecent(t *testing.T) {
	db := openAppTestDB(t)
	appBook(t, db, "Unread", nil, nil)
	read := appBook(t, db, "Read", nil, nil)
	if err := db.Reading.UpsertState(core.ReadingState{BookID: read.ID, Status: "complete", Percent: 1}); err != nil {
		t.Fatal(err)
	}
	svc := &LibraryService{db: db}
	sh, err := svc.SaveSmartShelf(SmartShelfInput{
		Name:  "Non lus récents",
		Match: "all",
		Rules: []SmartShelfRule{
			{Field: "reading_status", Operator: "is", Value: "unread"},
			{Field: "added_within_days", Operator: "is", Value: "30"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sh.Count != 1 {
		t.Fatalf("count = %d, want 1", sh.Count)
	}
}

func appBook(t *testing.T, db *core.DB, title string, authors []string, tags []string) *core.Book {
	t.Helper()
	b := &core.Book{Title: title}
	for i, a := range authors {
		b.Authors = append(b.Authors, core.Contribution{Author: core.Author{Name: a}, Role: "aut", Position: i})
	}
	for _, tag := range tags {
		b.Tags = append(b.Tags, core.Tag{Name: tag})
	}
	if err := db.Books.Create(b); err != nil {
		t.Fatal(err)
	}
	got, err := db.Books.ByID(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

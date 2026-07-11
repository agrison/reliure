package main

import (
	"testing"

	"github.com/agrison/reliure/internal/core"
)

func TestDashboardAggregates(t *testing.T) {
	db, err := core.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	mk := func(title, lang, author string, size int64) *core.Book {
		b := &core.Book{
			Title:    title,
			Language: lang,
			Authors:  []core.Contribution{{Author: core.Author{Name: author}, Role: "aut"}},
			Files:    []core.File{{Path: "/lib/" + title + ".epub", Format: "epub", Size: size}},
		}
		if err := db.Books.Create(b); err != nil {
			t.Fatal(err)
		}
		return b
	}
	a := mk("A", "fr", "Hobb", 1000)
	mk("B", "fr", "Hobb", 2000)
	mk("C", "en", "Verne", 3000)

	// One book finished; the rest unread.
	if err := db.Reading.UpsertState(core.ReadingState{BookID: a.ID, Percent: 1, Status: "complete"}); err != nil {
		t.Fatal(err)
	}

	svc := &StatsService{db: db}
	d, err := svc.Dashboard()
	if err != nil {
		t.Fatal(err)
	}

	if d.Books != 3 || d.Authors != 2 || d.Files != 3 {
		t.Errorf("counts: books=%d authors=%d files=%d", d.Books, d.Authors, d.Files)
	}
	if d.TotalSize != 6000 {
		t.Errorf("total size = %d, want 6000", d.TotalSize)
	}
	if len(d.Formats) != 1 || d.Formats[0].Name != "epub" || d.Formats[0].Count != 3 {
		t.Errorf("formats = %+v", d.Formats)
	}
	// Languages ordered by count desc: fr(2) before en(1).
	if len(d.Languages) != 2 || d.Languages[0].Name != "fr" || d.Languages[0].Count != 2 {
		t.Errorf("languages = %+v", d.Languages)
	}
	// Top author by count: Hobb(2) first.
	if len(d.TopAuthors) == 0 || d.TopAuthors[0].Name != "Hobb" || d.TopAuthors[0].Count != 2 {
		t.Errorf("top authors = %+v", d.TopAuthors)
	}
	if d.Reading.Complete != 1 || d.Reading.Unread != 2 {
		t.Errorf("reading = %+v", d.Reading)
	}
}

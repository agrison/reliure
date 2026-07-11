package main

import (
	"path/filepath"
	"testing"

	"github.com/agrison/reliure/internal/core"
	"github.com/agrison/reliure/internal/settings"
)

func newLibraryServiceForReading(t *testing.T) (*LibraryService, int64) {
	t.Helper()
	db, err := core.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store, err := settings.Open(filepath.Join(t.TempDir(), "settings.json"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b := &core.Book{Title: "Book"}
	if err := db.Books.Create(b); err != nil {
		t.Fatal(err)
	}
	return &LibraryService{db: db, settings: store}, b.ID
}

func TestSetReadingStateManual(t *testing.T) {
	svc, id := newLibraryServiceForReading(t)

	// Mark "reading" with an explicit percentage.
	d, err := svc.SetReadingState(ReadingUpdate{BookID: id, Status: "reading", Percent: 0.25})
	if err != nil {
		t.Fatal(err)
	}
	if d.ReadingStatus != "reading" || d.Percent != 0.25 {
		t.Errorf("detail = %+v", d)
	}

	// Set progress by page: 120 / 240 → 0.5, and total pages stored.
	d, err = svc.SetReadingState(ReadingUpdate{BookID: id, Page: 120, TotalPages: 240})
	if err != nil {
		t.Fatal(err)
	}
	if d.Pages != 240 || d.Percent != 0.5 || d.ReadingStatus != "reading" {
		t.Errorf("page-based detail = %+v", d)
	}

	// Mark complete → percent forced to 1.
	d, _ = svc.SetReadingState(ReadingUpdate{BookID: id, Status: "complete"})
	if d.ReadingStatus != "complete" || d.Percent != 1 {
		t.Errorf("complete detail = %+v", d)
	}

	// Clear → tracking removed.
	d, _ = svc.SetReadingState(ReadingUpdate{BookID: id, Clear: true})
	if d.ReadingStatus != "" || d.Percent != 0 {
		t.Errorf("cleared detail = %+v", d)
	}
	if _, ok, _ := svc.db.Reading.State(id); ok {
		t.Error("state should be deleted after clear")
	}
}

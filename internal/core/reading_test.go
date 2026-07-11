package core

import "testing"

// readingTestBook inserts a minimal book and returns its id.
func readingTestBook(t *testing.T, db *DB) int64 {
	t.Helper()
	b := &Book{Title: "Book", Authors: []Contribution{{Author: Author{Name: "Author"}, Role: "aut"}}}
	if err := db.Books.Create(b); err != nil {
		t.Fatalf("create book: %v", err)
	}
	return b.ID
}

func TestReadingStateUpsert(t *testing.T) {
	db := newTestDB(t)
	id := readingTestBook(t, db)

	if err := db.Reading.UpsertState(ReadingState{BookID: id, Percent: 0.3, Status: "reading", LastReadAt: "2026-07-10"}); err != nil {
		t.Fatal(err)
	}
	// Re-sync with new progress overwrites, not duplicates.
	if err := db.Reading.UpsertState(ReadingState{BookID: id, Percent: 0.8, Status: "reading"}); err != nil {
		t.Fatal(err)
	}
	st, ok, err := db.Reading.State(id)
	if err != nil || !ok {
		t.Fatalf("State: ok=%v err=%v", ok, err)
	}
	if st.Percent != 0.8 {
		t.Errorf("percent = %v, want 0.8 (upsert)", st.Percent)
	}
}

func TestStatusCountsAndListByStatus(t *testing.T) {
	db := newTestDB(t)
	reading := readingTestBook(t, db)
	done := readingTestBook(t, db)
	if err := db.Reading.UpsertState(ReadingState{BookID: reading, Percent: 0.3, Pages: 200, Status: "reading"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Reading.UpsertState(ReadingState{BookID: done, Percent: 1, Status: "complete"}); err != nil {
		t.Fatal(err)
	}

	counts, err := db.Reading.StatusCounts()
	if err != nil {
		t.Fatal(err)
	}
	if counts["reading"] != 1 || counts["complete"] != 1 || counts["abandoned"] != 0 {
		t.Errorf("counts = %v", counts)
	}

	// Pages round-trips.
	st, _, _ := db.Reading.State(reading)
	if st.Pages != 200 {
		t.Errorf("pages = %d, want 200", st.Pages)
	}

	inProgress, err := db.Books.ListByReadingStatus("reading")
	if err != nil {
		t.Fatal(err)
	}
	if len(inProgress) != 1 || inProgress[0].ID != reading {
		t.Errorf("ListByReadingStatus(reading) = %v", inProgress)
	}
}

func TestReplaceAnnotationsIdempotent(t *testing.T) {
	db := newTestDB(t)
	id := readingTestBook(t, db)

	anns := []Annotation{
		{BookID: id, Text: "highlight one", Chapter: "Ch1", CreatedAt: "2026-07-09 10:00:00"},
		{BookID: id, Text: "highlight two", Note: "a note", CreatedAt: "2026-07-09 11:00:00"},
		// Exact duplicate of the first — dedup_key must collapse it.
		{BookID: id, Text: "highlight one", Chapter: "Ch1", CreatedAt: "2026-07-09 10:00:00"},
	}
	if err := db.Reading.ReplaceAnnotations(id, anns); err != nil {
		t.Fatal(err)
	}
	got, err := db.Reading.Annotations(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d annotations, want 2 (dupe collapsed)", len(got))
	}

	// Replacing again with a smaller set reflects the device state exactly.
	if err := db.Reading.ReplaceAnnotations(id, anns[:1]); err != nil {
		t.Fatal(err)
	}
	got, _ = db.Reading.Annotations(id)
	if len(got) != 1 {
		t.Fatalf("got %d after replace, want 1", len(got))
	}

	counts, err := db.Reading.AnnotationCounts()
	if err != nil {
		t.Fatal(err)
	}
	if counts[id] != 1 {
		t.Errorf("count = %d, want 1", counts[id])
	}
}

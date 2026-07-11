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

func TestMergeDeviceStateOneDirectional(t *testing.T) {
	db := newTestDB(t)
	id := readingTestBook(t, db)

	// Empty → device state is taken.
	if err := db.Reading.MergeDeviceState(ReadingState{BookID: id, Percent: 0.3, Pages: 300, Status: "reading"}); err != nil {
		t.Fatal(err)
	}
	st, _, _ := db.Reading.State(id)
	if st.Percent != 0.3 {
		t.Fatalf("percent = %v, want 0.3", st.Percent)
	}

	// A more advanced device state moves it forward.
	_ = db.Reading.MergeDeviceState(ReadingState{BookID: id, Percent: 0.7, Status: "reading"})
	if st, _, _ = db.Reading.State(id); st.Percent != 0.7 {
		t.Errorf("percent = %v, want 0.7 (advanced)", st.Percent)
	}

	// A LOWER device state must NOT roll it back.
	_ = db.Reading.MergeDeviceState(ReadingState{BookID: id, Percent: 0.2, Status: "reading"})
	if st, _, _ = db.Reading.State(id); st.Percent != 0.7 {
		t.Errorf("percent = %v, want 0.7 (no rollback)", st.Percent)
	}

	// A manual "complete" outranks any partial device percentage.
	_ = db.Reading.UpsertState(ReadingState{BookID: id, Percent: 1, Status: "complete"})
	_ = db.Reading.MergeDeviceState(ReadingState{BookID: id, Percent: 0.9, Status: "reading"})
	if st, _, _ = db.Reading.State(id); st.Status != "complete" {
		t.Errorf("status = %q, want complete (device 90%% must not downgrade)", st.Status)
	}

	// Even when not more advanced, the device teaches the page count if missing.
	other := readingTestBook(t, db)
	_ = db.Reading.UpsertState(ReadingState{BookID: other, Percent: 0.5, Status: "reading"}) // pages 0
	_ = db.Reading.MergeDeviceState(ReadingState{BookID: other, Percent: 0.4, Pages: 250, Status: "reading"})
	if st, _, _ = db.Reading.State(other); st.Pages != 250 || st.Percent != 0.5 {
		t.Errorf("state = %+v, want pages 250 kept percent 0.5", st)
	}
}

func TestDeleteState(t *testing.T) {
	db := newTestDB(t)
	id := readingTestBook(t, db)
	_ = db.Reading.UpsertState(ReadingState{BookID: id, Percent: 0.5, Status: "reading"})
	if err := db.Reading.DeleteState(id); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := db.Reading.State(id); ok {
		t.Error("state should be gone after DeleteState")
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

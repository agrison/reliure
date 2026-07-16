package koreaderstats

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// buildStatsDB creates a statistics.sqlite3-shaped database with the given
// page-turn events and returns its path.
func buildStatsDB(t *testing.T, events []struct {
	book     int
	unixTime int64
	duration int64
}) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "statistics.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE book (id INTEGER PRIMARY KEY, title TEXT, authors TEXT, total_read_time INTEGER, total_read_pages INTEGER);
		CREATE TABLE page_stat_data (id_book INTEGER, page INTEGER, start_time INTEGER, duration INTEGER, total_pages INTEGER);
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO book (id, title, authors, total_read_time, total_read_pages) VALUES (1,'A','Auth A',0,120),(2,'B','Auth B',0,30)`); err != nil {
		t.Fatal(err)
	}
	page := 0
	for _, e := range events {
		page++
		if _, err := db.Exec(`INSERT INTO page_stat_data (id_book, page, start_time, duration, total_pages) VALUES (?,?,?,?,300)`,
			e.book, page, e.unixTime, e.duration); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestReadAggregates(t *testing.T) {
	// Three sessions on two distinct local days.
	day1 := time.Date(2026, 7, 13, 21, 0, 0, 0, time.Local) // a Monday, 21h
	day2 := time.Date(2026, 7, 14, 8, 0, 0, 0, time.Local)  // a Tuesday, 08h
	events := []struct {
		book     int
		unixTime int64
		duration int64
	}{
		{1, day1.Unix(), 100},
		{1, day1.Add(time.Minute).Unix(), 200},
		{2, day2.Unix(), 60},
	}
	st, err := Read(buildStatsDB(t, events))
	if err != nil {
		t.Fatal(err)
	}

	if st.TotalSeconds != 360 {
		t.Errorf("total seconds = %d, want 360", st.TotalSeconds)
	}
	if st.DaysRead != 2 {
		t.Errorf("days read = %d, want 2", st.DaysRead)
	}
	if st.Books != 2 || st.TotalPages != 150 {
		t.Errorf("books=%d pages=%d, want 2/150", st.Books, st.TotalPages)
	}
	if len(st.ByWeekday) != 7 || len(st.ByHour) != 24 {
		t.Fatalf("bucket lengths: weekday=%d hour=%d", len(st.ByWeekday), len(st.ByHour))
	}
	// Weekday buckets are Monday-first; verify the two sample days landed right.
	wd := func(tm time.Time) int { return (int(tm.Weekday()) + 6) % 7 } // Go Weekday: Sun=0
	if st.ByWeekday[wd(day1)] != 300 {
		t.Errorf("day1 weekday bucket = %d, want 300", st.ByWeekday[wd(day1)])
	}
	if st.ByWeekday[wd(day2)] != 60 {
		t.Errorf("day2 weekday bucket = %d, want 60", st.ByWeekday[wd(day2)])
	}
	// Hour buckets follow the local hour of each event.
	if st.ByHour[day1.Hour()] != 300 || st.ByHour[day2.Hour()] != 60 {
		t.Errorf("hour buckets wrong: %d @%dh, %d @%dh", st.ByHour[day1.Hour()], day1.Hour(), st.ByHour[day2.Hour()], day2.Hour())
	}
	// The busiest day is day1 with 300s.
	if st.LongestDay.Seconds != 300 {
		t.Errorf("longest day = %+v, want 300s", st.LongestDay)
	}
	if len(st.ByDay) != 2 {
		t.Errorf("by-day rows = %d, want 2", len(st.ByDay))
	}
	// Top books by time: book A (300s) before book B (60s).
	if len(st.TopBooks) != 2 {
		t.Fatalf("top books = %d, want 2", len(st.TopBooks))
	}
	if st.TopBooks[0].Title != "A" || st.TopBooks[0].Seconds != 300 {
		t.Errorf("top book[0] = %+v, want A/300", st.TopBooks[0])
	}
	if st.TopBooks[1].Title != "B" || st.TopBooks[1].Seconds != 60 {
		t.Errorf("top book[1] = %+v, want B/60", st.TopBooks[1])
	}

	// Per-day book breakdown: day1 has book A (300s), day2 has book B (60s).
	k1, k2 := day1.Format("2006-01-02"), day2.Format("2006-01-02")
	if got := st.DayBooks[k1]; len(got) != 1 || got[0].Title != "A" || got[0].Seconds != 300 {
		t.Errorf("dayBooks[%s] = %+v, want [A/300]", k1, got)
	}
	if got := st.DayBooks[k2]; len(got) != 1 || got[0].Title != "B" || got[0].Seconds != 60 {
		t.Errorf("dayBooks[%s] = %+v, want [B/60]", k2, got)
	}

	// Per-month breakdown: both sample days are in the same month here.
	ym := day1.Format("2006-01")
	if got := st.MonthBooks[ym]; len(got) != 2 {
		t.Errorf("monthBooks[%s] = %d entries, want 2 (books A and B)", ym, len(got))
	}
}

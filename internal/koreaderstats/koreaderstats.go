// Package koreaderstats reads KOReader's Statistics plugin database
// (statistics.sqlite3) and computes reading-time aggregates for the dashboard.
//
// The plugin records one row per page turn in page_stat_data(id_book, page,
// start_time, duration, ...), where start_time is a Unix timestamp (seconds) and
// duration is the seconds spent on that page. All time bucketing is done in the
// local timezone (via SQLite strftime '...localtime'), so "read on Mondays"
// reflects the user's own days.
package koreaderstats

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DayCount is reading time (seconds) on a given local calendar day.
type DayCount struct {
	Date    string `json:"date"` // YYYY-MM-DD, local time
	Seconds int64  `json:"seconds"`
}

// BookTime is how long a book was read and when it was last opened.
type BookTime struct {
	Title    string `json:"title"`
	Authors  string `json:"authors"`
	Seconds  int64  `json:"seconds"`
	LastRead string `json:"lastRead"` // YYYY-MM-DD, local time
}

// ReadingStats is the aggregated view of a KOReader statistics database.
type ReadingStats struct {
	TotalSeconds int64      `json:"totalSeconds"`
	TotalPages   int        `json:"totalPages"`
	DaysRead     int        `json:"daysRead"`
	Books        int        `json:"books"`
	LongestDay   DayCount   `json:"longestDay"`
	ByWeekday    []int64    `json:"byWeekday"` // length 7, index 0 = Monday … 6 = Sunday
	ByHour       []int64    `json:"byHour"`    // length 24, local hour of day
	ByDay        []DayCount `json:"byDay"`     // chronological
	TopBooks     []BookTime `json:"topBooks"`  // most-read books by time
	// DayBooks maps a local day (YYYY-MM-DD) to the books read that day and for
	// how long, most-read first. Bounded to a recent window (see dayBooksWindow).
	DayBooks map[string][]BookTime `json:"dayBooks"`
	// MonthBooks maps a local month (YYYY-MM) to the books read that month, in the
	// order they were first opened. Covers all history (for the year selector).
	MonthBooks map[string][]BookTime `json:"monthBooks"`
	FetchedAt  string                `json:"fetchedAt"` // set by the caller when cached
}

// dayBooksWindow bounds the per-day book breakdown to keep the cache lean; it
// comfortably covers the calendar heatmap's range.
const dayBooksWindow = 200 * 24 * time.Hour

// Read opens a copy of statistics.sqlite3 and computes the aggregates. The file
// should be a local copy (e.g. fetched from the device), not the live database.
func Read(path string) (*ReadingStats, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	st := &ReadingStats{
		ByWeekday: make([]int64, 7),
		ByHour:    make([]int64, 24),
	}

	if err := db.QueryRow(`
		SELECT COALESCE(SUM(duration), 0),
		       COUNT(DISTINCT strftime('%Y-%m-%d', start_time, 'unixepoch', 'localtime'))
		FROM page_stat_data`).Scan(&st.TotalSeconds, &st.DaysRead); err != nil {
		return nil, fmt.Errorf("totals: %w", err)
	}

	// book table may be absent on very old versions — tolerate it.
	_ = db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(total_read_pages), 0) FROM book`).Scan(&st.Books, &st.TotalPages)

	// Reading time per weekday. strftime('%w') is 0=Sunday … 6=Saturday; remap to
	// Monday-first so the histogram reads Mon → Sun.
	if err := eachRow(db, `
		SELECT CAST(strftime('%w', start_time, 'unixepoch', 'localtime') AS INTEGER), SUM(duration)
		FROM page_stat_data GROUP BY 1`, func(w int, secs int64) {
		if w >= 0 && w <= 6 {
			st.ByWeekday[(w+6)%7] += secs
		}
	}); err != nil {
		return nil, fmt.Errorf("weekday: %w", err)
	}

	if err := eachRow(db, `
		SELECT CAST(strftime('%H', start_time, 'unixepoch', 'localtime') AS INTEGER), SUM(duration)
		FROM page_stat_data GROUP BY 1`, func(h int, secs int64) {
		if h >= 0 && h <= 23 {
			st.ByHour[h] += secs
		}
	}); err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}

	rows, err := db.Query(`
		SELECT strftime('%Y-%m-%d', start_time, 'unixepoch', 'localtime') AS d, SUM(duration)
		FROM page_stat_data GROUP BY d ORDER BY d`)
	if err != nil {
		return nil, fmt.Errorf("days: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var dc DayCount
		if err := rows.Scan(&dc.Date, &dc.Seconds); err != nil {
			return nil, err
		}
		st.ByDay = append(st.ByDay, dc)
		if dc.Seconds > st.LongestDay.Seconds {
			st.LongestDay = dc
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	st.TopBooks = topBooks(db)
	st.DayBooks = dayBooks(db, time.Now().Add(-dayBooksWindow).Unix())
	st.MonthBooks = monthBooks(db)
	return st, nil
}

// monthBooks returns, per local month (all history), the books read that month
// in first-opened order. Best-effort on old schemas.
func monthBooks(db *sql.DB) map[string][]BookTime {
	rows, err := db.Query(`
		SELECT strftime('%Y-%m', p.start_time, 'unixepoch', 'localtime') AS ym,
		       COALESCE(b.title, ''), COALESCE(b.authors, ''), COALESCE(SUM(p.duration), 0) AS secs
		FROM page_stat_data p JOIN book b ON b.id = p.id_book
		GROUP BY ym, p.id_book
		HAVING secs > 0
		ORDER BY ym, MIN(p.start_time)`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := map[string][]BookTime{}
	for rows.Next() {
		var ym string
		var bt BookTime
		if err := rows.Scan(&ym, &bt.Title, &bt.Authors, &bt.Seconds); err != nil {
			return out
		}
		out[ym] = append(out[ym], bt)
	}
	return out
}

// dayBooks returns, per local day (since cutoff), the books read that day and
// their reading time, most-read first. Best-effort on old schemas.
func dayBooks(db *sql.DB, cutoff int64) map[string][]BookTime {
	rows, err := db.Query(`
		SELECT strftime('%Y-%m-%d', p.start_time, 'unixepoch', 'localtime') AS d,
		       COALESCE(b.title, ''), COALESCE(b.authors, ''), COALESCE(SUM(p.duration), 0) AS secs
		FROM page_stat_data p JOIN book b ON b.id = p.id_book
		WHERE p.start_time >= ?
		GROUP BY d, p.id_book
		HAVING secs > 0
		ORDER BY d, secs DESC`, cutoff)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := map[string][]BookTime{}
	for rows.Next() {
		var day string
		var bt BookTime
		if err := rows.Scan(&day, &bt.Title, &bt.Authors, &bt.Seconds); err != nil {
			return out
		}
		out[day] = append(out[day], bt)
	}
	return out
}

// topBooks returns the most-read books by time (best-effort: an old schema
// without a joinable `book` table yields none).
func topBooks(db *sql.DB) []BookTime {
	rows, err := db.Query(`
		SELECT COALESCE(b.title, ''), COALESCE(b.authors, ''),
		       COALESCE(SUM(p.duration), 0) AS secs,
		       strftime('%Y-%m-%d', MAX(p.start_time), 'unixepoch', 'localtime') AS last
		FROM page_stat_data p JOIN book b ON b.id = p.id_book
		GROUP BY p.id_book
		HAVING secs > 0
		ORDER BY secs DESC
		LIMIT 12`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []BookTime
	for rows.Next() {
		var bt BookTime
		if err := rows.Scan(&bt.Title, &bt.Authors, &bt.Seconds, &bt.LastRead); err != nil {
			return out
		}
		out = append(out, bt)
	}
	return out
}

// eachRow runs a two-column (int, int64) aggregate query and calls fn per row.
func eachRow(db *sql.DB, query string, fn func(key int, secs int64)) error {
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key int
		var secs int64
		if err := rows.Scan(&key, &secs); err != nil {
			return err
		}
		fn(key, secs)
	}
	return rows.Err()
}

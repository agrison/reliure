package core

// This file holds read-only aggregate queries backing the dashboard. They stay
// deliberately simple (one GROUP BY each) so the whole dashboard loads in a
// handful of fast queries.

// FileStats returns the total byte size of every file and the file count.
func (r *BookRepo) FileStats() (totalSize int64, files int, err error) {
	err = r.db.QueryRow(`SELECT COALESCE(SUM(size), 0), COUNT(*) FROM file`).Scan(&totalSize, &files)
	return totalSize, files, err
}

// FormatCounts returns the number of files per format ("epub", "pdf"…), most
// common first.
func (r *BookRepo) FormatCounts() ([]NamedCount, error) {
	rows, err := r.db.Query(`
		SELECT 0 AS id, format AS name, COUNT(*) AS n
		FROM file WHERE format != '' GROUP BY format ORDER BY n DESC, format`)
	if err != nil {
		return nil, err
	}
	return scanCounts(rows)
}

// LanguageCounts returns book counts per (non-empty) language, most common first.
func (r *BookRepo) LanguageCounts() ([]NamedCount, error) {
	rows, err := r.db.Query(`
		SELECT 0 AS id, language AS name, COUNT(*) AS n
		FROM book WHERE language != '' GROUP BY language ORDER BY n DESC, language`)
	if err != nil {
		return nil, err
	}
	return scanCounts(rows)
}

// AddedByMonth returns the number of books added per calendar month
// ("YYYY-MM"), oldest first. added_at is RFC3339, which strftime parses.
func (r *BookRepo) AddedByMonth() ([]NamedCount, error) {
	rows, err := r.db.Query(`
		SELECT 0 AS id, strftime('%Y-%m', added_at) AS name, COUNT(*) AS n
		FROM book WHERE added_at != '' GROUP BY name ORDER BY name`)
	if err != nil {
		return nil, err
	}
	return scanCounts(rows)
}

// TrackedCount returns how many books have a reading state recorded.
func (r *ReadingRepo) TrackedCount() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM reading_state`).Scan(&n)
	return n, err
}

// AnnotationTotal returns the total number of annotations across all books.
func (r *ReadingRepo) AnnotationTotal() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM annotation`).Scan(&n)
	return n, err
}

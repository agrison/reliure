package core

// NamedCount is a named entity (author, series or tag) with how many books
// reference it. Used to populate the sidebar.
type NamedCount struct {
	ID    int64
	Name  string
	Count int
}

func scanCounts(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}) ([]NamedCount, error) {
	defer rows.Close()
	var out []NamedCount
	for rows.Next() {
		var nc NamedCount
		if err := rows.Scan(&nc.ID, &nc.Name, &nc.Count); err != nil {
			return nil, err
		}
		out = append(out, nc)
	}
	return out, rows.Err()
}

// Counts returns every author with their book count, ordered by sort name.
// Authors with zero books are omitted (they can linger after deletions).
func (r *AuthorRepo) Counts() ([]NamedCount, error) {
	rows, err := r.db.Query(`
		SELECT a.id, a.name, COUNT(ba.book_id) AS n
		FROM author a JOIN book_author ba ON ba.author_id = a.id
		GROUP BY a.id
		ORDER BY a.sort_name, a.name`)
	if err != nil {
		return nil, err
	}
	return scanCounts(rows)
}

// CountMissing returns the number of books without any author link.
func (r *AuthorRepo) CountMissing() (int, error) {
	var n int
	err := r.db.QueryRow(`
		SELECT COUNT(*)
		FROM book b
		WHERE NOT EXISTS (SELECT 1 FROM book_author ba WHERE ba.book_id = b.id)`).Scan(&n)
	return n, err
}

// Counts returns every series with their book count, ordered by sort name.
func (r *SeriesRepo) Counts() ([]NamedCount, error) {
	rows, err := r.db.Query(`
		SELECT s.id, s.name, COUNT(b.id) AS n
		FROM series s JOIN book b ON b.series_id = s.id
		GROUP BY s.id
		ORDER BY s.sort_name, s.name`)
	if err != nil {
		return nil, err
	}
	return scanCounts(rows)
}

// CountMissing returns the number of books not attached to a series.
func (r *SeriesRepo) CountMissing() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM book WHERE series_id IS NULL`).Scan(&n)
	return n, err
}

// Counts returns every tag with their book count, ordered by name.
func (r *TagRepo) Counts() ([]NamedCount, error) {
	rows, err := r.db.Query(`
		SELECT t.id, t.name, COUNT(bt.book_id) AS n
		FROM tag t JOIN book_tag bt ON bt.tag_id = t.id
		GROUP BY t.id
		ORDER BY t.name`)
	if err != nil {
		return nil, err
	}
	return scanCounts(rows)
}

// CountMissing returns the number of books without any tag.
func (r *TagRepo) CountMissing() (int, error) {
	var n int
	err := r.db.QueryRow(`
		SELECT COUNT(*)
		FROM book b
		WHERE NOT EXISTS (SELECT 1 FROM book_tag bt WHERE bt.book_id = b.id)`).Scan(&n)
	return n, err
}

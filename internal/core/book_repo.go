package core

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// BookRepo manages books and their relations (authors, series, tags, files).
type BookRepo struct{ db *sql.DB }

// now is indirected to ease testing; UTC for stability.
var now = func() time.Time { return time.Now().UTC() }

const rfc3339 = time.RFC3339Nano

// Create inserts a new book and all its relations in one transaction.
// Authors/series/tags are resolved by name (get-or-create). On any failure the
// whole thing rolls back. b.ID and the relation ids are filled in on return.
func (r *BookRepo) Create(b *Book) error {
	if strings.TrimSpace(b.Title) == "" {
		return errors.New("empty title")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ts := now()
	b.AddedAt, b.UpdatedAt = ts, ts

	seriesID, err := resolveSeries(tx, b)
	if err != nil {
		return err
	}
	res, err := tx.Exec(
		`INSERT INTO book
		   (title, title_sort, description, language, isbn, published_at,
		    series_id, series_index, added_at, updated_at, cover_path)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		b.Title, b.TitleSort, b.Description, b.Language, b.ISBN, b.PublishedAt,
		seriesID, floatPtr(b.SeriesIndex), ts.Format(rfc3339), ts.Format(rfc3339), b.CoverPath)
	if err != nil {
		return err
	}
	b.ID, err = res.LastInsertId()
	if err != nil {
		return err
	}

	if err := writeAuthors(tx, b); err != nil {
		return err
	}
	if err := writeTags(tx, b); err != nil {
		return err
	}
	if err := writeFiles(tx, b); err != nil {
		return err
	}
	if err := syncFTS(tx, b.ID); err != nil {
		return err
	}
	return tx.Commit()
}

// Update rewrites the book's metadata, series, authors and tags. Files are
// managed separately (AddFile). updated_at is refreshed.
func (r *BookRepo) Update(b *Book) error {
	if b.ID == 0 {
		return errors.New("book without id")
	}
	if strings.TrimSpace(b.Title) == "" {
		return errors.New("empty title")
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ts := now()
	b.UpdatedAt = ts
	seriesID, err := resolveSeries(tx, b)
	if err != nil {
		return err
	}
	res, err := tx.Exec(
		`UPDATE book SET
		   title=?, title_sort=?, description=?, language=?, isbn=?, published_at=?,
		   series_id=?, series_index=?, updated_at=?, cover_path=?
		 WHERE id=?`,
		b.Title, b.TitleSort, b.Description, b.Language, b.ISBN, b.PublishedAt,
		seriesID, floatPtr(b.SeriesIndex), ts.Format(rfc3339), b.CoverPath, b.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}

	// Fully replace the author/tag links.
	if _, err := tx.Exec(`DELETE FROM book_author WHERE book_id=?`, b.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM book_tag WHERE book_id=?`, b.ID); err != nil {
		return err
	}
	if err := writeAuthors(tx, b); err != nil {
		return err
	}
	if err := writeTags(tx, b); err != nil {
		return err
	}
	if err := syncFTS(tx, b.ID); err != nil {
		return err
	}
	return tx.Commit()
}

// Delete removes a book; its links and files go away via ON DELETE CASCADE.
func (r *BookRepo) Delete(id int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM book WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.Exec(`DELETE FROM book_fts WHERE rowid=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// AddFile attaches a file to an existing book.
func (r *BookRepo) AddFile(bookID int64, f File) (File, error) {
	f.BookID = bookID
	if f.AddedAt.IsZero() {
		f.AddedAt = now()
	}
	res, err := r.db.Exec(
		`INSERT INTO file (book_id, path, format, size, sha256, added_at)
		 VALUES (?,?,?,?,?,?)`,
		f.BookID, f.Path, f.Format, f.Size, f.SHA256, f.AddedAt.UTC().Format(rfc3339))
	if err != nil {
		return File{}, err
	}
	f.ID, err = res.LastInsertId()
	return f, err
}

// FindByFileSHA returns the id of the book owning a file with the given
// SHA-256, if any. Used for exact-duplicate detection on import.
func (r *BookRepo) FindByFileSHA(sha string) (int64, bool, error) {
	if strings.TrimSpace(sha) == "" {
		return 0, false, nil
	}
	var id int64
	err := r.db.QueryRow(`SELECT book_id FROM file WHERE sha256 = ? LIMIT 1`, sha).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// FindByTitleAuthor returns the id of a book matching the given title and
// primary author (case-insensitive). This is the heuristic half of import
// deduplication: it lets a new file be attached to an existing book (e.g. a
// second format of the same work) instead of creating a duplicate. An empty
// author matches a book that has no authors.
func (r *BookRepo) FindByTitleAuthor(title, author string) (int64, bool, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return 0, false, nil
	}
	var row *sql.Row
	if strings.TrimSpace(author) == "" {
		row = r.db.QueryRow(
			`SELECT b.id FROM book b
			 WHERE b.title = ? COLLATE NOCASE
			   AND NOT EXISTS (SELECT 1 FROM book_author ba WHERE ba.book_id = b.id)
			 LIMIT 1`, title)
	} else {
		row = r.db.QueryRow(
			`SELECT b.id FROM book b
			 JOIN book_author ba ON ba.book_id = b.id
			 JOIN author a ON a.id = ba.author_id
			 WHERE b.title = ? COLLATE NOCASE AND a.name = ? COLLATE NOCASE
			 LIMIT 1`, title, author)
	}
	var id int64
	err := row.Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// SetCover updates just a book's cached cover path.
func (r *BookRepo) SetCover(bookID int64, coverPath string) error {
	res, err := r.db.Exec(`UPDATE book SET cover_path = ?, updated_at = ? WHERE id = ?`,
		coverPath, now().Format(rfc3339), bookID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ByID loads a full book (series, authors, tags, files).
func (r *BookRepo) ByID(id int64) (*Book, error) {
	books, err := r.scanBooks(`SELECT `+bookCols+` FROM book b WHERE b.id=?`, id)
	if err != nil {
		return nil, err
	}
	if len(books) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := r.hydrate(books); err != nil {
		return nil, err
	}
	return books[0], nil
}

// List returns books sorted by title (sort key), paginated.
// limit <= 0 → no limit.
func (r *BookRepo) List(limit, offset int) ([]*Book, error) {
	q := `SELECT ` + bookCols + ` FROM book b
	      ORDER BY (CASE WHEN b.title_sort='' THEN b.title ELSE b.title_sort END) COLLATE NOCASE, b.id`
	q += limitClause(limit, offset)
	books, err := r.scanBooks(q)
	if err != nil {
		return nil, err
	}
	return books, r.hydrate(books)
}

// Count returns the total number of books.
func (r *BookRepo) Count() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM book`).Scan(&n)
	return n, err
}

// Browse returns books in the requested order, paginated. sort is one of
// "title" (default), "author" or "added" (most recent first).
func (r *BookRepo) Browse(sort string, limit, offset int) ([]*Book, error) {
	q := `SELECT ` + bookCols + ` FROM book b ` + orderClause(sort) + limitClause(limit, offset)
	books, err := r.scanBooks(q)
	if err != nil {
		return nil, err
	}
	return books, r.hydrate(books)
}

// ListByTag returns the books carrying a tag, ordered by title.
func (r *BookRepo) ListByTag(tagID int64) ([]*Book, error) {
	q := `SELECT ` + bookCols + ` FROM book b
	      JOIN book_tag bt ON bt.book_id = b.id
	      WHERE bt.tag_id = ? ` + orderClause("title")
	books, err := r.scanBooks(q, tagID)
	if err != nil {
		return nil, err
	}
	return books, r.hydrate(books)
}

// orderClause maps a sort key to a safe ORDER BY clause (never interpolates
// user input — only fixed strings).
func orderClause(sort string) string {
	titleKey := `(CASE WHEN b.title_sort='' THEN b.title ELSE b.title_sort END) COLLATE NOCASE`
	switch sort {
	case "added":
		return `ORDER BY b.added_at DESC, b.id DESC`
	case "author":
		return `ORDER BY (SELECT a.sort_name FROM book_author ba
		                  JOIN author a ON a.id = ba.author_id
		                  WHERE ba.book_id = b.id ORDER BY ba.position LIMIT 1) COLLATE NOCASE, ` + titleKey
	default:
		return `ORDER BY ` + titleKey + `, b.id`
	}
}

// ListByAuthor returns an author's books, ordered by series then title.
func (r *BookRepo) ListByAuthor(authorID int64) ([]*Book, error) {
	q := `SELECT ` + bookCols + ` FROM book b
	      JOIN book_author ba ON ba.book_id = b.id
	      WHERE ba.author_id = ?
	      ORDER BY b.series_id IS NULL, b.series_index,
	               (CASE WHEN b.title_sort='' THEN b.title ELSE b.title_sort END) COLLATE NOCASE`
	books, err := r.scanBooks(q, authorID)
	if err != nil {
		return nil, err
	}
	return books, r.hydrate(books)
}

// ListBySeries returns a series' books, ordered by index then title.
func (r *BookRepo) ListBySeries(seriesID int64) ([]*Book, error) {
	q := `SELECT ` + bookCols + ` FROM book b
	      WHERE b.series_id = ?
	      ORDER BY b.series_index,
	               (CASE WHEN b.title_sort='' THEN b.title ELSE b.title_sort END) COLLATE NOCASE`
	books, err := r.scanBooks(q, seriesID)
	if err != nil {
		return nil, err
	}
	return books, r.hydrate(books)
}

// --- internals ---

const bookCols = `b.id, b.title, b.title_sort, b.description, b.language, b.isbn,
	b.published_at, b.series_id, b.series_index, b.added_at, b.updated_at, b.cover_path`

func (r *BookRepo) scanBooks(query string, args ...any) ([]*Book, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Book
	for rows.Next() {
		var (
			b                  Book
			seriesID           sql.NullInt64
			seriesIdx          sql.NullFloat64
			addedAt, updatedAt string
		)
		if err := rows.Scan(&b.ID, &b.Title, &b.TitleSort, &b.Description, &b.Language,
			&b.ISBN, &b.PublishedAt, &seriesID, &seriesIdx, &addedAt, &updatedAt, &b.CoverPath); err != nil {
			return nil, err
		}
		b.AddedAt, _ = time.Parse(rfc3339, addedAt)
		b.UpdatedAt, _ = time.Parse(rfc3339, updatedAt)
		if seriesIdx.Valid {
			v := seriesIdx.Float64
			b.SeriesIndex = &v
		}
		if seriesID.Valid {
			b.Series = &Series{ID: seriesID.Int64} // name filled in by hydrate
		}
		out = append(out, &b)
	}
	return out, rows.Err()
}

// hydrate batch-loads the series, authors, tags and files for a set of books
// (avoids the N+1 problem).
func (r *BookRepo) hydrate(books []*Book) error {
	if len(books) == 0 {
		return nil
	}
	ids := make([]int64, len(books))
	byID := make(map[int64]*Book, len(books))
	for i, b := range books {
		ids[i] = b.ID
		byID[b.ID] = b
	}
	if err := r.loadSeries(books); err != nil {
		return err
	}
	if err := r.loadAuthors(ids, byID); err != nil {
		return err
	}
	if err := r.loadTags(ids, byID); err != nil {
		return err
	}
	return r.loadFiles(ids, byID)
}

func (r *BookRepo) loadSeries(books []*Book) error {
	for _, b := range books {
		if b.Series == nil {
			continue
		}
		s, err := (&SeriesRepo{db: r.db}).ByID(b.Series.ID)
		if err != nil {
			return err
		}
		b.Series = &s
	}
	return nil
}

func (r *BookRepo) loadAuthors(ids []int64, byID map[int64]*Book) error {
	in, args := inClause(ids)
	rows, err := r.db.Query(`
		SELECT ba.book_id, a.id, a.name, a.sort_name, ba.role, ba.position
		FROM book_author ba JOIN author a ON a.id = ba.author_id
		WHERE ba.book_id IN `+in+`
		ORDER BY ba.book_id, ba.position, a.sort_name`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var bookID int64
		var c Contribution
		if err := rows.Scan(&bookID, &c.Author.ID, &c.Author.Name, &c.Author.SortName, &c.Role, &c.Position); err != nil {
			return err
		}
		if b := byID[bookID]; b != nil {
			b.Authors = append(b.Authors, c)
		}
	}
	return rows.Err()
}

func (r *BookRepo) loadTags(ids []int64, byID map[int64]*Book) error {
	in, args := inClause(ids)
	rows, err := r.db.Query(`
		SELECT bt.book_id, t.id, t.name
		FROM book_tag bt JOIN tag t ON t.id = bt.tag_id
		WHERE bt.book_id IN `+in+`
		ORDER BY bt.book_id, t.name`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var bookID int64
		var t Tag
		if err := rows.Scan(&bookID, &t.ID, &t.Name); err != nil {
			return err
		}
		if b := byID[bookID]; b != nil {
			b.Tags = append(b.Tags, t)
		}
	}
	return rows.Err()
}

func (r *BookRepo) loadFiles(ids []int64, byID map[int64]*Book) error {
	in, args := inClause(ids)
	rows, err := r.db.Query(`
		SELECT id, book_id, path, format, size, sha256, added_at
		FROM file WHERE book_id IN `+in+` ORDER BY book_id, id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var f File
		var addedAt string
		if err := rows.Scan(&f.ID, &f.BookID, &f.Path, &f.Format, &f.Size, &f.SHA256, &addedAt); err != nil {
			return err
		}
		f.AddedAt, _ = time.Parse(rfc3339, addedAt)
		if b := byID[f.BookID]; b != nil {
			b.Files = append(b.Files, f)
		}
	}
	return rows.Err()
}

// resolveSeries get-or-creates the book's series and returns its id, or NULL if
// the book has no series.
func resolveSeries(tx querier, b *Book) (any, error) {
	if b.Series == nil || strings.TrimSpace(b.Series.Name) == "" {
		return nil, nil
	}
	s, err := getOrCreateSeries(tx, b.Series.Name)
	if err != nil {
		return nil, err
	}
	b.Series = &s
	return s.ID, nil
}

func writeAuthors(tx querier, b *Book) error {
	for i := range b.Authors {
		c := &b.Authors[i]
		a, err := getOrCreateAuthor(tx, c.Author.Name)
		if err != nil {
			return err
		}
		c.Author = a
		role := c.Role
		if role == "" {
			role = "aut"
			c.Role = role
		}
		if _, err := tx.Exec(
			`INSERT INTO book_author (book_id, author_id, role, position)
			 VALUES (?,?,?,?)
			 ON CONFLICT(book_id, author_id, role) DO UPDATE SET position=excluded.position`,
			b.ID, a.ID, role, c.Position); err != nil {
			return err
		}
	}
	return nil
}

func writeTags(tx querier, b *Book) error {
	for i := range b.Tags {
		t, err := getOrCreateTag(tx, b.Tags[i].Name)
		if err != nil {
			return err
		}
		b.Tags[i] = t
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO book_tag (book_id, tag_id) VALUES (?,?)`, b.ID, t.ID); err != nil {
			return err
		}
	}
	return nil
}

func writeFiles(tx querier, b *Book) error {
	for i := range b.Files {
		f := &b.Files[i]
		f.BookID = b.ID
		if f.AddedAt.IsZero() {
			f.AddedAt = now()
		}
		res, err := tx.Exec(
			`INSERT INTO file (book_id, path, format, size, sha256, added_at)
			 VALUES (?,?,?,?,?,?)`,
			f.BookID, f.Path, f.Format, f.Size, f.SHA256, f.AddedAt.UTC().Format(rfc3339))
		if err != nil {
			return err
		}
		f.ID, err = res.LastInsertId()
		if err != nil {
			return err
		}
	}
	return nil
}

func floatPtr(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func limitClause(limit, offset int) string {
	if limit <= 0 {
		if offset > 0 {
			return fmt.Sprintf(" LIMIT -1 OFFSET %d", offset)
		}
		return ""
	}
	return fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
}

// inClause builds a "(?,?,…)" placeholder list and the matching arguments.
func inClause(ids []int64) (string, []any) {
	if len(ids) == 0 {
		return "(NULL)", nil
	}
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	return "(" + strings.Join(ph, ",") + ")", args
}

package core

import (
	"strings"
)

// syncFTS rebuilds a book's full-text index entry from its current DB state
// (denormalized title + authors + series + tags). Called inside the write
// transaction, after the relations have been updated.
func syncFTS(tx querier, bookID int64) error {
	var title, series, authors, tags string
	err := tx.QueryRow(`
		SELECT b.title,
		       COALESCE(s.name, ''),
		       COALESCE((SELECT group_concat(a.name, ' ')
		                 FROM book_author ba JOIN author a ON a.id = ba.author_id
		                 WHERE ba.book_id = b.id), ''),
		       COALESCE((SELECT group_concat(t.name, ' ')
		                 FROM book_tag bt JOIN tag t ON t.id = bt.tag_id
		                 WHERE bt.book_id = b.id), '')
		FROM book b LEFT JOIN series s ON s.id = b.series_id
		WHERE b.id = ?`, bookID).Scan(&title, &series, &authors, &tags)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM book_fts WHERE rowid = ?`, bookID); err != nil {
		return err
	}
	_, err = tx.Exec(
		`INSERT INTO book_fts (rowid, title, authors, series, tags) VALUES (?,?,?,?,?)`,
		bookID, title, authors, series, tags)
	return err
}

// Search runs a full-text query and returns the matching full books, ranked by
// relevance (bm25). limit <= 0 → 50 by default.
func (r *BookRepo) Search(query string, limit int) ([]*Book, error) {
	return r.SearchScoped(query, SearchScope{}, limit)
}

func (r *BookRepo) SearchScoped(query string, scope SearchScope, limit int) ([]*Book, error) {
	match := ftsQuery(query)
	if match == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	filter, args := bookScopeFilter(scope)
	args = append([]any{match}, args...)
	q := `SELECT ` + bookCols + `
	      FROM book_fts f JOIN book b ON b.id = f.rowid
	      WHERE book_fts MATCH ?` + filter + `
	      ORDER BY rank` + limitClause(limit, 0)
	books, err := r.scanBooks(q, args...)
	if err != nil {
		return nil, err
	}
	return books, r.hydrate(books)
}

// ftsQuery turns free-form user input into a safe FTS5 MATCH expression: each
// term is quoted (neutralizing special characters) and suffixed with '*' for
// prefix matching. Terms combine with implicit AND. Returns "" when the input
// contains no term.
func ftsQuery(raw string) string {
	fields := strings.Fields(raw)
	terms := make([]string, 0, len(fields))
	for _, f := range fields {
		// Drop inner quotes, then requote cleanly.
		f = strings.ReplaceAll(f, `"`, "")
		if f == "" {
			continue
		}
		terms = append(terms, `"`+f+`"*`)
	}
	return strings.Join(terms, " ")
}

func bookScopeFilter(scope SearchScope) (string, []any) {
	switch scope.Kind {
	case "author":
		if scope.ID == 0 {
			return ` AND NOT EXISTS (SELECT 1 FROM book_author ba WHERE ba.book_id = b.id)`, nil
		}
		return ` AND EXISTS (SELECT 1 FROM book_author ba WHERE ba.book_id = b.id AND ba.author_id = ?)`, []any{scope.ID}
	case "series":
		if scope.ID == 0 {
			return ` AND b.series_id IS NULL`, nil
		}
		return ` AND b.series_id = ?`, []any{scope.ID}
	case "tag":
		if scope.ID == 0 {
			return ` AND NOT EXISTS (SELECT 1 FROM book_tag bt WHERE bt.book_id = b.id)`, nil
		}
		return ` AND EXISTS (SELECT 1 FROM book_tag bt WHERE bt.book_id = b.id AND bt.tag_id = ?)`, []any{scope.ID}
	case "book":
		return ` AND b.id = ?`, []any{scope.ID}
	default:
		return "", nil
	}
}

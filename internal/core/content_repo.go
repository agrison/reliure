package core

import (
	"database/sql"
	"sort"
	"strings"
	"unicode"
)

// ContentRepo stores and searches extracted ebook text.
type ContentRepo struct{ db *sql.DB }

type ContentStats struct {
	IndexedBooks int   `json:"indexedBooks"`
	EmptyBooks   int   `json:"emptyBooks"`
	FailedBooks  int   `json:"failedBooks"`
	IndexedChars int64 `json:"indexedChars"`
}

type SearchScope struct {
	Kind string `json:"kind"`
	ID   int64  `json:"id"`
}

type ContentFragment struct {
	Page int
	Text string
}

type ContentHit struct {
	BookID      int64
	Title       string
	Authors     string
	Series      string
	SeriesIndex float64
	Cover       string
	Page        int
	Ordinal     int
	Snippet     string
	More        int
}

type ContentHitPage struct {
	Total   int
	Page    int
	PerPage int
	Hits    []ContentHit
}

func (r *ContentRepo) Upsert(bookID, fileID int64, fragments []ContentFragment) error {
	fragments = cleanFragments(fragments)
	status := "indexed"
	if len(fragments) == 0 {
		status = "empty"
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM content_fts WHERE book_id=?`, bookID); err != nil {
		return err
	}
	chars := 0
	for i, f := range fragments {
		chars += len([]rune(f.Text))
		if _, err := tx.Exec(`INSERT INTO content_fts (book_id, ordinal, page, body) VALUES (?,?,?,?)`,
			bookID, i+1, f.Page, f.Text); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`
		INSERT INTO content_index (book_id, file_id, status, chars, error, indexed_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(book_id) DO UPDATE SET
			file_id=excluded.file_id,
			status=excluded.status,
			chars=excluded.chars,
			error='',
			indexed_at=excluded.indexed_at`,
		bookID, nullInt(fileID), status, chars, "", now().Format(rfc3339)); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *ContentRepo) MarkFailed(bookID, fileID int64, msg string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM content_fts WHERE book_id=?`, bookID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO content_index (book_id, file_id, status, chars, error, indexed_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(book_id) DO UPDATE SET
			file_id=excluded.file_id,
			status=excluded.status,
			chars=0,
			error=excluded.error,
			indexed_at=excluded.indexed_at`,
		bookID, nullInt(fileID), "failed", 0, truncate(msg, 500), now().Format(rfc3339)); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *ContentRepo) Delete(bookID int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM content_fts WHERE book_id=?`, bookID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM content_index WHERE book_id=?`, bookID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *ContentRepo) Stats() (ContentStats, error) {
	var st ContentStats
	rows, err := r.db.Query(`SELECT status, COUNT(*), COALESCE(SUM(chars), 0) FROM content_index GROUP BY status`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		var chars int64
		if err := rows.Scan(&status, &count, &chars); err != nil {
			return st, err
		}
		switch status {
		case "indexed":
			st.IndexedBooks = count
			st.IndexedChars = chars
		case "empty":
			st.EmptyBooks = count
		case "failed":
			st.FailedBooks = count
		}
	}
	return st, rows.Err()
}

func (r *ContentRepo) Search(query string, scope SearchScope, limit int) ([]*Book, error) {
	match := ftsQuery(query)
	if match == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 200
	}
	filter, args := bookScopeFilter(scope)
	args = append([]any{match}, args...)
	q := `SELECT ` + bookCols + `
	      FROM (
	          SELECT DISTINCT CAST(book_id AS INTEGER) AS book_id
	          FROM content_fts
	          WHERE content_fts MATCH ?
	      ) m
	      JOIN book b ON b.id = m.book_id
	      LEFT JOIN series s ON s.id = b.series_id
	      WHERE 1=1` + filter + `
	      ORDER BY COALESCE(NULLIF(s.sort_name, ''), s.name, (CASE WHEN b.title_sort='' THEN b.title ELSE b.title_sort END)) COLLATE NOCASE,
	               b.series_index IS NULL,
	               b.series_index,
	               (CASE WHEN b.title_sort='' THEN b.title ELSE b.title_sort END) COLLATE NOCASE,
	               b.id` + limitClause(limit, 0)
	books, err := r.scanBooks(q, args...)
	if err != nil {
		return nil, err
	}
	return books, r.hydrate(books)
}

func (r *ContentRepo) Snippets(query string, scope SearchScope, booksLimit, hitsPerBook int, contextMode string) ([]ContentHit, error) {
	match := ftsQuery(query)
	if match == "" {
		return nil, nil
	}
	terms := contentTerms(query)
	if len(terms) == 0 {
		return nil, nil
	}
	if booksLimit <= 0 {
		booksLimit = 20
	}
	if hitsPerBook <= 0 {
		hitsPerBook = 3
	}
	books, err := r.Search(query, scope, booksLimit)
	if err != nil {
		return nil, err
	}
	var out []ContentHit
	for _, b := range books {
		total, err := r.countOccurrences(match, b.ID, terms)
		if err != nil {
			return nil, err
		}
		rows, err := r.db.Query(`
			SELECT CAST(page AS INTEGER),
			       CAST(ordinal AS INTEGER),
			       body
			FROM content_fts
			WHERE content_fts MATCH ? AND CAST(book_id AS INTEGER) = ?
			ORDER BY CAST(page AS INTEGER), CAST(ordinal AS INTEGER)`, match, b.ID)
		if err != nil {
			return nil, err
		}
		n := 0
		done := false
		for rows.Next() {
			var page, ordinal int
			var body string
			if err := rows.Scan(&page, &ordinal, &body); err != nil {
				rows.Close()
				return nil, err
			}
			for _, snippet := range occurrenceSnippets(body, terms, contextMode) {
				n++
				h := ContentHit{
					BookID:      b.ID,
					Title:       b.Title,
					Authors:     contentAuthors(b),
					Series:      contentSeries(b),
					SeriesIndex: contentSeriesIndex(b),
					Cover:       b.CoverPath,
					Page:        page,
					Ordinal:     ordinal,
					Snippet:     snippet,
				}
				if total > n {
					h.More = total - n
				}
				out = append(out, h)
				if n >= hitsPerBook {
					done = true
					break
				}
			}
			if done {
				break
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}

func (r *ContentRepo) Occurrences(query string, scope SearchScope, page, perPage int, contextMode string) (ContentHitPage, error) {
	match := ftsQuery(query)
	if match == "" {
		return ContentHitPage{Page: 1, PerPage: perPage}, nil
	}
	terms := contentTerms(query)
	if len(terms) == 0 {
		return ContentHitPage{Page: 1, PerPage: perPage}, nil
	}
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	filter, scopeArgs := bookScopeFilter(scope)
	offset := (page - 1) * perPage
	args := append([]any{match}, scopeArgs...)
	rows, err := r.db.Query(`
		SELECT b.id,
		       b.title,
		       COALESCE((SELECT group_concat(a.name, ', ')
		                 FROM book_author ba JOIN author a ON a.id = ba.author_id
		                 WHERE ba.book_id = b.id
		                 ORDER BY ba.position), ''),
		       COALESCE(s.name, ''),
		       COALESCE(b.series_index, 0),
		       b.cover_path,
		       CAST(cf.page AS INTEGER),
		       CAST(cf.ordinal AS INTEGER),
		       cf.body
		FROM content_fts cf
		JOIN book b ON b.id = CAST(cf.book_id AS INTEGER)
		LEFT JOIN series s ON s.id = b.series_id
		WHERE content_fts MATCH ?`+filter+`
		ORDER BY COALESCE(NULLIF(s.sort_name, ''), s.name, (CASE WHEN b.title_sort='' THEN b.title ELSE b.title_sort END)) COLLATE NOCASE,
		         b.series_index IS NULL,
		         b.series_index,
		         (CASE WHEN b.title_sort='' THEN b.title ELSE b.title_sort END) COLLATE NOCASE,
		         b.id,
		         CAST(cf.page AS INTEGER),
		         CAST(cf.ordinal AS INTEGER)`, args...)
	if err != nil {
		return ContentHitPage{}, err
	}
	defer rows.Close()
	var hits []ContentHit
	total := 0
	for rows.Next() {
		var base ContentHit
		var body string
		if err := rows.Scan(&base.BookID, &base.Title, &base.Authors, &base.Series, &base.SeriesIndex, &base.Cover, &base.Page, &base.Ordinal, &body); err != nil {
			return ContentHitPage{}, err
		}
		for _, snippet := range occurrenceSnippets(body, terms, contextMode) {
			total++
			if total <= offset || len(hits) >= perPage {
				continue
			}
			h := base
			h.Snippet = snippet
			hits = append(hits, h)
		}
	}
	return ContentHitPage{Total: total, Page: page, PerPage: perPage, Hits: hits}, rows.Err()
}

func (r *ContentRepo) countOccurrences(match string, bookID int64, terms []string) (int, error) {
	rows, err := r.db.Query(`
		SELECT body
		FROM content_fts
		WHERE content_fts MATCH ? AND CAST(book_id AS INTEGER) = ?`, match, bookID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	total := 0
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			return 0, err
		}
		total += len(occurrencePositions(body, terms))
	}
	return total, rows.Err()
}

func contentAuthors(b *Book) string {
	var names []string
	for _, c := range b.Authors {
		if c.Author.Name != "" {
			names = append(names, c.Author.Name)
		}
	}
	return strings.Join(names, ", ")
}

func contentSeries(b *Book) string {
	if b.Series == nil {
		return ""
	}
	return b.Series.Name
}

func contentSeriesIndex(b *Book) float64 {
	if b.SeriesIndex == nil {
		return 0
	}
	return *b.SeriesIndex
}

type occurrencePos struct {
	Start int
	End   int
}

func contentTerms(query string) []string {
	fields := strings.Fields(query)
	out := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, f := range fields {
		f = strings.Trim(f, `"'“”‘’.,;:!?()[]{}<>`)
		if f == "" {
			continue
		}
		f = strings.ToLower(f)
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

func occurrencePositions(text string, terms []string) []occurrencePos {
	lower := strings.ToLower(text)
	var out []occurrencePos
	for _, term := range terms {
		if term == "" {
			continue
		}
		start := 0
		for {
			i := strings.Index(lower[start:], term)
			if i < 0 {
				break
			}
			pos := start + i
			end := pos + len(term)
			if termBoundary(lower, pos, end) {
				out = append(out, occurrencePos{Start: pos, End: end})
			}
			start = end
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Start == out[j].Start {
			return out[i].End > out[j].End
		}
		return out[i].Start < out[j].Start
	})
	return dedupeOccurrences(out)
}

func dedupeOccurrences(in []occurrencePos) []occurrencePos {
	out := make([]occurrencePos, 0, len(in))
	lastEnd := -1
	for _, p := range in {
		if p.Start < lastEnd {
			continue
		}
		out = append(out, p)
		lastEnd = p.End
	}
	return out
}

func termBoundary(s string, start, end int) bool {
	beforeOK := start == 0 || !isTermRune(runeBefore(s, start))
	afterOK := end >= len(s) || !isTermRune(runeAfter(s, end))
	return beforeOK && afterOK
}

func runeBefore(s string, pos int) rune {
	for i, r := range s {
		if i >= pos {
			break
		}
		if i+len(string(r)) == pos {
			return r
		}
	}
	return 0
}

func runeAfter(s string, pos int) rune {
	for i, r := range s {
		if i == pos {
			return r
		}
		if i > pos {
			return r
		}
	}
	return 0
}

func isTermRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

func occurrenceSnippets(text string, terms []string, mode string) []string {
	positions := occurrencePositions(text, terms)
	out := make([]string, 0, len(positions))
	for _, p := range positions {
		start, end := contextBounds(text, p, mode)
		snippet := strings.TrimSpace(text[start:end])
		rel := occurrencePos{Start: p.Start - start, End: p.End - start}
		out = append(out, highlightSnippet(snippet, rel, terms))
	}
	return out
}

func contextBounds(text string, p occurrencePos, mode string) (int, int) {
	switch mode {
	case "phrase":
		return sentenceBounds(text, p)
	case "paragraph":
		return windowBounds(text, p, 520)
	default:
		return wordWindowBounds(text, p, 9)
	}
}

func sentenceBounds(text string, p occurrencePos) (int, int) {
	start := strings.LastIndexAny(text[:p.Start], ".!?;:\n")
	if start < 0 {
		start = 0
	} else {
		start++
	}
	endRel := strings.IndexAny(text[p.End:], ".!?;:\n")
	end := len(text)
	if endRel >= 0 {
		end = p.End + endRel + 1
	}
	if end-start > 520 {
		return windowBounds(text, p, 300)
	}
	return start, end
}

func wordWindowBounds(text string, p occurrencePos, words int) (int, int) {
	start := p.Start
	for i := 0; i < words && start > 0; i++ {
		start = previousWordStart(text, start)
	}
	end := p.End
	for i := 0; i < words && end < len(text); i++ {
		end = nextWordEnd(text, end)
	}
	return start, end
}

func windowBounds(text string, p occurrencePos, radius int) (int, int) {
	start := p.Start - radius
	if start < 0 {
		start = 0
	}
	end := p.End + radius
	if end > len(text) {
		end = len(text)
	}
	for start > 0 && text[start] != ' ' {
		start--
	}
	for end < len(text) && text[end-1] != ' ' {
		end++
	}
	return start, end
}

func previousWordStart(text string, pos int) int {
	for pos > 0 && unicode.IsSpace(rune(text[pos-1])) {
		pos--
	}
	for pos > 0 && !unicode.IsSpace(rune(text[pos-1])) {
		pos--
	}
	return pos
}

func nextWordEnd(text string, pos int) int {
	for pos < len(text) && unicode.IsSpace(rune(text[pos])) {
		pos++
	}
	for pos < len(text) && !unicode.IsSpace(rune(text[pos])) {
		pos++
	}
	return pos
}

func highlightSnippet(snippet string, primary occurrencePos, terms []string) string {
	positions := []occurrencePos{primary}
	for _, p := range occurrencePositions(snippet, terms) {
		if p.Start == primary.Start && p.End == primary.End {
			continue
		}
		positions = append(positions, p)
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i].Start < positions[j].Start })
	positions = dedupeOccurrences(positions)
	var b strings.Builder
	last := 0
	for _, p := range positions {
		if p.Start < last || p.Start < 0 || p.End > len(snippet) {
			continue
		}
		b.WriteString(snippet[last:p.Start])
		b.WriteString("[[[")
		b.WriteString(snippet[p.Start:p.End])
		b.WriteString("]]]")
		last = p.End
	}
	b.WriteString(snippet[last:])
	return b.String()
}

func (r *ContentRepo) scanBooks(query string, args ...any) ([]*Book, error) {
	return (&BookRepo{db: r.db}).scanBooks(query, args...)
}

func (r *ContentRepo) hydrate(books []*Book) error {
	return (&BookRepo{db: r.db}).hydrate(books)
}

func nullInt(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func cleanFragments(in []ContentFragment) []ContentFragment {
	out := make([]ContentFragment, 0, len(in))
	for _, f := range in {
		f.Text = strings.TrimSpace(f.Text)
		if f.Text == "" {
			continue
		}
		if f.Page <= 0 {
			f.Page = len(out) + 1
		}
		if len([]rune(f.Text)) > 4000 {
			f.Text = truncateRunes(f.Text, 4000)
		}
		out = append(out, f)
	}
	return out
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

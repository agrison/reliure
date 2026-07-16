package core

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"
)

// ReadingState mirrors a book's KOReader progress.
type ReadingState struct {
	BookID       int64
	Percent      float64 // 0..1
	Pages        int     // total pages of the device rendering (0 = unknown)
	Status       string  // reading|complete|abandoned|new
	Device       string
	LastReadAt   string // KOReader summary.modified, verbatim
	Rating       int    // 1..5 star rating (0 = unrated)
	RatingManual bool   // rating set inside Reliure → protected from device sync
	SyncedAt     time.Time
}

// Annotation is a highlight and/or note attached to a book.
type Annotation struct {
	ID        int64
	BookID    int64
	Text      string
	Note      string
	Chapter   string
	Drawer    string
	CreatedAt string // device datetime, verbatim
}

// ReadingRepo persists reading progress and annotations.
type ReadingRepo struct{ db *sql.DB }

// UpsertState inserts or replaces a book's reading state.
func (r *ReadingRepo) UpsertState(s ReadingState) error {
	if s.SyncedAt.IsZero() {
		s.SyncedAt = time.Now().UTC()
	}
	_, err := r.db.Exec(`
		INSERT INTO reading_state (book_id, percent, pages, status, device, last_read_at, rating, rating_manual, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(book_id) DO UPDATE SET
			percent = excluded.percent,
			pages = excluded.pages,
			status = excluded.status,
			device = excluded.device,
			last_read_at = excluded.last_read_at,
			rating = excluded.rating,
			rating_manual = excluded.rating_manual,
			synced_at = excluded.synced_at`,
		s.BookID, s.Percent, s.Pages, s.Status, s.Device, s.LastReadAt, s.Rating, boolInt(s.RatingManual), s.SyncedAt.Format(time.RFC3339))
	return err
}

// MergeDeviceState applies a state read from a KOReader device. Device sync is
// one-directional per field:
//   - Progress/status: adopted only when the device is FURTHER ALONG (KOReader can
//     advance Reliure, never roll back a status set by hand or a higher progress
//     from another device).
//   - Rating: a rating the user set inside Reliure is authoritative and never
//     overwritten; otherwise the device rating is mirrored.
//
// Annotations are handled separately and always taken from the device.
func (r *ReadingRepo) MergeDeviceState(s ReadingState) error {
	existing, ok, err := r.State(s.BookID)
	if err != nil {
		return err
	}
	if !ok {
		s.RatingManual = false // came from the device, not set by hand
		return r.UpsertState(s)
	}
	merged := existing
	if effectivePercent(s) > effectivePercent(existing) {
		merged.Percent = s.Percent
		merged.Status = s.Status
		merged.LastReadAt = s.LastReadAt
		merged.Device = s.Device
	}
	if merged.Pages == 0 && s.Pages > 0 {
		merged.Pages = s.Pages // learn the page count when the device knows it
	}
	if !existing.RatingManual {
		merged.Rating = s.Rating // mirror the device rating unless set by hand
	}
	return r.UpsertState(merged)
}

// SetRating records a star rating (1..5; 0 clears it) chosen inside Reliure. A
// non-zero rating is flagged manual so a later device sync won't overwrite it;
// clearing it (0) reverts to device-driven so KOReader's rating can fill in again.
func (r *ReadingRepo) SetRating(bookID int64, rating int) error {
	if rating < 0 {
		rating = 0
	}
	if rating > 5 {
		rating = 5
	}
	existing, ok, err := r.State(bookID)
	if err != nil {
		return err
	}
	if !ok {
		existing = ReadingState{BookID: bookID}
	}
	existing.Rating = rating
	existing.RatingManual = rating > 0
	return r.UpsertState(existing)
}

// DeleteState removes a book's reading state (used by "mark as unread").
func (r *ReadingRepo) DeleteState(bookID int64) error {
	_, err := r.db.Exec(`DELETE FROM reading_state WHERE book_id = ?`, bookID)
	return err
}

// effectivePercent treats a completed book as fully read whatever its stored
// percentage, so "complete" always outranks a partial percentage.
func effectivePercent(s ReadingState) float64 {
	if s.Status == "complete" {
		return 1
	}
	return s.Percent
}

// State returns a book's reading state, if any.
func (r *ReadingRepo) State(bookID int64) (ReadingState, bool, error) {
	row := r.db.QueryRow(`
		SELECT book_id, percent, pages, status, device, last_read_at, rating, rating_manual, synced_at
		FROM reading_state WHERE book_id = ?`, bookID)
	s, err := scanState(row)
	if err == sql.ErrNoRows {
		return ReadingState{}, false, nil
	}
	if err != nil {
		return ReadingState{}, false, err
	}
	return s, true, nil
}

// AllStates returns every reading state, keyed by book id (for grid badges).
func (r *ReadingRepo) AllStates() (map[int64]ReadingState, error) {
	rows, err := r.db.Query(`
		SELECT book_id, percent, pages, status, device, last_read_at, rating, rating_manual, synced_at FROM reading_state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]ReadingState)
	for rows.Next() {
		s, err := scanState(rows)
		if err != nil {
			return nil, err
		}
		out[s.BookID] = s
	}
	return out, rows.Err()
}

// ReplaceAnnotations swaps a book's annotations for the given set in one
// transaction, so a re-sync reflects the device's current state exactly. The
// dedup key (text+note+chapter+datetime) collapses accidental duplicates.
func (r *ReadingRepo) ReplaceAnnotations(bookID int64, anns []Annotation) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM annotation WHERE book_id = ?`, bookID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO annotation (book_id, text, note, chapter, drawer, created_at, dedup_key)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, a := range anns {
		key := annotationKey(a)
		if _, err := stmt.Exec(a.BookID, a.Text, a.Note, a.Chapter, a.Drawer, a.CreatedAt, key); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Annotations returns a book's annotations, newest-created first.
func (r *ReadingRepo) Annotations(bookID int64) ([]Annotation, error) {
	rows, err := r.db.Query(`
		SELECT id, book_id, text, note, chapter, drawer, created_at
		FROM annotation WHERE book_id = ? ORDER BY created_at DESC, id DESC`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Annotation
	for rows.Next() {
		var a Annotation
		if err := rows.Scan(&a.ID, &a.BookID, &a.Text, &a.Note, &a.Chapter, &a.Drawer, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AnnotationCounts returns the number of annotations per book (for badges).
func (r *ReadingRepo) AnnotationCounts() (map[int64]int, error) {
	rows, err := r.db.Query(`SELECT book_id, COUNT(*) FROM annotation GROUP BY book_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]int)
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// StatusCounts returns the number of books in each non-empty reading status
// (e.g. {"reading": 3, "complete": 12}).
func (r *ReadingRepo) StatusCounts() (map[string]int, error) {
	rows, err := r.db.Query(`SELECT status, COUNT(*) FROM reading_state WHERE status != '' GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}

func scanState(s interface{ Scan(...any) error }) (ReadingState, error) {
	var (
		st           ReadingState
		ratingManual int
		synced       string
	)
	if err := s.Scan(&st.BookID, &st.Percent, &st.Pages, &st.Status, &st.Device, &st.LastReadAt, &st.Rating, &ratingManual, &synced); err != nil {
		return ReadingState{}, err
	}
	st.RatingManual = ratingManual != 0
	if t, err := time.Parse(time.RFC3339, synced); err == nil {
		st.SyncedAt = t
	}
	return st, nil
}

func annotationKey(a Annotation) string {
	h := sha256.Sum256([]byte(a.Text + "\x00" + a.Note + "\x00" + a.Chapter + "\x00" + a.CreatedAt))
	return hex.EncodeToString(h[:16])
}

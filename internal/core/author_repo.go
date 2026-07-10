package core

import (
	"database/sql"
	"errors"
	"strings"
)

// AuthorRepo manages authors.
type AuthorRepo struct{ db *sql.DB }

// deriveSortName produces a basic "Last, First" key from a full name when no
// explicit sort form is given. Deliberately simple: the last word is the key.
func deriveSortName(name string) string {
	fields := strings.Fields(name)
	if len(fields) < 2 {
		return name
	}
	last := fields[len(fields)-1]
	rest := strings.Join(fields[:len(fields)-1], " ")
	return last + ", " + rest
}

func scanAuthor(s interface{ Scan(...any) error }) (Author, error) {
	var a Author
	err := s.Scan(&a.ID, &a.Name, &a.SortName)
	return a, err
}

// GetOrCreate returns the author with the given name, creating it if needed.
// The name is unique, so concurrent calls converge on the same row.
func (r *AuthorRepo) GetOrCreate(name string) (Author, error) {
	return getOrCreateAuthor(r.db, name)
}

// getOrCreateAuthor is the shared logic (direct or within a transaction).
func getOrCreateAuthor(q querier, name string) (Author, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Author{}, errors.New("empty author name")
	}
	if a, err := scanAuthor(q.QueryRow(`SELECT id, name, sort_name FROM author WHERE name = ?`, name)); err == nil {
		return a, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Author{}, err
	}
	sortName := deriveSortName(name)
	res, err := q.Exec(
		`INSERT INTO author (name, sort_name) VALUES (?, ?)
		 ON CONFLICT(name) DO UPDATE SET name = excluded.name`,
		name, sortName)
	if err != nil {
		return Author{}, err
	}
	id, err := res.LastInsertId()
	if err != nil || id == 0 {
		// ON CONFLICT may not return a reliable id: re-read by name.
		return scanAuthor(q.QueryRow(`SELECT id, name, sort_name FROM author WHERE name = ?`, name))
	}
	return Author{ID: id, Name: name, SortName: sortName}, nil
}

// ByName fetches an author by exact name.
func (r *AuthorRepo) ByName(name string) (Author, error) {
	row := r.db.QueryRow(`SELECT id, name, sort_name FROM author WHERE name = ?`, name)
	return scanAuthor(row)
}

// ByID fetches an author by id.
func (r *AuthorRepo) ByID(id int64) (Author, error) {
	row := r.db.QueryRow(`SELECT id, name, sort_name FROM author WHERE id = ?`, id)
	return scanAuthor(row)
}

// List returns all authors sorted by sort_name.
func (r *AuthorRepo) List() ([]Author, error) {
	rows, err := r.db.Query(`SELECT id, name, sort_name FROM author ORDER BY sort_name, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Author
	for rows.Next() {
		a, err := scanAuthor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Rename changes the name (and recomputes the derived sort key).
func (r *AuthorRepo) Rename(id int64, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.New("empty author name")
	}
	_, err := r.db.Exec(`UPDATE author SET name = ?, sort_name = ? WHERE id = ?`,
		newName, deriveSortName(newName), id)
	return err
}

package core

import (
	"database/sql"
	"errors"
	"strings"
)

// TagRepo manages tags.
type TagRepo struct{ db *sql.DB }

func scanTag(s interface{ Scan(...any) error }) (Tag, error) {
	var t Tag
	err := s.Scan(&t.ID, &t.Name)
	return t, err
}

// GetOrCreate returns the tag with the given name, creating it if needed.
func (r *TagRepo) GetOrCreate(name string) (Tag, error) {
	return getOrCreateTag(r.db, name)
}

func getOrCreateTag(q querier, name string) (Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Tag{}, errors.New("empty tag name")
	}
	if t, err := scanTag(q.QueryRow(`SELECT id, name FROM tag WHERE name = ?`, name)); err == nil {
		return t, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Tag{}, err
	}
	res, err := q.Exec(
		`INSERT INTO tag (name) VALUES (?)
		 ON CONFLICT(name) DO UPDATE SET name = excluded.name`, name)
	if err != nil {
		return Tag{}, err
	}
	id, err := res.LastInsertId()
	if err != nil || id == 0 {
		return scanTag(q.QueryRow(`SELECT id, name FROM tag WHERE name = ?`, name))
	}
	return Tag{ID: id, Name: name}, nil
}

// ByName fetches a tag by exact name.
func (r *TagRepo) ByName(name string) (Tag, error) {
	row := r.db.QueryRow(`SELECT id, name FROM tag WHERE name = ?`, name)
	return scanTag(row)
}

// List returns all tags sorted by name.
func (r *TagRepo) List() ([]Tag, error) {
	rows, err := r.db.Query(`SELECT id, name FROM tag ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tag
	for rows.Next() {
		t, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

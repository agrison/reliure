package core

import (
	"database/sql"
	"errors"
	"strings"
)

// SeriesRepo manages series.
type SeriesRepo struct{ db *sql.DB }

func scanSeries(s interface{ Scan(...any) error }) (Series, error) {
	var x Series
	err := s.Scan(&x.ID, &x.Name, &x.SortName)
	return x, err
}

// GetOrCreate returns the series with the given name, creating it if needed.
func (r *SeriesRepo) GetOrCreate(name string) (Series, error) {
	return getOrCreateSeries(r.db, name)
}

func getOrCreateSeries(q querier, name string) (Series, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Series{}, errors.New("empty series name")
	}
	if s, err := scanSeries(q.QueryRow(`SELECT id, name, sort_name FROM series WHERE name = ?`, name)); err == nil {
		return s, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Series{}, err
	}
	// Sort key: the name as-is (series have no "Last, First" convention).
	res, err := q.Exec(
		`INSERT INTO series (name, sort_name) VALUES (?, ?)
		 ON CONFLICT(name) DO UPDATE SET name = excluded.name`,
		name, name)
	if err != nil {
		return Series{}, err
	}
	id, err := res.LastInsertId()
	if err != nil || id == 0 {
		return scanSeries(q.QueryRow(`SELECT id, name, sort_name FROM series WHERE name = ?`, name))
	}
	return Series{ID: id, Name: name, SortName: name}, nil
}

// ByName fetches a series by exact name.
func (r *SeriesRepo) ByName(name string) (Series, error) {
	row := r.db.QueryRow(`SELECT id, name, sort_name FROM series WHERE name = ?`, name)
	return scanSeries(row)
}

// ByID fetches a series by id.
func (r *SeriesRepo) ByID(id int64) (Series, error) {
	row := r.db.QueryRow(`SELECT id, name, sort_name FROM series WHERE id = ?`, id)
	return scanSeries(row)
}

// List returns all series, sorted.
func (r *SeriesRepo) List() ([]Series, error) {
	rows, err := r.db.Query(`SELECT id, name, sort_name FROM series ORDER BY sort_name, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Series
	for rows.Next() {
		s, err := scanSeries(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ShelfRule is one predicate in a dynamic smart shelf. Field/Operator are
// intentionally strings so the app layer can evolve the rule vocabulary without
// new migrations.
type ShelfRule struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// SmartShelf is a persisted dynamic collection definition.
type SmartShelf struct {
	ID        int64
	Name      string
	Match     string // all|any
	Rules     []ShelfRule
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SmartShelfRepo persists dynamic shelves. It does not evaluate rules; the app
// layer evaluates them because some predicates depend on connected-device state.
type SmartShelfRepo struct{ db *sql.DB }

// List returns all shelves in display order.
func (r *SmartShelfRepo) List() ([]SmartShelf, error) {
	rows, err := r.db.Query(`SELECT id, name, match, rules_json, position, created_at, updated_at
		FROM smart_shelf ORDER BY position, name COLLATE NOCASE, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SmartShelf
	for rows.Next() {
		sh, err := scanShelf(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

// ByID returns one shelf.
func (r *SmartShelfRepo) ByID(id int64) (SmartShelf, error) {
	return scanShelf(r.db.QueryRow(`SELECT id, name, match, rules_json, position, created_at, updated_at
		FROM smart_shelf WHERE id = ?`, id))
}

// Save creates or updates a shelf.
func (r *SmartShelfRepo) Save(sh SmartShelf) (SmartShelf, error) {
	sh.Name = strings.TrimSpace(sh.Name)
	if sh.Name == "" {
		return SmartShelf{}, errors.New("empty shelf name")
	}
	if sh.Match != "any" {
		sh.Match = "all"
	}
	rules, err := json.Marshal(sh.Rules)
	if err != nil {
		return SmartShelf{}, err
	}
	ts := now()
	if sh.ID == 0 {
		if sh.Position == 0 {
			_ = r.db.QueryRow(`SELECT COALESCE(MAX(position), 0) + 1 FROM smart_shelf`).Scan(&sh.Position)
		}
		res, err := r.db.Exec(`INSERT INTO smart_shelf (name, match, rules_json, position, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			sh.Name, sh.Match, string(rules), sh.Position, ts.Format(rfc3339), ts.Format(rfc3339))
		if err != nil {
			return SmartShelf{}, err
		}
		sh.ID, err = res.LastInsertId()
		if err != nil {
			return SmartShelf{}, err
		}
		return r.ByID(sh.ID)
	}
	res, err := r.db.Exec(`UPDATE smart_shelf SET name = ?, match = ?, rules_json = ?, position = ?, updated_at = ? WHERE id = ?`,
		sh.Name, sh.Match, string(rules), sh.Position, ts.Format(rfc3339), sh.ID)
	if err != nil {
		return SmartShelf{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return SmartShelf{}, sql.ErrNoRows
	}
	return r.ByID(sh.ID)
}

// Delete removes a shelf definition. Books are unaffected because membership is
// computed dynamically.
func (r *SmartShelfRepo) Delete(id int64) error {
	res, err := r.db.Exec(`DELETE FROM smart_shelf WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type shelfScanner interface{ Scan(...any) error }

func scanShelf(row shelfScanner) (SmartShelf, error) {
	var (
		sh      SmartShelf
		raw     string
		created string
		updated string
	)
	if err := row.Scan(&sh.ID, &sh.Name, &sh.Match, &raw, &sh.Position, &created, &updated); err != nil {
		return SmartShelf{}, err
	}
	if err := json.Unmarshal([]byte(raw), &sh.Rules); err != nil {
		return SmartShelf{}, err
	}
	sh.CreatedAt, _ = time.Parse(rfc3339, created)
	sh.UpdatedAt, _ = time.Parse(rfc3339, updated)
	return sh, nil
}

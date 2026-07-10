package core

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migration is a versioned SQL file. Its version is the numeric filename
// prefix: "0001_init.sql" → version 1.
type migration struct {
	version int
	name    string
	sql     string
}

// loadMigrations reads and sorts the embedded migrations.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, err
	}
	var migs []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(e.Name(), "_")
		if !ok {
			return nil, fmt.Errorf("malformed migration name (want NNNN_desc.sql): %s", e.Name())
		}
		v, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("invalid version prefix in %s: %w", e.Name(), err)
		}
		body, err := fs.ReadFile(migrationFS, "migrations/"+e.Name())
		if err != nil {
			return nil, err
		}
		migs = append(migs, migration{version: v, name: e.Name(), sql: string(body)})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })

	// Guard against duplicate or non-positive versions.
	for i, m := range migs {
		if m.version <= 0 {
			return nil, fmt.Errorf("invalid migration version (%d): %s", m.version, m.name)
		}
		if i > 0 && migs[i-1].version == m.version {
			return nil, fmt.Errorf("duplicate migration version: %d", m.version)
		}
	}
	return migs, nil
}

// currentVersion reads the applied schema version (0 if the table is absent).
func currentVersion(db *sql.DB) (int, error) {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return 0, err
	}
	var v sql.NullInt64
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&v); err != nil {
		return 0, err
	}
	if !v.Valid {
		return 0, nil
	}
	return int(v.Int64), nil
}

// migrate applies every migration whose version exceeds the current one, each
// in its own transaction. Idempotent.
func migrate(db *sql.DB) error {
	migs, err := loadMigrations()
	if err != nil {
		return err
	}
	current, err := currentVersion(db)
	if err != nil {
		return err
	}
	for _, m := range migs {
		if m.version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(m.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_version (version) VALUES (?)`, m.version); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s (recording version): %w", m.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration %s (commit): %w", m.name, err)
		}
	}
	return nil
}

package core

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// querier abstracts *sql.DB and *sql.Tx, so shared logic (upserts, links) can
// run both directly and inside a transaction.
type querier interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// DB wraps the SQLite connection and exposes the domain repositories. It is the
// entry point of the core package.
type DB struct {
	sql *sql.DB

	Books   *BookRepo
	Authors *AuthorRepo
	Series  *SeriesRepo
	Tags    *TagRepo
	Reading *ReadingRepo
	Shelves *SmartShelfRepo
}

// Open opens (or creates) the database at path, applies pending migrations and
// wires the repositories. Use ":memory:" for tests.
func Open(path string) (*DB, error) {
	// modernc.org/sqlite: pure-Go driver, registered as "sqlite".
	dsn := path
	if path == ":memory:" {
		// One in-memory database shared across the pool's connections.
		dsn = "file::memory:?cache=shared"
	}
	dsn += pragmaSuffix(dsn)

	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// A file database handles concurrency via WAL; a shared in-memory one does
	// not, so we pin it to a single connection to avoid surprises in tests.
	if path == ":memory:" {
		sqldb.SetMaxOpenConns(1)
	}
	if err := sqldb.Ping(); err != nil {
		sqldb.Close()
		return nil, err
	}
	if err := migrate(sqldb); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("migrations: %w", err)
	}

	db := &DB{sql: sqldb}
	db.Books = &BookRepo{db: sqldb}
	db.Authors = &AuthorRepo{db: sqldb}
	db.Series = &SeriesRepo{db: sqldb}
	db.Tags = &TagRepo{db: sqldb}
	db.Reading = &ReadingRepo{db: sqldb}
	db.Shelves = &SmartShelfRepo{db: sqldb}
	return db, nil
}

// pragmaSuffix appends modernc connection pragmas to a DSN.
func pragmaSuffix(dsn string) string {
	sep := "?"
	for i := 0; i < len(dsn); i++ {
		if dsn[i] == '?' {
			sep = "&"
			break
		}
	}
	// foreign_keys: referential integrity (ON DELETE CASCADE). busy_timeout:
	// smooth over transient locks. journal_mode=WAL: concurrent read/write for
	// a file database.
	return sep + "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
}

// Close closes the underlying connection.
func (db *DB) Close() error { return db.sql.Close() }

// SQL exposes the raw connection (handy for one-off needs; domain code should
// go through the repositories instead).
func (db *DB) SQL() *sql.DB { return db.sql }

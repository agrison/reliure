-- Initial library schema.
-- A book has N files (formats), N authors (via book_author), 0..1 series and
-- N tags. Full-text search goes through an FTS5 table maintained by the
-- application code (a denormalized blob of title + authors + series + tags).

CREATE TABLE author (
    id        INTEGER PRIMARY KEY,
    name      TEXT NOT NULL,
    sort_name TEXT NOT NULL DEFAULT '',
    UNIQUE (name)
);

CREATE TABLE series (
    id        INTEGER PRIMARY KEY,
    name      TEXT NOT NULL,
    sort_name TEXT NOT NULL DEFAULT '',
    UNIQUE (name)
);

CREATE TABLE book (
    id            INTEGER PRIMARY KEY,
    title         TEXT NOT NULL,
    title_sort    TEXT NOT NULL DEFAULT '',
    description   TEXT NOT NULL DEFAULT '',
    language      TEXT NOT NULL DEFAULT '',
    isbn          TEXT NOT NULL DEFAULT '',
    published_at  TEXT NOT NULL DEFAULT '',           -- publication date, free-form (ISO when known)
    series_id     INTEGER REFERENCES series(id) ON DELETE SET NULL,
    series_index  REAL,
    added_at      TEXT NOT NULL,                       -- RFC3339 UTC
    updated_at    TEXT NOT NULL,                       -- RFC3339 UTC
    cover_path    TEXT NOT NULL DEFAULT ''             -- cached thumbnail, relative to the config dir
);

CREATE INDEX idx_book_series ON book (series_id);
CREATE INDEX idx_book_title_sort ON book (title_sort);

CREATE TABLE book_author (
    book_id   INTEGER NOT NULL REFERENCES book(id) ON DELETE CASCADE,
    author_id INTEGER NOT NULL REFERENCES author(id) ON DELETE CASCADE,
    role      TEXT NOT NULL DEFAULT 'aut',            -- MARC relator code (aut, edt, trl…)
    position  INTEGER NOT NULL DEFAULT 0,             -- display order
    PRIMARY KEY (book_id, author_id, role)
);

CREATE INDEX idx_book_author_author ON book_author (author_id);

CREATE TABLE tag (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    UNIQUE (name)
);

CREATE TABLE book_tag (
    book_id INTEGER NOT NULL REFERENCES book(id) ON DELETE CASCADE,
    tag_id  INTEGER NOT NULL REFERENCES tag(id) ON DELETE CASCADE,
    PRIMARY KEY (book_id, tag_id)
);

CREATE INDEX idx_book_tag_tag ON book_tag (tag_id);

CREATE TABLE file (
    id       INTEGER PRIMARY KEY,
    book_id  INTEGER NOT NULL REFERENCES book(id) ON DELETE CASCADE,
    path     TEXT NOT NULL,                           -- absolute path, or relative to the library dir
    format   TEXT NOT NULL,                           -- 'epub', 'pdf'…
    size     INTEGER NOT NULL DEFAULT 0,
    sha256   TEXT NOT NULL DEFAULT '',
    added_at TEXT NOT NULL,
    UNIQUE (path)
);

CREATE INDEX idx_file_book ON file (book_id);
CREATE INDEX idx_file_sha256 ON file (sha256);

-- Full-text search. rowid = book.id. Populated/updated by code (no triggers:
-- the indexed data comes from several joined tables). Standard FTS5 table (it
-- stores the text) so plain INSERT/UPDATE/DELETE work, which keeps re-syncing
-- a changed book simple. remove_diacritics makes search accent-insensitive.
CREATE VIRTUAL TABLE book_fts USING fts5 (
    title,
    authors,
    series,
    tags,
    tokenize = "unicode61 remove_diacritics 2"
);

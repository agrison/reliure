-- Full-text index for extracted ebook contents. The metadata FTS table remains
-- separate so title/author search can still rank well and the content index can
-- be enabled/rebuilt independently.

CREATE TABLE content_index (
    book_id    INTEGER PRIMARY KEY REFERENCES book(id) ON DELETE CASCADE,
    file_id    INTEGER REFERENCES file(id) ON DELETE SET NULL,
    status     TEXT NOT NULL DEFAULT 'indexed', -- indexed | empty | failed
    chars      INTEGER NOT NULL DEFAULT 0,
    error      TEXT NOT NULL DEFAULT '',
    indexed_at TEXT NOT NULL
);

CREATE INDEX idx_content_index_status ON content_index (status);

CREATE VIRTUAL TABLE content_fts USING fts5 (
    book_id UNINDEXED,
    ordinal UNINDEXED,
    page UNINDEXED,
    body,
    tokenize = "unicode61 remove_diacritics 2"
);

-- Move content FTS from one row per book to one row per searchable fragment so
-- the UI can show several contextual snippets ordered inside each book.
DROP TABLE IF EXISTS content_fts;

CREATE VIRTUAL TABLE content_fts USING fts5 (
    book_id UNINDEXED,
    ordinal UNINDEXED,
    page UNINDEXED,
    body,
    tokenize = "unicode61 remove_diacritics 2"
);

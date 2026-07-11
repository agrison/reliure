-- Reading progress and annotations mirrored from KOReader sidecars.

-- One row per book that has been read on a device (percent 0..1, status).
CREATE TABLE reading_state (
    book_id      INTEGER PRIMARY KEY REFERENCES book(id) ON DELETE CASCADE,
    percent      REAL NOT NULL DEFAULT 0,     -- 0..1
    status       TEXT NOT NULL DEFAULT '',    -- reading|complete|abandoned|new
    device       TEXT NOT NULL DEFAULT '',    -- source device name, if known
    last_read_at TEXT NOT NULL DEFAULT '',    -- KOReader summary.modified, verbatim
    synced_at    TEXT NOT NULL                -- RFC3339 UTC when Reliure imported it
);

-- Highlights and notes. dedup_key makes a re-sync idempotent per book.
CREATE TABLE annotation (
    id         INTEGER PRIMARY KEY,
    book_id    INTEGER NOT NULL REFERENCES book(id) ON DELETE CASCADE,
    text       TEXT NOT NULL DEFAULT '',
    note       TEXT NOT NULL DEFAULT '',
    chapter    TEXT NOT NULL DEFAULT '',
    drawer     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',       -- annotation datetime from the device
    dedup_key  TEXT NOT NULL,
    UNIQUE (book_id, dedup_key)
);

CREATE INDEX idx_annotation_book ON annotation (book_id);
CREATE INDEX idx_reading_status ON reading_state (status);

-- Dynamic shelves. A shelf stores rule definitions; matching books are computed
-- at read time so the shelf stays up to date as metadata, reading state and
-- device inventory change.
CREATE TABLE smart_shelf (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    match      TEXT NOT NULL DEFAULT 'all', -- all|any
    rules_json TEXT NOT NULL DEFAULT '[]',
    position   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_smart_shelf_position ON smart_shelf (position, name);

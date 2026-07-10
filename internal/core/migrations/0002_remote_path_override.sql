-- Per-book KOReader destination path override. When disabled, the global
-- template remains the source for the effective remote path.
ALTER TABLE book ADD COLUMN remote_path_override_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE book ADD COLUMN remote_path_override TEXT NOT NULL DEFAULT '';

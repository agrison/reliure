-- Total page count of a book's rendering on the device (KOReader `doc_pages`),
-- so the UI can show "page X / Y" alongside the progress bar. 0 when unknown.
ALTER TABLE reading_state ADD COLUMN pages INTEGER NOT NULL DEFAULT 0;

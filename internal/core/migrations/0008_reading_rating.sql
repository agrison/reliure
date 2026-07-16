-- KOReader star rating (summary.rating, 1..5; 0 = unrated). rating_manual marks a
-- rating the user set inside Reliure, which device sync must never overwrite.
ALTER TABLE reading_state ADD COLUMN rating INTEGER NOT NULL DEFAULT 0;
ALTER TABLE reading_state ADD COLUMN rating_manual INTEGER NOT NULL DEFAULT 0;

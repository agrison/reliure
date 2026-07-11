# Database

The database is SQLite, accessed through the pure-Go `modernc.org/sqlite`
driver (no cgo). All database code lives in `internal/core`. The schema is
defined by embedded, versioned SQL migrations; the canonical source of truth is
`internal/core/migrations/`.

## Location

- **Database file:** the OS config directory + `reliure/library.db`
  (`os.UserConfigDir()`, i.e. `~/Library/Application Support/reliure/` on macOS,
  `%AppData%\reliure\` on Windows, `~/.config/reliure/` on Linux). See
  `core.DefaultDBPath`.
- **Library directory** (where ebook files are copied and organised) is a
  **separate, user-configurable** concept — it is *not* the config dir. Session 3
  wires it up.
- Tests use an in-memory database via `core.Open(":memory:")`.

## Connection pragmas

Set on every connection (see `core.pragmaSuffix`):

| Pragma | Value | Why |
| --- | --- | --- |
| `foreign_keys` | `1` | Enforce referential integrity — the `ON DELETE CASCADE`/`SET NULL` rules below depend on it. |
| `journal_mode` | `WAL` | Concurrent readers + one writer for the on-disk database. |
| `busy_timeout` | `5000` ms | Ride out transient locks instead of failing immediately. |

The shared in-memory database is pinned to a single connection (`MaxOpenConns=1`)
so tests behave deterministically.

## Schema

One book has many files (formats), many authors (with a role and order), zero
or one series, and many tags.

```
author ──┐                          ┌── tag
         │ book_author              │ book_tag
         │ (role, position)         │
         ▼                          ▼
        book ──────< file           (book ↔ tag many-to-many)
         │
         └──> series   (book.series_id, book.series_index)

        book_fts  (FTS5, rowid = book.id)
```

### Tables

- **`book`** — the central entity. Scalar metadata (`title`, `title_sort`,
  `description`, `language`, `isbn`, `published_at`, `cover_path`), a nullable
  `series_id` (+ `series_index REAL`), and `added_at` / `updated_at` timestamps
  stored as RFC3339 UTC strings. `published_at` is free-form text (ISO 8601 when
  known) because ebook metadata dates are notoriously irregular.
- **`author`** — unique `name`, plus a `sort_name` ("Last, First") used for
  ordering. When not supplied, `sort_name` is derived from the name.
- **`series`** — unique `name` + `sort_name`.
- **`tag`** — unique `name`.
- **`book_author`** — the contribution link. Primary key `(book_id, author_id,
  role)` so the same person can be, say, both author and translator of a work.
  Carries the MARC `role` ("aut", "trl", "edt"…) and a `position` for display
  order.
- **`book_tag`** — plain many-to-many join.
- **`file`** — one row per physical file (a book can have several formats,
  currently EPUB and PDF). `path` is UNIQUE (a file lives in exactly one place),
  and `sha256` is indexed to support deduplication in Session 3.
- **`reading_state`** — one row per book read on a KOReader device (mirrored
  from its `.sdr` sidecars): `percent` (0..1), `pages` (total pages of the device
  rendering, 0 = unknown; migration `0004`), `status`
  (reading/complete/abandoned/new), `device`, `last_read_at` (KOReader's
  `summary.modified`, verbatim) and `synced_at` (RFC3339). Primary key `book_id`.
  Indexed by `status` (sidebar reading filters).
- **`annotation`** — highlights and notes imported from KOReader: `text`,
  `note`, `chapter`, `drawer` (highlight style), `created_at` (device datetime)
  and a `dedup_key` with `UNIQUE (book_id, dedup_key)` so a re-sync is
  idempotent. Indexed by `book_id`. Added in migration `0003`.
- **`schema_version`** — bookkeeping for the migration runner (see below).

Deleting a book cascades to `book_author`, `book_tag`, `file`, `reading_state`
and `annotation` (via `ON DELETE CASCADE`); its `book_fts` row is removed
explicitly in the same transaction. Deleting a series sets dependent
`book.series_id` to NULL.

### Sorting conventions

Listings order by the sort key, falling back to the display value when the sort
key is empty: `CASE WHEN title_sort='' THEN title ELSE title_sort END COLLATE
NOCASE`. Series listings order by `series_index` first.

## Full-text search (FTS5)

`book_fts` is an FTS5 virtual table with columns `title, authors, series, tags`
and `rowid = book.id`. Two decisions worth noting:

- **Standard (content-storing) FTS5 table, not `content=''`.** A contentless
  table forbids ordinary `UPDATE`/`DELETE`, which would make re-indexing a book
  after an edit painful. The stored text is tiny for a personal library, so we
  keep a normal table and get plain DML.
- **`tokenize = "unicode61 remove_diacritics 2"`** → search is
  accent-insensitive (`etranger` matches "L'Étranger").

The index is **maintained by application code**, not triggers, because the
indexed text is denormalized across several joined tables. `core.syncFTS`
rebuilds a book's row (title + concatenated author names + series + tags) inside
the same write transaction as any create/update. Queries rank by `bm25` (`ORDER
BY rank`). User input is sanitised into a safe MATCH expression in
`core.ftsQuery`: each term is quoted and suffixed with `*` for prefix matching,
combined with implicit AND.

## Import deduplication & covers

The import pipeline (`internal/library`) relies on two `core` queries:

- `BookRepo.FindByFileSHA` — an identical file (same `file.sha256`) is skipped as
  a duplicate.
- `BookRepo.FindByTitleAuthor` — a case-insensitive title+primary-author match
  lets a new file be **attached** to an existing book as another format instead
  of creating a duplicate.

Cover thumbnails are generated at import and cached as `<CoverDir>/<bookID>.jpg`;
`book.cover_path` stores the `<bookID>.jpg` name, resolved against the cover
directory (under the OS config dir by default). `BookRepo.SetCover` updates it.

## Migrations

A deliberately small, home-grown runner (`internal/core/migrate.go`):

- SQL files live in `internal/core/migrations/`, embedded with `//go:embed`.
- Filenames are `NNNN_description.sql`; the numeric prefix is the version
  (`0001_init.sql` → 1).
- A `schema_version` table records applied versions. On `Open`, every migration
  with a version greater than the current maximum is applied **in its own
  transaction**, in order. The process is idempotent — reopening an up-to-date
  database is a no-op.
- The loader rejects duplicate or non-positive versions.

**To add a migration:** drop a new `000N_description.sql` in the migrations
directory with the next number. Never edit an already-released migration —
append a new one. Update this document when the schema changes.

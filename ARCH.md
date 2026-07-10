# Architecture

Reliure is a cross-platform desktop EPUB library manager. The backend is 100 %
Go; the UI is a lightweight web frontend rendered in the system webview via
[Wails v3](https://v3.wails.io). See `AGENTS.md` for the session-by-session
plan and product scope.

## Guiding principle: the framework is an implementation detail

All real functionality lives in framework-agnostic packages under `internal/`.
The Wails binary is just one entry point on top of them; a future headless
`cmd/cli` (a pure OPDS server for a NAS/VPS) reuses the exact same packages.
This keeps us insulated from Wails v3's moving alpha tooling — if we ever had to
drop to v2 or another shell, `internal/` would be untouched.

```
cmd/
  app/          Wails v3 desktop entry point (thin glue)
  cli/          (later) headless / server mode
internal/
  core/         models, SQLite, migrations, repositories, full-text search
  formats/      FormatHandler interface + registry (extensibility seam)
    epub/       EPUB parser + cover/thumbnail extraction (Session 2, done)
  library/      import pipeline: dedup, file organisation, thumbnails (Session 3, done)
  settings/     persisted user preferences (import mode, library dir)
  opds/         OPDS server over net/http (Session 6)
  calibre/      Calibre "smart device" wireless protocol (Session 7)
  hooks/        user scripting on lifecycle events (Session 8)
frontend/       Svelte + Vite UI, embedded into the binary
```

## Layers

- **`internal/core`** — the heart. Owns the domain models (`Book`, `Author`,
  `Series`, `Tag`, `File`), the SQLite database, a small home-grown migration
  runner, the repositories (CRUD, listing by author/series, pagination) and
  FTS5 search. Zero dependency on Wails or the frontend. Fully unit-tested
  against an in-memory database. See `DB.md` for the data model.

- **`internal/formats`** — the extensibility seam. `FormatHandler` is the
  contract every format implements (`Format`, `CanHandle`, `Metadata`,
  `Cover`); a `Registry` dispatches a file to the first handler that accepts it.
  Handlers produce a neutral `BookMetadata` that the `library` layer maps onto
  `core` models, so parsing stays independent of storage. Adding PDF later is
  one new package that registers itself — nothing else changes. The first
  concrete handler, **`formats/epub`**, is implemented: it parses container.xml
  → OPF → Dublin Core plus Calibre (`calibre:series`…) and EPUB3 (`refines`,
  `belongs-to-collection`) metadata, resolves covers through a fallback chain,
  and generates JPEG thumbnails. It is exhaustively tolerant — malformed input
  never panics and degrades to a filename title. It self-registers into
  `formats.Default` via `init` (blank-import idiom).

- **`internal/library`** — the import pipeline. Detects a file's format via the
  registry, extracts metadata, deduplicates (SHA-256 for identical files; a
  title+author heuristic to attach a new format to an existing book), copies the
  file into `LibraryDir/Author/Title/`, inserts it via `core`, and caches a JPEG
  cover thumbnail. Imports run on a worker pool: parsing/hashing/cover extraction
  fan out across workers, while a **single committer goroutine** performs the DB
  writes — this keeps SQLite's lone writer happy and makes deduplication
  race-free without locks. Progress is reported per file through a callback, in
  completion order.

- **`internal/settings`** — persisted user preferences as JSON in the OS config
  dir (`settings.json`), behind a concurrency-safe `Store` with atomic writes
  and defaults. The headline preference is the **import mode**:
  - `copy` — Reliure copies imported files into a managed library tree
    (`Author/Title/`); it owns those copies.
  - `reference` — files are indexed where they already live; `file.path` points
    at the user's original and nothing is copied. Ideal for an existing
    collection the user doesn't want duplicated.
  The two are exclusive. Cover thumbnails are cached by Reliure in both modes.
  The pipeline never deletes a referenced original, only copies it made.

- **`cmd/app`** — the desktop shell. Creates the Wails application, registers Go
  *services* whose public methods are callable from JS, and opens the main
  window. It stays deliberately thin: it wires `internal/*` to the UI and holds
  no business logic. `App` exposes a `Ping()` health check; `LibraryService`
  wraps the importer and the catalog. Import: `ChooseAndImport()` opens a native
  picker (multiple EPUB files and/or folders); `ImportPaths()` handles a mix of
  files and directories and is also the target of window drag-and-drop — both
  build an importer from the *current* settings (so a mode change takes effect at
  once) and forward progress as `import:progress` / `import:done` events.
  Catalog: `Books`/`Search`/`BooksBy{Author,Series,Tag}` return `BookCard`s,
  `Authors`/`SeriesList`/`Tags` feed the sidebar with counts, `Book` returns the
  detail. `SettingsService` reads/writes preferences. Cover thumbnails are served
  as files by a custom asset handler (`/covers/…` from the cover cache), never
  inlined as base64.

- **`frontend`** — Svelte 5 + Vite, TypeScript. The library UI: a sidebar
  (All / Authors / Series / Tags, with counts) drives a main area that toggles
  between a cover grid and a list, with instant debounced full-text search,
  sorts, a book detail drawer, a settings modal and a drag-and-drop overlay.
  Components live in `src/lib/`; `src/lib/api.ts` re-exports the generated
  bindings under a short path. It talks to Go exclusively through the
  **generated bindings** (`frontend/bindings/…`), which give strongly-typed
  JS/TS wrappers around the bound Go methods and models. No hand-written IPC.

## Go ↔ JS data flow

```
Svelte component
  → import { App } from "…/bindings/github.com/agrison/reliure/cmd/app"
  → App.Ping()                         (typed async call)
      → Wails runtime (system webview bridge)
          → Go: (*App).Ping() returns PingResult
      ← PingResult marshalled to JS
  ← rendered in the UI
```

Bindings are regenerated by `wails3 generate bindings ./...` during every build,
so the TS API can never drift from the Go signatures. Assets are served to the
webview by an in-process HTTP asset server backed by the embedded frontend.

## Project-layout choices (deviations from the stock Wails template)

The stock `wails3 init` template puts `main.go` at the module root and builds
the root package. We keep the Wails entry under **`cmd/app`** (per the target
tree in `AGENTS.md`), which required three small, deliberate adjustments:

1. **Asset embedding lives in the `frontend` package** (`frontend/embed.go`),
   not in `main`. `go:embed` cannot reference parent directories, so the embed
   must sit at/above `frontend/dist`; `cmd/app` imports `frontend.Assets`. The
   Wails asset server locates `index.html` inside the FS automatically, so the
   embed prefix needs no stripping.
2. **The build tasks target `./cmd/app`.** The generated `go build` commands in
   `build/{darwin,linux,windows}/Taskfile.yml` and `build/Taskfile.yml` had
   `./cmd/app` appended to their `-o` output.
3. **Binding generation scans the whole module** (`./...`) instead of just the
   root, so services are found wherever they live.

The mobile (iOS/Android) scaffolding from the template was removed: the product
is desktop-only (macOS first, then Windows/Linux).

## Toolchain & build

- **Go 1.26**, **Node 22+**, **Wails CLI pinned** to `v3.0.0-alpha2.117`
  (`go install github.com/wailsapp/wails/v3/cmd/wails3@<version>`). The version
  is pinned in the CI workflow and must be bumped deliberately after reading the
  changelog — Wails v3 is alpha and its tooling moves (see `AGENTS.md`, Risk 1).
- **Task runner:** the project uses [Task](https://taskfile.dev) via the CLI's
  embedded runner (`wails3 task <name>`). Common commands:
  - `wails3 task build` — generate bindings, build the frontend, compile the app
    into `bin/reliure`.
  - `wails3 task package` — build and produce a `.app` bundle.
  - `wails3 task dev` — hot-reloading dev loop (Vite dev server + Go rebuilds).
  - `go test ./internal/...` — the framework-agnostic unit tests.
- **SQLite driver:** `modernc.org/sqlite` — pure Go, no cgo, so cross-compilation
  stays trivial. (Note: the Wails webview layer itself does use cgo on desktop.)

## CI

`.github/workflows/ci.yml` runs on macOS (`macos-14`): it runs the core tests
with `-race`, vets, installs the pinned Wails CLI, runs `wails3 doctor`, builds
the app and uploads the binary. Windows and Linux jobs wait until we can test
those webviews (`AGENTS.md`, Session 9).

## Testing strategy

- `internal/core` is covered by table-ish unit tests against an in-memory
  SQLite database (`Open(":memory:")`): migrations, CRUD, cascade deletes,
  get-or-create de-duplication, ordering, pagination, FTS search (including
  accent-insensitivity and index re-sync on update), and real file-backed
  persistence across reopen.
- Later sessions add a corpus of real (and deliberately broken) EPUBs for the
  parser — the single most valuable test investment of the project.

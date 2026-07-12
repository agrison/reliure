<h1>
  <img src="frontend/public/reliure-logo.png" alt="" width="96" height="96" align="center">
  Reliure
</h1>

Reliure is a desktop ebook library manager built for readers who use
KOReader and want a fast, clean way to organise, enrich and send books to their
device.

It focuses on the everyday workflow around a personal library: importing books,
fixing metadata, browsing covers, integrating with KOReader by tracking what is 
already on the reader, and moving files there with the right folder structure.

I need it for my personal use, and my LLM friend is kind enough to help me build it,
but I thought maybe others would be interested.

## Downloads

Prebuilt releases are available from the
[GitHub Releases page](https://github.com/agrison/reliure/releases).

- [macOS](https://github.com/agrison/reliure/releases/latest/download/reliure-macos-arm64.zip)
- [Windows](https://github.com/agrison/reliure/releases/latest/download/reliure-windows-amd64.zip)
- [Linux amd64](https://github.com/agrison/reliure/releases/latest/download/reliure-linux-amd64.zip)

## Screenshots

| Feature | Screenshot |
| --- | --- |
| Currently reading | <img src="screenshots/currently_reading.png" alt="Currently reading" width="520"> |
| KOReader currently reading sync | <img src="screenshots/currently_reading_koreader.png" alt="KOReader currently reading sync" width="520"> |
| KOReader reading details | <img src="screenshots/currently_reading_koreader_details.png" alt="KOReader reading details" width="520"> |
| Discover | <img src="screenshots/discover.png" alt="Discover" width="520"> |
| Dynamic shelves | <img src="screenshots/dynamic_shelf.png" alt="Dynamic shelves" width="520"> |
| Full-text search | <img src="screenshots/fulltext.png" alt="Full-text search" width="520"> |
| Full-text search results | <img src="screenshots/fulltext2.png" alt="Full-text search results" width="520"> |
| Full-text search in dark mode | <img src="screenshots/fulltext_dark.png" alt="Full-text search in dark mode" width="520"> |
| Internationalized interface | <img src="screenshots/ui_another_language.png" alt="Internationalized interface" width="520"> |

## What Reliure Does

- Builds a local ebook library from files or folders.
- Imports books by copying them into a managed library, or by referencing files
  where they already are.
- Extracts metadata, authors, series, tags, language, identifiers and covers.
- Shows the library as a cover grid, list, author view, series view or tag view.
- Provides fast search across the library.
- Lets you edit metadata book by book or quickly in a spreadsheet-like table.
- Generates and regenerates cover thumbnails.
- Supports light and dark themes.

## KOReader First

Reliure is designed to be a good companion app for KOReader.

- Send selected books wirelessly through the Calibre wireless protocol.
- Expose the library as an OPDS catalog for pull-based downloads.
- Configure the remote path used when books are sent to the reader.
- Keep a `.reliure` inventory file on the reader so the app can show which
  books are already present.
- Sync reading progress and annotations from KOReader sidecar files.

## Metadata And Discovery

Reliure includes tools to improve and complete messy libraries:

- Online metadata lookup from Google Books, OpenLibrary and the BnF catalog.
- Per-field metadata merge, so you choose what to keep or replace.
- Cover replacement from online results.
- Project Gutenberg discovery and import for public domain books.

## Supported Formats

Reliure currently focuses on EPUB and PDF, with an extensible format system for
future additions.

Current support includes:

- EPUB, EPUB 3 and common EPUB-derived extensions such as `.epub.images`,
  `.epub.noimages`, `.epub3.images`, `.kepub` and `.kepub.epub`.
- PDF metadata import.
- Cover thumbnails from common image formats including JPEG, PNG, GIF and WebP.

## Current State

Reliure is actively developed and already covers the core library and KOReader
workflow. It is not positioned as a Calibre clone; the goal is a lighter,
modern desktop app with strong KOReader integration.

Some areas are still evolving, especially packaging, cross-platform polish and
future format support.

## Built With

Reliure is a desktop app with:

- Go for the backend and library logic.
- SQLite for local storage.
- Svelte for the interface.
- Wails v3 for the native desktop shell.

Developer-focused documentation lives separately:

- [ARCH.md](ARCH.md) for architecture.
- [DB.md](DB.md) for the database model.
- [FEATURES.md](FEATURES.md) for product ideas and prioritisation.
- [AGENTS.md](AGENTS.md) for the implementation session plan.

## Building From Source

On macOS, the app bundle can be built with:

```bash
env PATH=/Users/alex/go/bin:$PATH GOCACHE=/private/tmp/reliure-go-cache /Users/alex/go/bin/wails3 task package -f
```

The generated app is:

```text
bin/reliure.app
```

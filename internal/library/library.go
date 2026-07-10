// Package library implements the import pipeline: it detects a file's format
// via the formats registry, extracts metadata, deduplicates (SHA-256 plus a
// title/author heuristic), copies the file into the library tree
// (Author/Title/), inserts it into the core database and caches a cover
// thumbnail. Imports run on a worker pool and report progress, so the UI can
// show a live count while a whole Calibre library is ingested.
//
// It depends only on internal/core and internal/formats — no UI framework — so
// the same pipeline backs the desktop app and any future headless tool.
package library

import (
	"runtime"

	"github.com/agrison/reliure/internal/core"
	"github.com/agrison/reliure/internal/formats"
)

// Outcome is the result of importing a single file.
type Outcome int

const (
	// OutcomeImported: a new book was created.
	OutcomeImported Outcome = iota
	// OutcomeAttached: the file was attached to an existing book as an extra
	// format (title/author matched an existing entry).
	OutcomeAttached
	// OutcomeDuplicate: an identical file (same SHA-256) already existed; skipped.
	OutcomeDuplicate
	// OutcomeFailed: the file could not be imported (I/O error, unsupported…).
	OutcomeFailed
)

func (o Outcome) String() string {
	switch o {
	case OutcomeImported:
		return "imported"
	case OutcomeAttached:
		return "attached"
	case OutcomeDuplicate:
		return "duplicate"
	case OutcomeFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Progress is reported once per processed file, in order, from a single
// goroutine (safe to forward straight to UI events).
type Progress struct {
	Total   int    // total files in this import
	Done    int    // files processed so far, including this one
	Path    string // source file
	Outcome Outcome
	BookID  int64  // resulting/target book (0 on failure)
	Title   string // resolved title (filename fallback for broken files)
	Err     error  // non-nil only for OutcomeFailed
}

// Summary aggregates an import run.
type Summary struct {
	Total      int
	Imported   int
	Attached   int
	Duplicates int
	Failed     int
}

func (s *Summary) add(o Outcome) {
	s.Total++
	switch o {
	case OutcomeImported:
		s.Imported++
	case OutcomeAttached:
		s.Attached++
	case OutcomeDuplicate:
		s.Duplicates++
	case OutcomeFailed:
		s.Failed++
	}
}

// Mode selects how imported files are managed on disk.
type Mode string

const (
	// ModeCopy copies imported files into a managed library tree
	// (LibraryDir/Author/Title/). Reliure owns those copies.
	ModeCopy Mode = "copy"
	// ModeReference indexes files where they already live, without copying:
	// file.path points at the user's original file. Ideal for an existing
	// collection the user doesn't want duplicated.
	ModeReference Mode = "reference"
)

// Valid reports whether m is a known mode.
func (m Mode) Valid() bool { return m == ModeCopy || m == ModeReference }

// Config controls an Importer.
type Config struct {
	// Mode selects copy vs. in-place reference. Defaults to ModeCopy.
	Mode Mode
	// LibraryDir is the root files are copied into in ModeCopy (Author/Title/).
	// Unused in ModeReference.
	LibraryDir string
	// CoverDir is where cover thumbnails are cached (named <bookID>.jpg).
	// Reliure always owns this cache, in both modes.
	CoverDir string
	// ThumbnailMax is the largest side, in pixels, of generated thumbnails.
	ThumbnailMax int
	// Workers is the number of parallel parse/hash workers.
	Workers int
	// Merge attaches a file to an existing book when title+author match, instead
	// of creating a duplicate. Enabled by default.
	Merge bool
}

// Importer runs imports against a database using the given configuration.
type Importer struct {
	db  *core.DB
	reg *formats.Registry
	cfg Config
}

// New builds an Importer. Zero-valued config fields get sensible defaults; the
// default format registry (into which formats/epub registers itself) is used.
func New(db *core.DB, cfg Config) *Importer {
	if !cfg.Mode.Valid() {
		cfg.Mode = ModeCopy
	}
	if cfg.ThumbnailMax <= 0 {
		cfg.ThumbnailMax = 400
	}
	if cfg.Workers <= 0 {
		cfg.Workers = runtime.NumCPU()
	}
	return &Importer{db: db, reg: formats.Default, cfg: cfg}
}

// metadataToBook maps format-neutral metadata onto a core.Book (without files,
// which are added once copied).
func metadataToBook(md formats.BookMetadata) *core.Book {
	b := &core.Book{
		Title:       md.Title,
		TitleSort:   md.TitleSort,
		Description: md.Description,
		Language:    md.Language,
		ISBN:        md.ISBN,
		PublishedAt: md.Published,
		SeriesIndex: md.SeriesIndex,
	}
	if md.Series != "" {
		b.Series = &core.Series{Name: md.Series}
	}
	for i, c := range md.Contributors {
		b.Authors = append(b.Authors, core.Contribution{
			Author:   core.Author{Name: c.Name, SortName: c.SortName},
			Role:     c.Role,
			Position: i,
		})
	}
	for _, t := range md.Tags {
		b.Tags = append(b.Tags, core.Tag{Name: t})
	}
	return b
}

// primaryAuthor returns the first contributor's name, or "" when unknown.
func primaryAuthor(md formats.BookMetadata) string {
	if len(md.Contributors) > 0 {
		return md.Contributors[0].Name
	}
	return ""
}

package main

import (
	"context"
	"os"

	"github.com/agrison/reliure/internal/core"
	"github.com/agrison/reliure/internal/library"
	"github.com/agrison/reliure/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Frontend event names: one per imported file, and one when an import finishes
// (used to refresh the view after a drag-and-drop import that no call awaits).
const (
	progressEvent = "import:progress"
	doneEvent     = "import:done"
)

// Registering events gives the binding generator strongly-typed JS/TS APIs for
// their payloads.
func init() {
	application.RegisterEvent[ImportProgress](progressEvent)
	application.RegisterEvent[ImportSummary](doneEvent)
}

// LibraryService exposes library operations to the frontend: importing folders
// of ebooks (with live progress events) and basic stats. It is a thin adapter
// over internal/library and internal/core.
type LibraryService struct {
	db       *core.DB
	settings *settings.Store
	coverDir string
}

// ImportProgress is the per-file payload sent on the "import:progress" event.
type ImportProgress struct {
	Total   int    `json:"total"`
	Done    int    `json:"done"`
	Title   string `json:"title"`
	Outcome string `json:"outcome"`
	BookID  int64  `json:"bookId"`
	Error   string `json:"error,omitempty"`
}

// ImportSummary is returned when an import finishes.
type ImportSummary struct {
	Total      int `json:"total"`
	Imported   int `json:"imported"`
	Attached   int `json:"attached"`
	Duplicates int `json:"duplicates"`
	Failed     int `json:"failed"`
}

// LibraryStats is a small snapshot for the UI.
type LibraryStats struct {
	Books int `json:"books"`
}

// ChooseAndImport opens a native picker that accepts any mix of EPUB files and
// folders (multiple selection), and imports them. Folders are scanned
// recursively. Emits an "import:progress" event per file. An empty selection
// (dialog cancelled) is a no-op.
func (s *LibraryService) ChooseAndImport() (ImportSummary, error) {
	paths, err := application.Get().Dialog.OpenFile().
		CanChooseFiles(true).
		CanChooseDirectories(true).
		AddFilter("Livres EPUB", "*.epub").
		SetTitle("Choisir des livres ou des dossiers à importer").
		PromptForMultipleSelection()
	if err != nil || len(paths) == 0 {
		return ImportSummary{}, err
	}
	return s.ImportPaths(paths)
}

// ImportPaths imports a mix of files and directories (directories are scanned
// recursively) using the current import mode. It emits an "import:progress"
// event per file and an "import:done" event with the summary at the end — the
// latter lets a drag-and-drop import (which nothing awaits) refresh the view.
func (s *LibraryService) ImportPaths(paths []string) (ImportSummary, error) {
	// Build the importer from current settings so a mode/library-dir change
	// takes effect immediately, without restarting the app.
	cfg := s.settings.Get()
	imp := library.New(s.db, library.Config{
		Mode:       cfg.ImportMode,
		LibraryDir: cfg.LibraryDir,
		CoverDir:   s.coverDir,
		Merge:      true,
	})

	files := expandPaths(imp, paths)
	sum, err := imp.Import(context.Background(), files, func(p library.Progress) {
		ev := ImportProgress{
			Total:   p.Total,
			Done:    p.Done,
			Title:   p.Title,
			Outcome: p.Outcome.String(),
			BookID:  p.BookID,
		}
		if p.Err != nil {
			ev.Error = p.Err.Error()
		}
		application.Get().Event.Emit(progressEvent, ev)
	})

	summary := ImportSummary{
		Total:      sum.Total,
		Imported:   sum.Imported,
		Attached:   sum.Attached,
		Duplicates: sum.Duplicates,
		Failed:     sum.Failed,
	}
	application.Get().Event.Emit(doneEvent, summary)
	return summary, err
}

// expandPaths turns a mix of files and directories into a flat file list:
// directories are scanned recursively for supported files, plain files pass
// through unchanged.
func expandPaths(imp *library.Importer, paths []string) []string {
	var files []string
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			if scanned, err := imp.Scan(p); err == nil {
				files = append(files, scanned...)
			}
			continue
		}
		files = append(files, p)
	}
	return files
}

// Stats returns basic counts for the UI.
func (s *LibraryService) Stats() (LibraryStats, error) {
	n, err := s.db.Books.Count()
	return LibraryStats{Books: n}, err
}

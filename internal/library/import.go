package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/agrison/reliure/internal/core"
	"github.com/agrison/reliure/internal/formats"
)

// parsed is the read-only work product of the parallel stage: everything needed
// to commit a file, gathered without touching the database.
type parsed struct {
	path    string
	format  string
	md      formats.BookMetadata
	metaErr error // non-fatal: metadata was degraded (filename-title fallback)
	sha     string
	size    int64
	cover   []byte
	err     error // fatal: the file could not be processed at all
}

// Import ingests the given files. Parsing, hashing and cover extraction run on a
// worker pool; the database commit runs on a single goroutine (this one), which
// keeps SQLite's single writer happy and makes deduplication race-free.
// onProgress, if non-nil, is called once per file, in completion order, from
// this goroutine. A cancelled context stops feeding new files and is returned.
func (imp *Importer) Import(ctx context.Context, paths []string, onProgress func(Progress)) (Summary, error) {
	var summary Summary
	total := len(paths)
	if total == 0 {
		return summary, nil
	}

	jobs := make(chan string)
	results := make(chan parsed, imp.cfg.Workers)

	// Feeder: hand out paths until exhausted or cancelled. The explicit check
	// guarantees prompt, deterministic cancellation (a bare select could pick
	// the ready send over a ready ctx.Done()).
	go func() {
		defer close(jobs)
		for _, path := range paths {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case jobs <- path:
			}
		}
	}()

	// Parallel parse/hash workers.
	var wg sync.WaitGroup
	for i := 0; i < imp.cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				results <- imp.prepare(path)
			}
		}()
	}
	go func() { wg.Wait(); close(results) }()

	// Single committer: dedup + copy + DB + thumbnail, then report.
	for p := range results {
		prog := imp.commit(p)
		summary.add(prog.Outcome)
		prog.Total = total
		prog.Done = summary.Total
		if onProgress != nil {
			onProgress(prog)
		}
	}
	return summary, ctx.Err()
}

// ImportDir scans root for supported files and imports them.
func (imp *Importer) ImportDir(ctx context.Context, root string, onProgress func(Progress)) (Summary, error) {
	paths, err := imp.Scan(root)
	if err != nil {
		return Summary{}, err
	}
	return imp.Import(ctx, paths, onProgress)
}

// prepare does all the read-only work for one file (no DB access).
func (imp *Importer) prepare(path string) parsed {
	p := parsed{path: path}
	h, ok := imp.reg.HandlerFor(path)
	if !ok {
		p.err = fmt.Errorf("unsupported format %q", filepath.Ext(path))
		return p
	}
	p.format = h.Format()
	p.md, p.metaErr = h.Metadata(path) // tolerant: md is always usable

	sha, size, err := hashFile(path)
	if err != nil {
		p.err = fmt.Errorf("hashing %s: %w", filepath.Base(path), err)
		return p
	}
	p.sha, p.size = sha, size

	if cover, err := h.Cover(path); err == nil {
		p.cover = cover
	}
	return p
}

// commit applies one prepared file to the database. Runs on the single
// committer goroutine.
func (imp *Importer) commit(p parsed) Progress {
	prog := Progress{Path: p.path, Title: p.md.Title}
	if p.err != nil {
		return failed(prog, p.err)
	}

	// 1. Exact duplicate: identical file content already imported.
	if id, found, err := imp.db.Books.FindByFileSHA(p.sha); err != nil {
		return failed(prog, err)
	} else if found {
		prog.Outcome, prog.BookID = OutcomeDuplicate, id
		return prog
	}

	author := primaryAuthor(p.md)

	// 2. Heuristic merge: same title+author → attach as another format.
	if imp.cfg.Merge {
		if id, found, err := imp.db.Books.FindByTitleAuthor(p.md.Title, author); err != nil {
			return failed(prog, err)
		} else if found {
			return imp.attach(prog, p, id)
		}
	}

	// 3. New book.
	return imp.create(prog, p, author)
}

func (imp *Importer) create(prog Progress, p parsed, author string) Progress {
	dst, copied, err := imp.place(p, author)
	if err != nil {
		return failed(prog, err)
	}
	book := metadataToBook(p.md)
	book.Files = []core.File{{Path: dst, Format: p.format, Size: p.size, SHA256: p.sha}}
	if err := imp.db.Books.Create(book); err != nil {
		imp.cleanup(dst, copied) // never delete the user's original in reference mode
		return failed(prog, err)
	}
	prog.Outcome, prog.BookID, prog.Title = OutcomeImported, book.ID, book.Title
	imp.cacheCover(book.ID, p.cover)
	return prog
}

func (imp *Importer) attach(prog Progress, p parsed, bookID int64) Progress {
	dst, copied, err := imp.place(p, primaryAuthor(p.md))
	if err != nil {
		return failed(prog, err)
	}
	if _, err := imp.db.Books.AddFile(bookID, core.File{
		Path: dst, Format: p.format, Size: p.size, SHA256: p.sha,
	}); err != nil {
		imp.cleanup(dst, copied)
		return failed(prog, err)
	}
	prog.Outcome, prog.BookID = OutcomeAttached, bookID
	imp.cacheCoverIfMissing(bookID, p.cover)
	return prog
}

// place makes the file available to the library and returns its stored path.
// In ModeCopy it copies the file into the managed tree (copied=true); in
// ModeReference it indexes the original in place (copied=false), so file.path
// points at the user's own file.
func (imp *Importer) place(p parsed, author string) (path string, copied bool, err error) {
	if imp.cfg.Mode == ModeReference {
		return p.path, false, nil
	}
	dst, err := placeFile(p.path, imp.destPath(author, p.md.Title, p.path, p.format))
	return dst, true, err
}

// cleanup removes a file we created, but never one we merely referenced.
func (imp *Importer) cleanup(path string, copied bool) {
	if copied {
		os.Remove(path)
	}
}

// cacheCover generates and stores a thumbnail for a book (best-effort: a
// missing or unreadable cover is not an import failure).
func (imp *Importer) cacheCover(bookID int64, cover []byte) {
	if len(cover) == 0 || imp.cfg.CoverDir == "" {
		return
	}
	thumb, err := formats.Thumbnail(cover, imp.cfg.ThumbnailMax)
	if err != nil {
		return
	}
	if err := os.MkdirAll(imp.cfg.CoverDir, 0o755); err != nil {
		return
	}
	name := strconv.FormatInt(bookID, 10) + ".jpg"
	if err := os.WriteFile(filepath.Join(imp.cfg.CoverDir, name), thumb, 0o644); err != nil {
		return
	}
	_ = imp.db.Books.SetCover(bookID, name)
}

// cacheCoverIfMissing only caches a cover when the book has none yet.
func (imp *Importer) cacheCoverIfMissing(bookID int64, cover []byte) {
	if b, err := imp.db.Books.ByID(bookID); err == nil && b.CoverPath == "" {
		imp.cacheCover(bookID, cover)
	}
}

func failed(prog Progress, err error) Progress {
	prog.Outcome, prog.Err = OutcomeFailed, err
	return prog
}

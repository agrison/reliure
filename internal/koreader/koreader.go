// Package koreader reads KOReader's per-book sidecar metadata (the
// "<book>.sdr/metadata.<ext>.lua" files) to recover reading progress, status
// and annotations (highlights + notes) so Reliure can mirror them.
//
// The sidecar is a Lua chunk of the form `return { ... }`. We evaluate it with a
// sandboxed Lua VM — no standard library is opened and a deadline guards against
// pathological input — so a sidecar behaves as pure data, never as code. Both
// the modern unified `annotations` array and the legacy `highlight`/`bookmarks`
// tables are understood.
package koreader

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// Status is the normalized reading status KOReader stores under summary.status.
type Status string

const (
	StatusUnknown   Status = ""
	StatusNew       Status = "new" // opened, not started (a.k.a. "tbr")
	StatusReading   Status = "reading"
	StatusComplete  Status = "complete"
	StatusAbandoned Status = "abandoned"
)

// Annotation is one highlight and/or note.
type Annotation struct {
	Text     string // highlighted passage (may be empty for a bare note)
	Note     string // user note attached to the highlight
	Chapter  string
	Datetime string // KOReader's "YYYY-MM-DD HH:MM:SS", kept verbatim
	Drawer   string // highlight style ("lighten", "underscore"…)
}

// Sidecar is the parsed content of one metadata.<ext>.lua file.
type Sidecar struct {
	Path            string // the metadata file path
	DocBasename     string // best-effort document filename (e.g. "Book.epub")
	Format          string // document extension without dot ("epub", "pdf")
	Title           string // doc_props.title
	Authors         []string
	Language        string
	PercentFinished float64 // 0..1
	TotalPages      int     // doc_pages / stats.pages, 0 when unknown
	Status          Status
	ModifiedAt      string // summary.modified, verbatim
	Rating          int    // summary.rating, 1..5 (0 = unrated)
	Annotations     []Annotation
}

// parseDeadline bounds evaluation of a single sidecar.
const parseDeadline = 5 * time.Second

// ParseFile reads and parses one sidecar metadata file.
func ParseFile(path string) (*Sidecar, error) {
	p := newParser()
	defer p.close()
	return p.parseFile(path)
}

// Parse parses sidecar bytes that were obtained without a file path (e.g.
// fetched from a device over the Calibre protocol). Path/Format/DocBasename are
// left empty; progress, status, doc_props and annotations are filled.
func Parse(data []byte) (*Sidecar, error) {
	p := newParser()
	defer p.close()
	return p.parse(string(data))
}

// Scan walks root and parses every "*.sdr/metadata.*.lua" it finds. A file that
// fails to parse is skipped (its error is returned via errs) rather than
// aborting the whole scan.
func Scan(root string) (sidecars []*Sidecar, errs []error) {
	p := newParser()
	defer p.close()
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable dir: skip, keep going
		}
		if d.IsDir() || !isMetadataFile(d.Name()) {
			return nil
		}
		sc, perr := p.parseFile(path)
		if perr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, perr))
			return nil
		}
		sidecars = append(sidecars, sc)
		return nil
	})
	return sidecars, errs
}

// isMetadataFile matches "metadata.<ext>.lua".
func isMetadataFile(name string) bool {
	return strings.HasPrefix(name, "metadata.") && strings.HasSuffix(name, ".lua")
}

// parser holds a reusable sandboxed Lua state.
type parser struct{ L *lua.LState }

func newParser() *parser {
	// SkipOpenLibs: no base/os/io libraries — a data table needs none, and their
	// absence means a malicious sidecar can't reach the host.
	return &parser{L: lua.NewState(lua.Options{SkipOpenLibs: true})}
}

func (p *parser) close() { p.L.Close() }

func (p *parser) parseFile(path string) (*Sidecar, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	sc, err := p.parse(string(data))
	if err != nil {
		return nil, err
	}
	sc.Path = path
	sc.Format = formatFromMetadataName(filepath.Base(path))
	sc.DocBasename = docBasename(path, sc.Format)
	return sc, nil
}

// parse evaluates the sidecar chunk and extracts the fields we care about.
func (p *parser) parse(src string) (*Sidecar, error) {
	L := p.L
	// Reset the stack from any previous parse and bound this evaluation.
	L.SetTop(0)
	ctx, cancel := context.WithTimeout(context.Background(), parseDeadline)
	defer cancel()
	L.SetContext(ctx)

	if err := L.DoString(src); err != nil {
		return nil, fmt.Errorf("evaluate sidecar: %w", err)
	}
	if L.GetTop() == 0 {
		return nil, fmt.Errorf("sidecar returned no value")
	}
	root, ok := L.Get(-1).(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("sidecar is not a table")
	}

	sc := &Sidecar{
		PercentFinished: getNumber(root, "percent_finished"),
		TotalPages:      totalPages(root),
	}
	if summary, ok := getTable(root, "summary"); ok {
		sc.Status = normalizeStatus(getString(summary, "status"))
		sc.ModifiedAt = getString(summary, "modified")
		sc.Rating = clampRating(int(getNumber(summary, "rating")))
	}
	if props, ok := getTable(root, "doc_props"); ok {
		sc.Title = getString(props, "title")
		sc.Authors = splitAuthors(getString(props, "authors"))
		sc.Language = getString(props, "language")
	}
	sc.Annotations = extractAnnotations(root)
	return sc, nil
}

// extractAnnotations prefers the modern `annotations` array; if absent it falls
// back to the legacy `highlight` (page → entries) and `bookmarks` tables.
func extractAnnotations(root *lua.LTable) []Annotation {
	if arr, ok := getTable(root, "annotations"); ok {
		var out []Annotation
		arr.ForEach(func(_, v lua.LValue) {
			if t, ok := v.(*lua.LTable); ok {
				out = append(out, annotationFrom(t))
			}
		})
		if len(out) > 0 {
			return out
		}
	}
	return legacyAnnotations(root)
}

func legacyAnnotations(root *lua.LTable) []Annotation {
	var out []Annotation
	// highlight = { [page] = { [i] = {text, chapter, datetime, drawer} } }
	if hl, ok := getTable(root, "highlight"); ok {
		hl.ForEach(func(_, page lua.LValue) {
			if pageTbl, ok := page.(*lua.LTable); ok {
				pageTbl.ForEach(func(_, v lua.LValue) {
					if t, ok := v.(*lua.LTable); ok {
						out = append(out, annotationFrom(t))
					}
				})
			}
		})
	}
	// bookmarks carry notes in the legacy layout; keep the ones with a note.
	if bm, ok := getTable(root, "bookmarks"); ok {
		bm.ForEach(func(_, v lua.LValue) {
			t, ok := v.(*lua.LTable)
			if !ok {
				return
			}
			note := firstNonEmpty(getString(t, "note"), getString(t, "notes"))
			if note == "" {
				return
			}
			out = append(out, Annotation{
				Note:     note,
				Text:     getString(t, "text"),
				Chapter:  getString(t, "chapter"),
				Datetime: getString(t, "datetime"),
			})
		})
	}
	return out
}

// totalPages reads the rendering's page count: KOReader stores it top-level as
// `doc_pages`, with `stats.pages` as a fallback when the statistics plugin wrote
// it instead.
func totalPages(root *lua.LTable) int {
	if n := int(getNumber(root, "doc_pages")); n > 0 {
		return n
	}
	if stats, ok := getTable(root, "stats"); ok {
		if n := int(getNumber(stats, "pages")); n > 0 {
			return n
		}
	}
	return 0
}

func annotationFrom(t *lua.LTable) Annotation {
	return Annotation{
		Text:     getString(t, "text"),
		Note:     getString(t, "note"),
		Chapter:  getString(t, "chapter"),
		Datetime: getString(t, "datetime"),
		Drawer:   getString(t, "drawer"),
	}
}

// --- Lua value helpers ---

func getString(t *lua.LTable, key string) string {
	if s, ok := t.RawGetString(key).(lua.LString); ok {
		return strings.TrimSpace(string(s))
	}
	return ""
}

// clampRating keeps a rating within KOReader's 0..5 range.
func clampRating(n int) int {
	if n < 0 {
		return 0
	}
	if n > 5 {
		return 5
	}
	return n
}

func getNumber(t *lua.LTable, key string) float64 {
	if n, ok := t.RawGetString(key).(lua.LNumber); ok {
		return float64(n)
	}
	return 0
}

func getTable(t *lua.LTable, key string) (*lua.LTable, bool) {
	sub, ok := t.RawGetString(key).(*lua.LTable)
	return sub, ok
}

// --- string helpers ---

func normalizeStatus(s string) Status {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "reading":
		return StatusReading
	case "complete", "finished":
		return StatusComplete
	case "abandoned":
		return StatusAbandoned
	case "new", "tbr":
		return StatusNew
	default:
		return StatusUnknown
	}
}

// splitAuthors splits KOReader's authors field (newline-separated) into names.
func splitAuthors(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '\n' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// formatFromMetadataName extracts "epub" from "metadata.epub.lua".
func formatFromMetadataName(name string) string {
	name = strings.TrimSuffix(name, ".lua")
	name = strings.TrimPrefix(name, "metadata.")
	return strings.ToLower(name)
}

// docBasename reconstructs the document filename from the sidecar directory
// name ("Book.sdr" → "Book.epub"). Used as a secondary matching signal.
func docBasename(metadataPath, format string) string {
	dir := filepath.Base(filepath.Dir(metadataPath)) // "Book.sdr"
	stem := strings.TrimSuffix(dir, ".sdr")
	if stem == "" || format == "" {
		return stem
	}
	if strings.EqualFold(filepath.Ext(stem), "."+format) {
		return stem // already carries the extension
	}
	return stem + "." + format
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

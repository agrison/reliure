// Package formats defines the extensibility contract for ebook formats: a
// FormatHandler interface plus a Registry that dispatches a file to the first
// handler able to read it. Adding a new format (e.g. PDF) means adding one
// package that implements FormatHandler and registers it — no changes here.
//
// It deliberately does not import internal/core: handlers produce a neutral
// BookMetadata that the library layer maps onto core models, keeping parsing
// independent of storage.
package formats

import (
	"path/filepath"
	"strings"
)

// Contributor is a person involved in a work (author, translator, editor…).
// Role uses MARC relator codes ("aut", "trl", "edt"); empty means author.
type Contributor struct {
	Name     string
	SortName string // "Last, First" form when the source provides one
	Role     string
}

// BookMetadata is the format-neutral result of parsing a file. Fields are
// best-effort: a tolerant handler returns as much as it can and leaves the rest
// zero rather than failing (see Session 2's robustness requirement).
type BookMetadata struct {
	Title        string
	TitleSort    string
	Contributors []Contributor
	Series       string
	SeriesIndex  *float64
	Description  string
	Language     string
	Publisher    string
	Published    string // free-form date, ISO 8601 when known
	ISBN         string
	Identifiers  map[string]string // scheme → value (e.g. "isbn", "calibre", "uuid")
	Tags         []string
}

// MetadataWriter is an optional capability a FormatHandler may also implement:
// writing metadata back into the file on disk. The library checks for it with a
// type assertion, so formats that can't write (yet) simply don't implement it.
type MetadataWriter interface {
	// WriteMetadata rewrites the file's embedded metadata to match md, in place.
	WriteMetadata(path string, md BookMetadata) error
}

// FormatHandler reads a single ebook format. Implementations must be safe to
// call concurrently and must not panic on malformed input.
type FormatHandler interface {
	// Format is the short lower-case identifier stored on files ("epub", "pdf").
	Format() string
	// CanHandle reports whether this handler recognises the file at path. It
	// should be cheap (extension and/or magic-byte sniffing), not a full parse.
	CanHandle(path string) bool
	// Metadata parses and returns the file's metadata.
	Metadata(path string) (BookMetadata, error)
	// Cover returns the cover image bytes, or nil if the file has none.
	Cover(path string) ([]byte, error)
}

// Registry holds the known handlers and dispatches files to them. The zero
// value is ready to use; a package-level Default instance is provided for
// convenience.
type Registry struct {
	handlers []FormatHandler
}

// Default is the process-wide registry that format packages register into from
// their init functions.
var Default = &Registry{}

// Register adds a handler. Handlers are consulted in registration order, so
// more specific handlers should be registered first.
func (r *Registry) Register(h FormatHandler) {
	r.handlers = append(r.handlers, h)
}

// HandlerFor returns the first registered handler that CanHandle the path.
func (r *Registry) HandlerFor(path string) (FormatHandler, bool) {
	for _, h := range r.handlers {
		if h.CanHandle(path) {
			return h, true
		}
	}
	return nil, false
}

// Handlers returns a copy of the registered handlers, in order.
func (r *Registry) Handlers() []FormatHandler {
	out := make([]FormatHandler, len(r.handlers))
	copy(out, r.handlers)
	return out
}

// HasExt reports whether path ends with the given extension, case-insensitively.
// A small helper shared by handlers' CanHandle implementations.
func HasExt(path, ext string) bool {
	return strings.EqualFold(filepath.Ext(path), ext)
}

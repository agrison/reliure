// Package epub parses EPUB files (container.xml → OPF → Dublin Core, plus
// Calibre and EPUB3 metadata) behind a formats.FormatHandler, and extracts
// cover images.
//
// Robustness is a first-class goal: no input, however malformed, causes a
// panic or a hard failure. Metadata always returns a usable result — falling
// back to the filename as the title — and reports a non-nil error only so the
// caller can log the degradation. This mirrors the import requirement that a
// broken book must still be imported, never block the batch.
package epub

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/agrison/reliure/internal/formats"
)

// maxCoverBytes caps how much we read for a cover image, so a hostile or
// corrupt archive can't exhaust memory.
const maxCoverBytes = 32 << 20 // 32 MiB

// Handler implements formats.FormatHandler for EPUB.
type Handler struct{}

// New returns an EPUB handler.
func New() Handler { return Handler{} }

// Register the handler in the default registry on import, mirroring the
// image/database-driver idiom: `import _ ".../formats/epub"` wires it up.
func init() { formats.Default.Register(New()) }

// Format returns the identifier stored on files.
func (Handler) Format() string { return "epub" }

// CanHandle recognises EPUB containers and common EPUB-derived extensions
// (cheap, per the contract).
func (Handler) CanHandle(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	for _, suffix := range []string{".epub", ".epub.images", ".epub.noimages", ".epub3", ".epub3.images", ".kepub"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// Metadata parses the EPUB at path. It never panics and always returns usable
// metadata: on any structural problem it falls back to the filename title and
// returns a non-nil error describing the degradation (for logging).
func (h Handler) Metadata(path string) (formats.BookMetadata, error) {
	fallback := formats.BookMetadata{Title: fallbackTitle(path), Identifiers: map[string]string{}}

	zr, err := zip.OpenReader(path)
	if err != nil {
		return fallback, fmt.Errorf("epub %q: not a readable archive: %w", filepath.Base(path), err)
	}
	defer zr.Close()

	pkg, _, err := readPackage(&zr.Reader)
	if err != nil {
		return fallback, fmt.Errorf("epub %q: %w", filepath.Base(path), err)
	}

	md := pkg.toMetadata()
	if strings.TrimSpace(md.Title) == "" {
		md.Title = fallback.Title
	}
	if md.Identifiers == nil {
		md.Identifiers = map[string]string{}
	}
	return md, nil
}

// Cover returns the cover image bytes, or nil if the book has none. A parse
// failure is returned as an error (the caller may still import without a cover).
func (h Handler) Cover(path string) ([]byte, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	pkg, opfPath, err := readPackage(&zr.Reader)
	if err != nil {
		return nil, err
	}
	href := pkg.coverHref()
	if href == "" {
		return nil, nil
	}
	return readZipFile(&zr.Reader, resolveHref(opfPath, href))
}

// --- zip / container plumbing ---

type containerXML struct {
	Rootfiles []struct {
		FullPath  string `xml:"full-path,attr"`
		MediaType string `xml:"media-type,attr"`
	} `xml:"rootfiles>rootfile"`
}

// readPackage locates and parses the OPF package document. It reads the OPF path
// from META-INF/container.xml, falling back to scanning for any *.opf entry when
// the container is missing or unusable.
func readPackage(zr *zip.Reader) (*opfPackage, string, error) {
	opfPath := opfPathFromContainer(zr)
	if opfPath == "" {
		opfPath = firstOPF(zr)
	}
	if opfPath == "" {
		return nil, "", fmt.Errorf("no OPF package document found")
	}
	data, err := readZipFile(zr, opfPath)
	if err != nil {
		return nil, opfPath, fmt.Errorf("reading %s: %w", opfPath, err)
	}
	var pkg opfPackage
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return nil, opfPath, fmt.Errorf("parsing %s: %w", opfPath, err)
	}
	return &pkg, opfPath, nil
}

func opfPathFromContainer(zr *zip.Reader) string {
	data, err := readZipFile(zr, "META-INF/container.xml")
	if err != nil {
		return ""
	}
	var c containerXML
	if err := xml.Unmarshal(data, &c); err != nil {
		return ""
	}
	for _, rf := range c.Rootfiles {
		if p := strings.TrimSpace(rf.FullPath); p != "" {
			return path.Clean(p)
		}
	}
	return ""
}

func firstOPF(zr *zip.Reader) string {
	for _, f := range zr.File {
		if strings.EqualFold(extOf(f.Name), ".opf") {
			return f.Name
		}
	}
	return ""
}

// resolveHref resolves a manifest href (relative to the OPF file) to a zip entry
// name, URL-decoding it and stripping any fragment.
func resolveHref(opfPath, href string) string {
	href = stripFragment(href)
	if decoded, err := url.PathUnescape(href); err == nil {
		href = decoded
	}
	return path.Clean(path.Join(path.Dir(opfPath), href))
}

// readZipFile reads a single entry by name. It first tries an exact match, then
// a case-insensitive one (some archives disagree on casing), capping the size.
func readZipFile(zr *zip.Reader, name string) ([]byte, error) {
	name = path.Clean(name)
	f := findEntry(zr, name)
	if f == nil {
		return nil, fmt.Errorf("entry %q not found", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, maxCoverBytes))
}

func findEntry(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if path.Clean(f.Name) == name {
			return f
		}
	}
	for _, f := range zr.File {
		if strings.EqualFold(path.Clean(f.Name), name) {
			return f
		}
	}
	return nil
}

func stripFragment(href string) string {
	if i := strings.IndexAny(href, "#?"); i >= 0 {
		return href[:i]
	}
	return href
}

// fallbackTitle is the file's base name without extension, used when no title
// can be parsed.
func fallbackTitle(p string) string {
	base := filepath.Base(p)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

package epub

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/agrison/reliure/internal/formats"
)

// CreatorWrite is one author to write into the OPF.
type CreatorWrite struct {
	Name   string
	FileAs string // opf:file-as (sort name); omitted when empty
	Role   string // opf:role (MARC); defaults to "aut"
}

// MetaWrite is the set of metadata to write back into an EPUB's OPF. Only these
// fields are managed; the cover reference and the package's unique identifier
// are preserved, other custom metadata is dropped.
type MetaWrite struct {
	Title       string
	TitleSort   string
	Creators    []CreatorWrite
	Language    string
	Description string
	ISBN        string
	Published   string
	Series      string
	SeriesIndex *float64
	Tags        []string
}

var (
	// The metadata element, with or without a namespace prefix.
	metaOpenRe  = regexp.MustCompile(`(?is)<([a-z0-9_]+:)?metadata\b[^>]*>`)
	metaCloseRe = regexp.MustCompile(`(?is)</([a-z0-9_]+:)?metadata>`)
	uniqueIDRe  = regexp.MustCompile(`(?is)\bunique-identifier\s*=\s*"([^"]*)"`)
	coverMetaRe = regexp.MustCompile(`(?is)<meta\b[^>]*\bname\s*=\s*"cover"[^>]*/?>`)
)

// WriteMetadata implements formats.MetadataWriter: it rewrites the EPUB's OPF
// to match the given format-neutral metadata.
func (Handler) WriteMetadata(path string, md formats.BookMetadata) error {
	return WriteOPF(path, metaWriteFromBookMetadata(md))
}

// metaWriteFromBookMetadata adapts the neutral BookMetadata to the OPF writer's
// input.
func metaWriteFromBookMetadata(md formats.BookMetadata) MetaWrite {
	mw := MetaWrite{
		Title:       md.Title,
		TitleSort:   md.TitleSort,
		Language:    md.Language,
		Description: md.Description,
		ISBN:        md.ISBN,
		Published:   md.Published,
		Series:      md.Series,
		SeriesIndex: md.SeriesIndex,
		Tags:        md.Tags,
	}
	for _, c := range md.Contributors {
		mw.Creators = append(mw.Creators, CreatorWrite{Name: c.Name, FileAs: c.SortName, Role: c.Role})
	}
	return mw
}

// WriteOPF rewrites the metadata block of the EPUB at path to match m,
// preserving everything else (manifest, spine, cover reference, unique
// identifier). It writes atomically (temp file + rename). This modifies the
// file in place — callers decide whether that's appropriate (it is an opt-in
// feature; on referenced originals it edits the user's own file).
func WriteOPF(path string, m MetaWrite) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("epub write: open %s: %w", filepath.Base(path), err)
	}
	opfPath := opfPathFromContainer(&zr.Reader)
	if opfPath == "" {
		opfPath = firstOPF(&zr.Reader)
	}
	if opfPath == "" {
		zr.Close()
		return fmt.Errorf("epub write: no OPF in %s", filepath.Base(path))
	}
	rawOPF, err := readZipFile(&zr.Reader, opfPath)
	if err != nil {
		zr.Close()
		return fmt.Errorf("epub write: reading OPF: %w", err)
	}
	newOPF, err := rewriteOPF(rawOPF, m)
	if err != nil {
		zr.Close()
		return err
	}
	if err := rewriteZip(path, &zr.Reader, opfPath, newOPF); err != nil {
		zr.Close()
		return err
	}
	return zr.Close()
}

// rewriteOPF returns the OPF with its metadata inner content replaced. It keeps
// the original <metadata …> opening tag (so its xmlns:dc / xmlns:opf
// declarations remain valid for the elements we emit), and carries over the
// unique-identifier <dc:identifier> and the cover <meta> verbatim.
func rewriteOPF(raw []byte, m MetaWrite) ([]byte, error) {
	src := string(raw)
	open := metaOpenRe.FindStringIndex(src)
	if open == nil {
		return nil, fmt.Errorf("epub write: no <metadata> element")
	}
	closeLoc := metaCloseRe.FindStringIndex(src[open[1]:])
	if closeLoc == nil {
		return nil, fmt.Errorf("epub write: unterminated <metadata> element")
	}
	innerStart := open[1]
	innerEnd := open[1] + closeLoc[0]
	oldInner := src[innerStart:innerEnd]

	var b strings.Builder
	b.WriteString("\n")

	// Emit our ISBN first so it wins on re-parse even when the preserved unique
	// identifier below is itself an ISBN.
	if m.ISBN != "" {
		writeEl(&b, `<dc:identifier opf:scheme="ISBN">`+esc(m.ISBN)+`</dc:identifier>`)
	}
	// Preserve the package's unique identifier element verbatim (keeps the OPF
	// valid: package/@unique-identifier must reference an existing identifier).
	if id := firstSubmatch(uniqueIDRe, src); id != "" {
		if el := findIdentifier(oldInner, id); el != "" {
			b.WriteString("    ")
			b.WriteString(el)
			b.WriteString("\n")
		}
	}

	writeEl(&b, `<dc:title>`+esc(m.Title)+`</dc:title>`)
	for _, c := range m.Creators {
		role := c.Role
		if role == "" {
			role = "aut"
		}
		attrs := ` opf:role="` + esc(role) + `"`
		if c.FileAs != "" {
			attrs += ` opf:file-as="` + esc(c.FileAs) + `"`
		}
		writeEl(&b, `<dc:creator`+attrs+`>`+esc(c.Name)+`</dc:creator>`)
	}
	if m.Language != "" {
		writeEl(&b, `<dc:language>`+esc(m.Language)+`</dc:language>`)
	}
	if m.Description != "" {
		writeEl(&b, `<dc:description>`+esc(m.Description)+`</dc:description>`)
	}
	if m.Published != "" {
		writeEl(&b, `<dc:date>`+esc(m.Published)+`</dc:date>`)
	}
	for _, t := range m.Tags {
		if t = strings.TrimSpace(t); t != "" {
			writeEl(&b, `<dc:subject>`+esc(t)+`</dc:subject>`)
		}
	}
	if m.Series != "" {
		writeMeta(&b, "calibre:series", m.Series)
		if m.SeriesIndex != nil {
			writeMeta(&b, "calibre:series_index", trimFloat(*m.SeriesIndex))
		}
	}
	if m.TitleSort != "" {
		writeMeta(&b, "calibre:title_sort", m.TitleSort)
	}
	// Keep the cover reference so the cover still resolves.
	if cover := coverMetaRe.FindString(oldInner); cover != "" {
		b.WriteString("    ")
		b.WriteString(cover)
		b.WriteString("\n")
	}
	b.WriteString("  ")

	return []byte(src[:innerStart] + b.String() + src[innerEnd:]), nil
}

// rewriteZip streams every entry of zr into a fresh zip at path, replacing the
// target entry's bytes. mimetype is written first and stored, per EPUB rules.
func rewriteZip(path string, zr *zip.Reader, target string, newContent []byte) (err error) {
	tmp := path + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			out.Close()
			os.Remove(tmp)
		}
	}()
	zw := zip.NewWriter(out)

	writeEntry := func(f *zip.File) error {
		method := f.Method
		if f.Name == "mimetype" {
			method = zip.Store
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: method})
		if err != nil {
			return err
		}
		if f.Name == target {
			_, err = w.Write(newContent)
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		_, err = io.Copy(w, rc)
		return err
	}

	// mimetype first if present.
	for _, f := range zr.File {
		if f.Name == "mimetype" {
			if err = writeEntry(f); err != nil {
				return err
			}
			break
		}
	}
	for _, f := range zr.File {
		if f.Name == "mimetype" {
			continue
		}
		if err = writeEntry(f); err != nil {
			return err
		}
	}
	if err = zw.Close(); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// findIdentifier returns the <dc:identifier> element in inner whose id matches,
// verbatim, or "".
func findIdentifier(inner, id string) string {
	re := regexp.MustCompile(`(?is)<([a-z0-9_]+:)?identifier\b[^>]*\bid\s*=\s*"` +
		regexp.QuoteMeta(id) + `"[^>]*>.*?</([a-z0-9_]+:)?identifier>`)
	return re.FindString(inner)
}

func firstSubmatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

func writeEl(b *strings.Builder, el string) {
	b.WriteString("    ")
	b.WriteString(el)
	b.WriteString("\n")
}

func writeMeta(b *strings.Builder, name, content string) {
	writeEl(b, `<meta name="`+esc(name)+`" content="`+esc(content)+`"/>`)
}

func esc(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func trimFloat(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

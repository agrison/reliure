package library

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/agrison/reliure/internal/core"

	// Registers format handlers into formats.Default.
	_ "github.com/agrison/reliure/internal/formats/epub"
	_ "github.com/agrison/reliure/internal/formats/pdf"
)

func newImporter(t *testing.T, merge bool) (*Importer, *core.DB, string) {
	t.Helper()
	db, err := core.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	root := t.TempDir()
	imp := New(db, Config{
		LibraryDir:   filepath.Join(root, "Library"),
		CoverDir:     filepath.Join(root, "covers"),
		ThumbnailMax: 120,
		Workers:      4,
		Merge:        merge,
	})
	return imp, db, root
}

func pngCover(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 60, 90))
	for y := 0; y < 90; y++ {
		for x := 0; x < 60; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 2), 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// makeEPUB writes a valid EPUB with the given metadata (filler varies the bytes,
// hence the SHA-256, between otherwise-identical books). Returns its path.
func makeEPUB(t *testing.T, dir, name, title, author, series, filler string, cover []byte) string {
	t.Helper()
	var meta string
	if series != "" {
		meta = fmt.Sprintf(`<meta name="calibre:series" content="%s"/><meta name="calibre:series_index" content="1"/>`, series)
	}
	coverManifest := ""
	if cover != nil {
		meta += `<meta name="cover" content="cov"/>`
		coverManifest = `<item id="cov" href="cover.png" media-type="image/png"/>`
	}
	opf := fmt.Sprintf(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>%s</dc:title>
    <dc:creator opf:role="aut">%s</dc:creator>
    <dc:language>fr</dc:language>
    %s
  </metadata>
  <manifest><item id="c" href="c.xhtml" media-type="application/xhtml+xml"/>%s</manifest>
  <spine><itemref idref="c"/></spine>
</package>`, title, author, meta, coverManifest)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	mt, _ := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	mt.Write([]byte("application/epub+zip"))
	write := func(n, c string) {
		w, _ := zw.Create(n)
		w.Write([]byte(c))
	}
	write("META-INF/container.xml", `<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
<rootfiles><rootfile full-path="content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)
	write("content.opf", opf)
	write("c.xhtml", "<html><body>"+filler+"</body></html>")
	if cover != nil {
		w, _ := zw.Create("cover.png")
		w.Write(cover)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	if dir == "" {
		dir = t.TempDir()
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func makePDF(t *testing.T, dir, name, title, author string) string {
	t.Helper()
	if dir == "" {
		dir = t.TempDir()
	}
	path := filepath.Join(dir, name)
	body := fmt.Sprintf("%%PDF-1.7\n1 0 obj\n<< /Title (%s) /Author (%s) >>\nendobj\n%%%%EOF\n", title, author)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImportCreatesBookCopiesFileAndCover(t *testing.T) {
	imp, db, _ := newImporter(t, true)
	src := makeEPUB(t, "", "book.epub", "Dune", "Frank Herbert", "Dune", "x", pngCover(t))

	sum, err := imp.Import(context.Background(), []string{src}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Imported != 1 || sum.Total != 1 {
		t.Fatalf("summary = %+v", sum)
	}

	books, _ := db.Books.List(0, 0)
	if len(books) != 1 {
		t.Fatalf("book count = %d", len(books))
	}
	b := books[0]
	if b.Title != "Dune" {
		t.Errorf("title = %q", b.Title)
	}
	if len(b.Authors) != 1 || b.Authors[0].Author.Name != "Frank Herbert" {
		t.Errorf("authors = %+v", b.Authors)
	}
	if b.Series == nil || b.Series.Name != "Dune" {
		t.Errorf("series = %+v", b.Series)
	}
	if len(b.Files) != 1 {
		t.Fatalf("files = %+v", b.Files)
	}

	// File copied to LibraryDir/Author/Title/Title.epub.
	want := filepath.Join(imp.cfg.LibraryDir, "Frank Herbert", "Dune", "Dune.epub")
	if b.Files[0].Path != want {
		t.Errorf("copied path = %q, want %q", b.Files[0].Path, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("copied file missing: %v", err)
	}

	// Cover thumbnail cached and referenced.
	if b.CoverPath == "" {
		t.Error("cover path not set")
	} else if _, err := os.Stat(filepath.Join(imp.cfg.CoverDir, b.CoverPath)); err != nil {
		t.Errorf("thumbnail missing: %v", err)
	}
}

func TestImportPDFCreatesEditableBook(t *testing.T) {
	imp, db, _ := newImporter(t, true)
	src := makePDF(t, "", "paper.pdf", "Designing Data-Intensive Applications", "Martin Kleppmann")

	sum, err := imp.Import(context.Background(), []string{src}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Imported != 1 || sum.Total != 1 {
		t.Fatalf("summary = %+v", sum)
	}

	books, err := db.Books.List(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 1 {
		t.Fatalf("book count = %d", len(books))
	}
	b := books[0]
	if b.Title != "Designing Data-Intensive Applications" {
		t.Fatalf("title = %q", b.Title)
	}
	if len(b.Authors) != 1 || b.Authors[0].Author.Name != "Martin Kleppmann" {
		t.Fatalf("authors = %+v", b.Authors)
	}
	if len(b.Files) != 1 || b.Files[0].Format != "pdf" {
		t.Fatalf("files = %+v", b.Files)
	}
	want := filepath.Join(imp.cfg.LibraryDir, "Martin Kleppmann", "Designing Data-Intensive Applications", "Designing Data-Intensive Applications.pdf")
	if b.Files[0].Path != want {
		t.Fatalf("copied path = %q, want %q", b.Files[0].Path, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("copied PDF missing: %v", err)
	}

	b.Title = "DDIA"
	b.Tags = []core.Tag{{Name: "architecture"}}
	if err := db.Books.Update(b); err != nil {
		t.Fatalf("update PDF metadata book: %v", err)
	}
	updated, err := db.Books.ByID(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "DDIA" || len(updated.Tags) != 1 || updated.Tags[0].Name != "architecture" {
		t.Fatalf("updated book = %+v", updated)
	}
}

func TestImportReferenceModeIndexesInPlace(t *testing.T) {
	db, err := core.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	root := t.TempDir()
	imp := New(db, Config{
		Mode:     ModeReference,
		CoverDir: filepath.Join(root, "covers"),
		Workers:  2,
		Merge:    true,
	})

	srcDir := mustMkdir(t, filepath.Join(root, "MyBooks"))
	src := makeEPUB(t, srcDir, "orig.epub", "Solaris", "Stanislas Lem", "", "x", pngCover(t))

	sum, err := imp.Import(context.Background(), []string{src}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Imported != 1 {
		t.Fatalf("summary = %+v", sum)
	}

	books, _ := db.Books.List(0, 0)
	if len(books) != 1 || len(books[0].Files) != 1 {
		t.Fatalf("unexpected book/files: %+v", books)
	}
	// The stored path is the ORIGINAL file, untouched and still present.
	if got := books[0].Files[0].Path; got != src {
		t.Errorf("reference mode stored %q, want original %q", got, src)
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("original file must remain in place: %v", err)
	}
	// A cover thumbnail is still cached (Reliure owns that, in both modes).
	if books[0].CoverPath == "" {
		t.Error("cover should still be cached in reference mode")
	}
	// Re-importing the same in-place file is a duplicate (SHA match), no error.
	sum2, err := imp.Import(context.Background(), []string{src}, nil)
	if err != nil || sum2.Duplicates != 1 {
		t.Errorf("re-import summary = %+v (err %v)", sum2, err)
	}
}

func TestImportDeduplicatesIdenticalFile(t *testing.T) {
	imp, db, _ := newImporter(t, true)
	src := makeEPUB(t, "", "book.epub", "1984", "George Orwell", "", "same", pngCover(t))

	if _, err := imp.Import(context.Background(), []string{src}, nil); err != nil {
		t.Fatal(err)
	}
	sum, err := imp.Import(context.Background(), []string{src}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Duplicates != 1 || sum.Imported != 0 {
		t.Fatalf("second import summary = %+v", sum)
	}
	if n, _ := db.Books.Count(); n != 1 {
		t.Fatalf("book count = %d, want 1", n)
	}
}

func TestImportAttachesSameTitleAuthor(t *testing.T) {
	imp, db, _ := newImporter(t, true)
	a := makeEPUB(t, "", "a.epub", "Neuromancer", "William Gibson", "", "content-A", nil)
	b := makeEPUB(t, "", "b.epub", "Neuromancer", "William Gibson", "", "content-B-different", nil)

	sum, err := imp.Import(context.Background(), []string{a, b}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Imported != 1 || sum.Attached != 1 {
		t.Fatalf("summary = %+v", sum)
	}
	if n, _ := db.Books.Count(); n != 1 {
		t.Fatalf("book count = %d, want 1 (attached)", n)
	}
	books, _ := db.Books.List(0, 0)
	if len(books[0].Files) != 2 {
		t.Errorf("file count = %d, want 2 formats on one book", len(books[0].Files))
	}
}

func TestImportMergeDisabledCreatesTwo(t *testing.T) {
	imp, db, _ := newImporter(t, false)
	a := makeEPUB(t, "", "a.epub", "Hyperion", "Dan Simmons", "", "A", nil)
	b := makeEPUB(t, "", "b.epub", "Hyperion", "Dan Simmons", "", "B-diff", nil)

	sum, err := imp.Import(context.Background(), []string{a, b}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Imported != 2 {
		t.Fatalf("summary = %+v", sum)
	}
	if n, _ := db.Books.Count(); n != 2 {
		t.Fatalf("book count = %d, want 2", n)
	}
	// Collision on same Author/Title dir must not clobber: two distinct files.
	books, _ := db.Books.List(0, 0)
	p0, p1 := books[0].Files[0].Path, books[1].Files[0].Path
	if p0 == p1 {
		t.Errorf("distinct books share a file path: %q", p0)
	}
}

func TestImportConcurrentReportsProgress(t *testing.T) {
	imp, db, _ := newImporter(t, true)
	const n = 25
	var paths []string
	for i := 0; i < n; i++ {
		paths = append(paths, makeEPUB(t, "",
			fmt.Sprintf("b%02d.epub", i),
			fmt.Sprintf("Title %02d", i),
			fmt.Sprintf("Author %02d", i), "", "f", nil))
	}

	var dones []int
	sum, err := imp.Import(context.Background(), paths, func(p Progress) {
		if p.Total != n {
			t.Errorf("progress total = %d", p.Total)
		}
		dones = append(dones, p.Done)
	})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Imported != n {
		t.Fatalf("imported = %d, want %d", sum.Imported, n)
	}
	if count, _ := db.Books.Count(); count != n {
		t.Fatalf("book count = %d", count)
	}
	// Progress fires once per file, and Done runs 1..n exactly.
	if len(dones) != n {
		t.Fatalf("progress callbacks = %d, want %d", len(dones), n)
	}
	sort.Ints(dones)
	for i, d := range dones {
		if d != i+1 {
			t.Fatalf("Done sequence broken at %d: got %d", i, d)
		}
	}
}

func TestImportDegradedAndUnsupported(t *testing.T) {
	imp, _, root := newImporter(t, true)

	// A .epub that is not a valid archive → degraded import, filename title.
	broken := filepath.Join(root, "Broken Book.epub")
	if err := os.WriteFile(broken, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	// An unsupported extension passed directly → failure.
	unsup := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(unsup, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	var outcomes = map[string]Outcome{}
	sum, err := imp.Import(context.Background(), []string{broken, unsup}, func(p Progress) {
		outcomes[filepath.Base(p.Path)] = p.Outcome
		if p.Outcome == OutcomeImported && p.Title != "Broken Book" {
			t.Errorf("degraded title = %q, want filename fallback", p.Title)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcomes["Broken Book.epub"] != OutcomeImported {
		t.Errorf("broken epub outcome = %v, want imported (degraded)", outcomes["Broken Book.epub"])
	}
	if outcomes["notes.txt"] != OutcomeFailed {
		t.Errorf("unsupported outcome = %v, want failed", outcomes["notes.txt"])
	}
	if sum.Imported != 1 || sum.Failed != 1 {
		t.Errorf("summary = %+v", sum)
	}
}

func TestImportContextCancelled(t *testing.T) {
	imp, _, _ := newImporter(t, true)
	src := makeEPUB(t, "", "b.epub", "T", "A", "", "f", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sum, err := imp.Import(ctx, []string{src}, nil)
	if err == nil {
		t.Error("expected context error")
	}
	if sum.Total != 0 {
		t.Errorf("nothing should be imported after cancel, summary = %+v", sum)
	}
}

func TestScanRecursive(t *testing.T) {
	imp, _, root := newImporter(t, true)
	src := filepath.Join(root, "src")
	makeEPUB(t, mustMkdir(t, src), "a.epub", "A", "X", "", "f", nil)
	makePDF(t, mustMkdir(t, filepath.Join(src, "sub")), "b.pdf", "B", "Y")
	// Noise: a non-ebook and a hidden directory with an epub in it.
	os.WriteFile(filepath.Join(src, "readme.txt"), []byte("x"), 0o644)
	makeEPUB(t, mustMkdir(t, filepath.Join(src, ".hidden")), "c.epub", "C", "Z", "", "f", nil)

	got, err := imp.Scan(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("scan found %d files, want 2: %v", len(got), got)
	}
	for _, p := range got {
		if strings.Contains(p, ".hidden") || strings.HasSuffix(p, ".txt") {
			t.Errorf("unexpected scan result: %s", p)
		}
	}
}

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"Normal Title":       "Normal Title",
		"a/b:c*d?":           "a_b_c_d_",
		"  trailing dots.. ": "trailing dots",
		"":                   "_",
		"   ":                "_",
	}
	for in, want := range cases {
		if got := sanitize(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
	// Length cap on rune boundaries.
	long := strings.Repeat("é", 300)
	if got := sanitize(long); len([]rune(got)) > maxNameLen {
		t.Errorf("sanitize did not cap length: %d runes", len([]rune(got)))
	}
}

func mustMkdir(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

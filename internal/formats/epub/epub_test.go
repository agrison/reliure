package epub

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/agrison/reliure/internal/formats"
)

// buildEPUB writes a minimal EPUB zip to a temp file and returns its path.
// When withContainer is false, META-INF/container.xml is omitted (to exercise
// the OPF-scan fallback). extra maps zip entry names to their bytes.
func buildEPUB(t *testing.T, opfPath, opfXML string, extra map[string]string, withContainer bool) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// A real EPUB stores an uncompressed "mimetype" entry first.
	mt, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	mt.Write([]byte("application/epub+zip"))

	write := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	if withContainer {
		write("META-INF/container.xml", `<?xml version="1.0"?>
<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0">
  <rootfiles><rootfile full-path="`+opfPath+`" media-type="application/oebps-package+xml"/></rootfiles>
</container>`)
	}
	if opfXML != "" {
		write(opfPath, opfXML)
	}
	for name, content := range extra {
		write(name, content)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "book.epub")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const epub2OPF = `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>La Main gauche de la nuit</dc:title>
    <dc:creator opf:role="aut" opf:file-as="Le Guin, Ursula K.">Ursula K. Le Guin</dc:creator>
    <dc:contributor opf:role="trl" opf:file-as="Bailhache, Jean">Jean Bailhache</dc:contributor>
    <dc:contributor opf:role="bkp">calibre (7.0)</dc:contributor>
    <dc:language>fr</dc:language>
    <dc:identifier id="uid" opf:scheme="ISBN">978-2-266-11209-1</dc:identifier>
    <dc:identifier opf:scheme="calibre">1234</dc:identifier>
    <dc:date>1971-01-01</dc:date>
    <dc:publisher>Robert Laffont</dc:publisher>
    <dc:description>Roman de science-fiction.</dc:description>
    <dc:subject>science-fiction</dc:subject>
    <dc:subject>classique</dc:subject>
    <meta name="calibre:series" content="Cycle de l'Ekumen"/>
    <meta name="calibre:series_index" content="4.0"/>
    <meta name="calibre:title_sort" content="Main gauche de la nuit, La"/>
    <meta name="cover" content="cover-img"/>
  </metadata>
  <manifest>
    <item id="cover-img" href="images/cover.jpg" media-type="image/jpeg"/>
    <item id="content" href="content.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="content"/></spine>
</package>`

func TestMetadataEPUB2Calibre(t *testing.T) {
	path := buildEPUB(t, "OEBPS/content.opf", epub2OPF,
		map[string]string{"OEBPS/images/cover.jpg": "JPEGBYTES"}, true)

	md, err := New().Metadata(path)
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if md.Title != "La Main gauche de la nuit" {
		t.Errorf("title = %q", md.Title)
	}
	if md.TitleSort != "Main gauche de la nuit, La" {
		t.Errorf("title sort = %q", md.TitleSort)
	}
	if md.Language != "fr" || md.Publisher != "Robert Laffont" || md.Published != "1971-01-01" {
		t.Errorf("scalar fields off: %+v", md)
	}
	if md.Description != "Roman de science-fiction." {
		t.Errorf("description = %q", md.Description)
	}
	if md.Series != "Cycle de l'Ekumen" || md.SeriesIndex == nil || *md.SeriesIndex != 4 {
		t.Errorf("series = %q idx = %v", md.Series, md.SeriesIndex)
	}
	// Two real contributors; the "calibre (7.0)" bkp entry must be dropped.
	if len(md.Contributors) != 2 {
		t.Fatalf("contributors = %d (%+v)", len(md.Contributors), md.Contributors)
	}
	if c := md.Contributors[0]; c.Name != "Ursula K. Le Guin" || c.Role != "aut" || c.SortName != "Le Guin, Ursula K." {
		t.Errorf("author = %+v", c)
	}
	if c := md.Contributors[1]; c.Name != "Jean Bailhache" || c.Role != "trl" {
		t.Errorf("translator = %+v", c)
	}
	if md.ISBN != "9782266112091" {
		t.Errorf("isbn = %q (hyphens should be stripped)", md.ISBN)
	}
	if md.Identifiers["calibre"] != "1234" {
		t.Errorf("calibre id = %q", md.Identifiers["calibre"])
	}
	if len(md.Tags) != 2 || md.Tags[0] != "science-fiction" || md.Tags[1] != "classique" {
		t.Errorf("tags = %v", md.Tags)
	}
}

const epub3OPF = `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title id="t1">The Left Hand of Darkness</dc:title>
    <meta refines="#t1" property="file-as">Left Hand of Darkness, The</meta>
    <dc:creator id="cre1">Ursula K. Le Guin</dc:creator>
    <meta refines="#cre1" property="role" scheme="marc:relators">aut</meta>
    <meta refines="#cre1" property="file-as">Le Guin, Ursula K.</meta>
    <dc:language>en</dc:language>
    <dc:identifier id="uid">urn:uuid:12345678-1234-1234-1234-123456789abc</dc:identifier>
    <meta property="belongs-to-collection" id="col1">Hainish Cycle</meta>
    <meta refines="#col1" property="collection-type">series</meta>
    <meta refines="#col1" property="group-position">4</meta>
  </metadata>
  <manifest>
    <item id="cover-img" href="cover.png" media-type="image/png" properties="cover-image"/>
    <item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="c1"/></spine>
</package>`

func TestMetadataEPUB3Refines(t *testing.T) {
	path := buildEPUB(t, "content.opf", epub3OPF,
		map[string]string{"cover.png": "PNGBYTES"}, true)

	md, err := New().Metadata(path)
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if md.Title != "The Left Hand of Darkness" {
		t.Errorf("title = %q", md.Title)
	}
	if md.TitleSort != "Left Hand of Darkness, The" {
		t.Errorf("title sort (from title file-as refine) = %q", md.TitleSort)
	}
	if len(md.Contributors) != 1 {
		t.Fatalf("contributors = %+v", md.Contributors)
	}
	if c := md.Contributors[0]; c.Role != "aut" || c.SortName != "Le Guin, Ursula K." {
		t.Errorf("author refines unresolved: %+v", c)
	}
	if md.Series != "Hainish Cycle" || md.SeriesIndex == nil || *md.SeriesIndex != 4 {
		t.Errorf("collection→series failed: %q %v", md.Series, md.SeriesIndex)
	}
	if md.Identifiers["uuid"] != "12345678-1234-1234-1234-123456789abc" {
		t.Errorf("uuid = %q", md.Identifiers["uuid"])
	}
}

// coverOPF builds a tiny OPF exercising a specific cover-resolution path.
func coverOPF(metaCover, manifest string) string {
	return `<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>T</dc:title>` + metaCover + `</metadata>
  <manifest>` + manifest + `</manifest></package>`
}

func TestCoverResolution(t *testing.T) {
	cases := []struct {
		name     string
		metaCov  string
		manifest string
		files    map[string]string
		want     string // expected cover bytes, "" means nil
	}{
		{
			name:     "epub2 meta name=cover",
			metaCov:  `<meta name="cover" content="cid"/>`,
			manifest: `<item id="cid" href="img/c.jpg" media-type="image/jpeg"/>`,
			files:    map[string]string{"img/c.jpg": "A"},
			want:     "A",
		},
		{
			name:     "epub3 cover-image property",
			manifest: `<item id="x" href="c.png" media-type="image/png" properties="cover-image"/>`,
			files:    map[string]string{"c.png": "B"},
			want:     "B",
		},
		{
			name:     "heuristic name contains cover",
			manifest: `<item id="i1" href="assets/mycover.jpeg" media-type="image/jpeg"/>`,
			files:    map[string]string{"assets/mycover.jpeg": "C"},
			want:     "C",
		},
		{
			name:     "first image fallback",
			manifest: `<item id="p" href="p1.xhtml" media-type="application/xhtml+xml"/><item id="i" href="pic.jpg" media-type="image/jpeg"/>`,
			files:    map[string]string{"pic.jpg": "D"},
			want:     "D",
		},
		{
			name:     "no image at all",
			manifest: `<item id="p" href="p1.xhtml" media-type="application/xhtml+xml"/>`,
			want:     "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opf := coverOPF(c.metaCov, c.manifest)
			path := buildEPUB(t, "content.opf", opf, c.files, true)
			got, err := New().Cover(path)
			if err != nil {
				t.Fatalf("Cover: %v", err)
			}
			if string(got) != c.want {
				t.Errorf("cover bytes = %q, want %q", got, c.want)
			}
		})
	}
}

func TestCoverResolvesRelativeToOPF(t *testing.T) {
	// OPF lives in OEBPS/, cover href is relative to it.
	opf := coverOPF(`<meta name="cover" content="cid"/>`,
		`<item id="cid" href="images/cover.jpg" media-type="image/jpeg"/>`)
	path := buildEPUB(t, "OEBPS/content.opf", opf,
		map[string]string{"OEBPS/images/cover.jpg": "COVER"}, true)
	got, err := New().Cover(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "COVER" {
		t.Errorf("relative href resolution failed: %q", got)
	}
}

func TestCoverURLEncodedHref(t *testing.T) {
	opf := coverOPF(`<meta name="cover" content="cid"/>`,
		`<item id="cid" href="img/cover%20art.jpg" media-type="image/jpeg"/>`)
	path := buildEPUB(t, "content.opf", opf,
		map[string]string{"img/cover art.jpg": "SPACED"}, true)
	got, err := New().Cover(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "SPACED" {
		t.Errorf("url-encoded href not resolved: %q", got)
	}
}

// --- tolerance to malformed input (must never panic or hard-fail import) ---

func TestToleranceNotAnArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Réellement Cassé.epub")
	if err := os.WriteFile(path, []byte("this is not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	md, err := New().Metadata(path)
	if err == nil {
		t.Error("expected a (loggable) error for a non-archive")
	}
	if md.Title != "Réellement Cassé" {
		t.Errorf("fallback title = %q, want the file base name", md.Title)
	}
}

func TestToleranceNoContainerButOPF(t *testing.T) {
	// No META-INF/container.xml: the scan fallback should still find the OPF.
	path := buildEPUB(t, "book.opf", epub2OPF, nil, false)
	md, err := New().Metadata(path)
	if err != nil {
		t.Fatalf("expected recovery via OPF scan, got %v", err)
	}
	if md.Title != "La Main gauche de la nuit" {
		t.Errorf("title = %q", md.Title)
	}
}

func TestToleranceNoOPF(t *testing.T) {
	path := buildEPUB(t, "", "", map[string]string{"random.txt": "x"}, false)
	md, err := New().Metadata(path)
	if err == nil {
		t.Error("expected error when no OPF is present")
	}
	if md.Title != "book" {
		t.Errorf("fallback title = %q", md.Title)
	}
}

func TestToleranceBrokenOPFXML(t *testing.T) {
	path := buildEPUB(t, "content.opf", `<package><metadata><dc:title>Oops</broken>`, nil, true)
	md, err := New().Metadata(path)
	if err == nil {
		t.Error("expected XML parse error")
	}
	if md.Title != "book" {
		t.Errorf("fallback title = %q", md.Title)
	}
}

func TestToleranceEmptyTitle(t *testing.T) {
	opf := `<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title></dc:title>
    <dc:creator>Some Author</dc:creator>
    <dc:language>fr</dc:language>
  </metadata><manifest/></package>`
	path := buildEPUB(t, "content.opf", opf, nil, true)
	md, err := New().Metadata(path)
	if err != nil {
		t.Fatalf("empty title should not be an error: %v", err)
	}
	if md.Title != "book" {
		t.Errorf("title should fall back to filename, got %q", md.Title)
	}
	if len(md.Contributors) != 1 || md.Contributors[0].Name != "Some Author" {
		t.Errorf("other fields should still parse: %+v", md.Contributors)
	}
}

func TestCanHandleAndFormat(t *testing.T) {
	h := New()
	if h.Format() != "epub" {
		t.Errorf("format = %q", h.Format())
	}
	cases := map[string]bool{
		"/x/Book.EPUB":          true,
		"/x/book.epub.images":   true,
		"/x/book.epub.noimages": true,
		"/x/book.epub3":         true,
		"/x/book.epub3.images":  true,
		"/x/book.kepub":         true,
		"/x/book.kepub.epub":    true,
		"/x/book.pdf":           false,
		"/x/book.images":        false,
	}
	for path, want := range cases {
		if got := h.CanHandle(path); got != want {
			t.Errorf("CanHandle(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestRegisteredInDefaultRegistry(t *testing.T) {
	h, ok := formats.Default.HandlerFor("/some/Book.epub")
	if !ok || h.Format() != "epub" {
		t.Fatalf("epub handler not registered in formats.Default (ok=%v)", ok)
	}
}

// TestCorpusRealEPUBs smoke-tests any real .epub files dropped into testdata/:
// they must parse without panicking and yield a non-empty title (possibly the
// filename fallback). This lets the maintainer grow a real-world corpus over
// time; it is a no-op when the directory is empty.
func TestCorpusRealEPUBs(t *testing.T) {
	matches, _ := filepath.Glob("testdata/*.epub")
	if len(matches) == 0 {
		t.Skip("no real EPUBs in testdata/ (drop some in to exercise the parser)")
	}
	for _, path := range matches {
		t.Run(filepath.Base(path), func(t *testing.T) {
			md, err := New().Metadata(path)
			if err != nil {
				t.Logf("degraded parse (still usable): %v", err)
			}
			if md.Title == "" {
				t.Errorf("title empty even after fallback for %s", path)
			}
			// Cover is optional but must not panic.
			if _, err := New().Cover(path); err != nil {
				t.Logf("no cover / cover error: %v", err)
			}
		})
	}
}

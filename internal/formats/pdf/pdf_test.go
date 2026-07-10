package pdf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agrison/reliure/internal/formats"
)

func TestMetadataReadsPDFInfo(t *testing.T) {
	path := writePDF(t, "book.pdf", `<<
/Title (The \(Portable\) Book)
/Author (Alice Example)
/Subject (A compact subject)
/Keywords (fiction, reference; fiction)
/CreationDate (D:20240315120000+01'00')
>>`)

	md, err := New().Metadata(path)
	if err != nil {
		t.Fatal(err)
	}
	if md.Title != "The (Portable) Book" {
		t.Fatalf("title = %q", md.Title)
	}
	if len(md.Contributors) != 1 || md.Contributors[0].Name != "Alice Example" {
		t.Fatalf("contributors = %#v", md.Contributors)
	}
	if md.Description != "A compact subject" {
		t.Fatalf("description = %q", md.Description)
	}
	if got := md.Tags; len(got) != 2 || got[0] != "fiction" || got[1] != "reference" {
		t.Fatalf("tags = %#v", got)
	}
	if md.Published != "2024-03-15" {
		t.Fatalf("published = %q", md.Published)
	}
}

func TestMetadataFallsBackToFilename(t *testing.T) {
	path := writePDF(t, "No Metadata.pdf", `%PDF-1.7
1 0 obj
<< /Type /Catalog >>
endobj`)
	md, err := New().Metadata(path)
	if err != nil {
		t.Fatal(err)
	}
	if md.Title != "No Metadata" {
		t.Fatalf("title = %q", md.Title)
	}
}

func TestMetadataReadsUTF16HexString(t *testing.T) {
	path := writePDF(t, "book.pdf", `<< /Title <FEFF004C0065002000540069007400720065> >>`)
	md, err := New().Metadata(path)
	if err != nil {
		t.Fatal(err)
	}
	if md.Title != "Le Titre" {
		t.Fatalf("title = %q", md.Title)
	}
}

func TestHandlerRegistered(t *testing.T) {
	h, ok := formats.Default.HandlerFor("/some/Book.PDF")
	if !ok || h.Format() != "pdf" {
		t.Fatalf("pdf handler not registered in formats.Default (ok=%v)", ok)
	}
}

func writePDF(t *testing.T, name, info string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	body := "%PDF-1.7\n1 0 obj\n" + info + "\nendobj\n%%EOF\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

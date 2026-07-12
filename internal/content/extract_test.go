package content

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEPUBExtractsVisibleText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.epub")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("chapter.xhtml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(`<html><body><h1>Titre</h1><p>Un passage vraiment unique.</p><script>cache</script></body></html>`)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	text, err := EPUB(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "passage vraiment unique") {
		t.Fatalf("extracted text = %q", text)
	}
	if strings.Contains(text, "cache") {
		t.Fatalf("script content should be ignored: %q", text)
	}
}

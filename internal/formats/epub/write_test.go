package epub

import (
	"os"
	"testing"
)

func TestWriteMetadataRoundTrip(t *testing.T) {
	// epub2OPF has a cover meta (id "cover-img") and a unique-identifier "uid".
	path := buildEPUB(t, "OEBPS/content.opf", epub2OPF,
		map[string]string{"OEBPS/images/cover.jpg": "JPEGDATA"}, true)

	idx := 2.0
	err := WriteOPF(path, MetaWrite{
		Title:       "Nouveau Titre",
		TitleSort:   "Titre, Nouveau",
		Creators:    []CreatorWrite{{Name: "Alice Auteur", FileAs: "Auteur, Alice", Role: "aut"}},
		Language:    "en",
		Description: "Nouvelle description.",
		ISBN:        "9781234567890",
		Published:   "2020",
		Series:      "Ma Série",
		SeriesIndex: &idx,
		Tags:        []string{"tag-a", "tag-b"},
	})
	if err != nil {
		t.Fatalf("WriteMetadata: %v", err)
	}

	// Re-parse the file from disk: the new values must be there.
	md, err := New().Metadata(path)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if md.Title != "Nouveau Titre" {
		t.Errorf("title = %q", md.Title)
	}
	if md.TitleSort != "Titre, Nouveau" {
		t.Errorf("title sort = %q", md.TitleSort)
	}
	if len(md.Contributors) != 1 || md.Contributors[0].Name != "Alice Auteur" ||
		md.Contributors[0].SortName != "Auteur, Alice" || md.Contributors[0].Role != "aut" {
		t.Errorf("creator = %+v", md.Contributors)
	}
	if md.Language != "en" || md.Description != "Nouvelle description." || md.Published != "2020" {
		t.Errorf("scalars off: %+v", md)
	}
	if md.ISBN != "9781234567890" {
		t.Errorf("isbn = %q", md.ISBN)
	}
	if md.Series != "Ma Série" || md.SeriesIndex == nil || *md.SeriesIndex != 2 {
		t.Errorf("series = %q idx = %v", md.Series, md.SeriesIndex)
	}
	if len(md.Tags) != 2 || md.Tags[0] != "tag-a" || md.Tags[1] != "tag-b" {
		t.Errorf("tags = %v", md.Tags)
	}

	// The cover reference must survive: Cover still resolves to the image bytes.
	cover, err := New().Cover(path)
	if err != nil {
		t.Fatalf("cover after write: %v", err)
	}
	if string(cover) != "JPEGDATA" {
		t.Errorf("cover bytes = %q, want JPEGDATA (cover ref lost?)", cover)
	}
}

func TestWriteMetadataEscapesSpecialChars(t *testing.T) {
	path := buildEPUB(t, "content.opf", epub2OPF, nil, true)
	if err := WriteOPF(path, MetaWrite{
		Title:    `Tom & Jerry <"guillemets">`,
		Creators: []CreatorWrite{{Name: "A & B"}},
	}); err != nil {
		t.Fatal(err)
	}
	md, err := New().Metadata(path)
	if err != nil {
		t.Fatalf("reparse after escaping: %v", err)
	}
	if md.Title != `Tom & Jerry <"guillemets">` {
		t.Errorf("title not preserved through escaping: %q", md.Title)
	}
	if len(md.Contributors) != 1 || md.Contributors[0].Name != "A & B" {
		t.Errorf("creator not preserved: %+v", md.Contributors)
	}
}

func TestWriteMetadataAtomicOnBadFile(t *testing.T) {
	// A non-EPUB must fail cleanly and leave no temp file behind.
	path := os.TempDir() + "/not-an-epub.epub"
	os.WriteFile(path, []byte("garbage"), 0o644)
	defer os.Remove(path)
	if err := WriteOPF(path, MetaWrite{Title: "X"}); err == nil {
		t.Error("expected error writing to a non-archive")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file should not be left behind")
	}
}

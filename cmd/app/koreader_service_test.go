package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agrison/reliure/internal/core"
	"github.com/agrison/reliure/internal/koreader"
	"github.com/agrison/reliure/internal/settings"
)

const koSidecar = `return {
    ["percent_finished"] = 0.5,
    ["doc_props"] = { ["title"] = "Match Me", ["authors"] = "Some Author" },
    ["summary"] = { ["status"] = "reading", ["modified"] = "2026-07-10" },
    ["annotations"] = {
        [1] = { ["text"] = "a highlight", ["chapter"] = "Ch 1", ["datetime"] = "2026-07-09 10:00:00" },
        [2] = { ["text"] = "another", ["note"] = "note!", ["datetime"] = "2026-07-09 11:00:00" },
    },
}`

func newKoreaderService(t *testing.T) (*KOReaderService, *core.DB) {
	t.Helper()
	db, err := core.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store, err := settings.Open(filepath.Join(t.TempDir(), "settings.json"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &KOReaderService{db: db, store: store}, db
}

func TestKoreaderSyncMatchesAndPersists(t *testing.T) {
	svc, db := newKoreaderService(t)

	// A library book that should match the sidecar by title + author.
	b := &core.Book{Title: "Match Me", Authors: []core.Contribution{{Author: core.Author{Name: "Some Author"}, Role: "aut"}}}
	if err := db.Books.Create(b); err != nil {
		t.Fatal(err)
	}
	// An unrelated book, to prove only the right one is touched.
	other := &core.Book{Title: "Different", Authors: []core.Contribution{{Author: core.Author{Name: "Nobody"}, Role: "aut"}}}
	if err := db.Books.Create(other); err != nil {
		t.Fatal(err)
	}

	// A KOReader library folder with one matching sidecar and one orphan.
	root := t.TempDir()
	writeSidecar(t, filepath.Join(root, "Match Me.sdr", "metadata.epub.lua"), koSidecar)
	writeSidecar(t, filepath.Join(root, "Orphan.sdr", "metadata.epub.lua"),
		`return { ["doc_props"] = { ["title"] = "Not In Library" }, ["percent_finished"] = 0.1 }`)

	res, err := svc.sync(root)
	if err != nil {
		t.Fatal(err)
	}
	if res.Scanned != 2 {
		t.Errorf("scanned = %d, want 2", res.Scanned)
	}
	if res.Matched != 1 {
		t.Errorf("matched = %d, want 1", res.Matched)
	}
	if res.Unmatched != 1 {
		t.Errorf("unmatched = %d, want 1", res.Unmatched)
	}
	if res.Annotations != 2 {
		t.Errorf("annotations = %d, want 2", res.Annotations)
	}

	st, ok, err := db.Reading.State(b.ID)
	if err != nil || !ok {
		t.Fatalf("reading state: ok=%v err=%v", ok, err)
	}
	if st.Percent != 0.5 || st.Status != "reading" {
		t.Errorf("state = %+v", st)
	}
	if _, ok, _ := db.Reading.State(other.ID); ok {
		t.Error("unrelated book must not get a reading state")
	}

	anns, _ := db.Reading.Annotations(b.ID)
	if len(anns) != 2 {
		t.Fatalf("got %d annotations, want 2", len(anns))
	}
}

func TestKoreaderMatchIndex(t *testing.T) {
	books := []*core.Book{
		{ID: 1, Title: "Le Livre", Authors: []core.Contribution{{Author: core.Author{Name: "Jean Dupont"}}}, Files: []core.File{{Path: "/lib/le-livre.epub"}}},
		{ID: 2, Title: "Unique Title", Authors: []core.Contribution{{Author: core.Author{Name: "Alice"}}}},
	}
	idx := buildMatchIndex(books)

	// Title + author match, case/space tolerant.
	if id, ok := idx.match(koSidecarStub("le  livre", "JEAN DUPONT", "")); !ok || id != 1 {
		t.Errorf("title+author match failed: id=%d ok=%v", id, ok)
	}
	// Filename fallback.
	if id, ok := idx.match(koSidecarStub("wrong", "wrong", "le-livre.epub")); !ok || id != 1 {
		t.Errorf("basename match failed: id=%d ok=%v", id, ok)
	}
	// Unique-title fallback (author differs).
	if id, ok := idx.match(koSidecarStub("unique title", "someone else", "")); !ok || id != 2 {
		t.Errorf("unique-title match failed: id=%d ok=%v", id, ok)
	}
}

func koSidecarStub(title, author, basename string) *koreader.Sidecar {
	var authors []string
	if author != "" {
		authors = []string{author}
	}
	return &koreader.Sidecar{Title: title, Authors: authors, DocBasename: basename}
}

func TestSidecarLpaths(t *testing.T) {
	got := sidecarLpaths("Robin Hobb/Assassin/01 L'apprenti.epub", "epub")
	want := []string{
		"Robin Hobb/Assassin/01 L'apprenti.sdr/metadata.epub.lua",      // KOReader default
		"Robin Hobb/Assassin/01 L'apprenti.epub.sdr/metadata.epub.lua", // fallback
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("sidecarLpaths = %v, want %v", got, want)
	}
	// Format derived from the extension when not supplied.
	if lp := sidecarLpaths("dir/Book.pdf", "")[0]; lp != "dir/Book.sdr/metadata.pdf.lua" {
		t.Errorf("derived-format lpath = %q", lp)
	}
}

func TestBestStatus(t *testing.T) {
	if bestStatus("reading", "complete") != "complete" {
		t.Error("complete should win over reading")
	}
	if bestStatus("complete", "reading") != "complete" {
		t.Error("complete should be kept")
	}
	if bestStatus("", "new") != "new" {
		t.Error("new should win over empty")
	}
}

func writeSidecar(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

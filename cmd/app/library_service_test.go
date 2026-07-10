package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agrison/reliure/internal/core"
	"github.com/agrison/reliure/internal/settings"
)

func TestManagedPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Library")
	cases := []struct {
		name string
		path string
		want bool
	}{
		{"inside", filepath.Join(root, "Author", "Book", "book.epub"), true},
		{"root itself", root, false},
		{"sibling prefix", root + "-old/book.epub", false},
		{"outside", filepath.Join(filepath.Dir(root), "Elsewhere", "book.epub"), false},
		{"empty root", filepath.Join(root, "book.epub"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			libraryDir := root
			if tc.name == "empty root" {
				libraryDir = ""
			}
			if got := managedPath(tc.path, libraryDir); got != tc.want {
				t.Fatalf("managedPath(%q, %q) = %v, want %v", tc.path, libraryDir, got, tc.want)
			}
		})
	}
}

func TestRemoveBookTrashesOnlyManagedFiles(t *testing.T) {
	svc, bookID, managedFile, externalFile := newRemoveBookService(t)
	var trashed []string
	restore := moveToTrash
	moveToTrash = func(path string) error {
		trashed = append(trashed, path)
		return os.Remove(path)
	}
	t.Cleanup(func() { moveToTrash = restore })

	res, err := svc.RemoveBook(bookID)
	if err != nil {
		t.Fatal(err)
	}
	if res.RemovedFromIndex != 1 || res.TrashedFiles != 1 || res.KeptFiles != 1 {
		t.Fatalf("result = %+v", res)
	}
	if !reflect.DeepEqual(trashed, []string{managedFile}) {
		t.Fatalf("trashed = %#v, want managed file only", trashed)
	}
	if _, err := os.Stat(managedFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("managed file stat = %v, want not exist", err)
	}
	if _, err := os.Stat(externalFile); err != nil {
		t.Fatalf("external file should remain: %v", err)
	}
	if _, err := svc.db.Books.ByID(bookID); err == nil {
		t.Fatal("book still indexed after RemoveBook")
	}
}

func TestRemoveBookKeepsIndexWhenTrashFails(t *testing.T) {
	svc, bookID, _, _ := newRemoveBookService(t)
	restore := moveToTrash
	moveToTrash = func(string) error { return errors.New("trash failed") }
	t.Cleanup(func() { moveToTrash = restore })

	if _, err := svc.RemoveBook(bookID); err == nil {
		t.Fatal("RemoveBook succeeded despite trash failure")
	}
	if _, err := svc.db.Books.ByID(bookID); err != nil {
		t.Fatalf("book should remain indexed after trash failure: %v", err)
	}
}

func TestUpdateBookMovesManagedFilesAndKeepsExternalFiles(t *testing.T) {
	svc, bookID, managedFile, externalFile := newRemoveBookService(t)

	got, err := svc.UpdateBook(BookUpdate{
		ID:          bookID,
		Title:       "New Title",
		Authors:     []string{"New Author"},
		Series:      "Cycle",
		SeriesIndex: "2.5",
		Tags:        []string{"edited", "test"},
		Language:    "fr",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantManaged := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(managedFile))), "New Author", "New Title", "New Title.epub")
	if _, err := os.Stat(wantManaged); err != nil {
		t.Fatalf("moved managed file missing: %v", err)
	}
	if _, err := os.Stat(managedFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old managed file stat = %v, want not exist", err)
	}
	if _, err := os.Stat(externalFile); err != nil {
		t.Fatalf("external file should remain: %v", err)
	}
	if got.Title != "New Title" || len(got.Authors) != 1 || got.Authors[0].Name != "New Author" {
		t.Fatalf("updated detail = %+v", got)
	}
	updated, err := svc.db.Books.ByID(bookID)
	if err != nil {
		t.Fatal(err)
	}
	var sawManaged, sawExternal bool
	for _, f := range updated.Files {
		if f.Path == wantManaged {
			sawManaged = true
		}
		if f.Path == externalFile {
			sawExternal = true
		}
	}
	if !sawManaged || !sawExternal {
		t.Fatalf("updated files = %+v, want managed %q and external %q", updated.Files, wantManaged, externalFile)
	}
}

func TestBatchSetSeriesAssignsConsecutiveIndices(t *testing.T) {
	svc, firstID, _, _ := newRemoveBookService(t)
	second := &core.Book{Title: "Second", Authors: []core.Contribution{{Author: core.Author{Name: "A"}}}}
	if err := svc.db.Books.Create(second); err != nil {
		t.Fatal(err)
	}

	res, err := svc.BatchSetSeries(BatchSeriesUpdate{
		IDs:              []int64{firstID, second.ID},
		Series:           "Batch Cycle",
		SeriesIndexStart: "4",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 2 {
		t.Fatalf("updated = %d, want 2", res.Updated)
	}
	first, _ := svc.db.Books.ByID(firstID)
	secondGot, _ := svc.db.Books.ByID(second.ID)
	if first.Series == nil || first.Series.Name != "Batch Cycle" || first.SeriesIndex == nil || *first.SeriesIndex != 4 {
		t.Fatalf("first after batch = %+v", first)
	}
	if secondGot.Series == nil || secondGot.Series.Name != "Batch Cycle" || secondGot.SeriesIndex == nil || *secondGot.SeriesIndex != 5 {
		t.Fatalf("second after batch = %+v", secondGot)
	}
}

func newRemoveBookService(t *testing.T) (*LibraryService, int64, string, string) {
	t.Helper()
	db, err := core.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	root := t.TempDir()
	libraryDir := filepath.Join(root, "Library")
	coverDir := filepath.Join(root, "covers")
	managedFile := filepath.Join(libraryDir, "Author", "Title", "Title.epub")
	externalFile := filepath.Join(root, "External", "Title.epub")
	mustWrite(t, managedFile)
	mustWrite(t, externalFile)

	book := &core.Book{
		Title: "Title",
		Files: []core.File{
			{Path: managedFile, Format: "epub", Size: 1, SHA256: "managed"},
			{Path: externalFile, Format: "epub", Size: 1, SHA256: "external"},
		},
	}
	if err := db.Books.Create(book); err != nil {
		t.Fatal(err)
	}

	store, err := settings.Open(filepath.Join(root, "settings.json"), libraryDir)
	if err != nil {
		t.Fatal(err)
	}
	return &LibraryService{db: db, settings: store, coverDir: coverDir}, book.ID, managedFile, externalFile
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

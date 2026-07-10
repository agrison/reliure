package device

import (
	"os"
	"testing"
	"time"
)

func TestInventoryUpsertReplacesByFileID(t *testing.T) {
	inv := newInventory("KOReader")
	first := Entry{BookID: 1, FileID: 10, RemotePath: "Old.epub", SentAt: time.Now()}
	next := Entry{BookID: 1, FileID: 10, RemotePath: "New.epub", SentAt: time.Now()}

	inv.Upsert(first)
	inv.Upsert(next)

	if len(inv.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(inv.Entries))
	}
	if inv.Entries[0].RemotePath != "New.epub" {
		t.Fatalf("remote path = %q", inv.Entries[0].RemotePath)
	}
}

func TestStoreSaveLoad(t *testing.T) {
	store := NewStore(t.TempDir())
	inv := newInventory(`KO/Reader:Test`)
	inv.Upsert(Entry{BookID: 2, FileID: 20, RemotePath: "Author/Book.epub", Title: "Book"})

	if err := store.Save(inv); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(`KO/Reader:Test`)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion {
		t.Fatalf("schema = %d", got.SchemaVersion)
	}
	if len(got.Entries) != 1 || got.Entries[0].BookID != 2 {
		t.Fatalf("entries = %+v", got.Entries)
	}
}

func TestLoadMissingReturnsEmptyInventory(t *testing.T) {
	got, err := NewStore(t.TempDir()).Load("Missing")
	if err != nil {
		t.Fatal(err)
	}
	if got.DeviceName != "Missing" || len(got.Entries) != 0 {
		t.Fatalf("inventory = %+v", got)
	}
}

func TestLoadRejectsCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := os.WriteFile(store.path("Broken"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("Broken"); err == nil {
		t.Fatal("expected corrupt JSON error")
	}
}

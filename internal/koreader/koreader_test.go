package koreader

import (
	"os"
	"path/filepath"
	"testing"
)

// A modern KOReader sidecar: unified annotations array + doc_props + summary.
const modernSidecar = `-- we can read Lua syntax here!
return {
    ["percent_finished"] = 0.4237,
    ["doc_pages"] = 312,
    ["doc_props"] = {
        ["title"] = "L'assassin royal",
        ["authors"] = "Robin Hobb",
        ["language"] = "fr",
    },
    ["summary"] = {
        ["status"] = "reading",
        ["modified"] = "2026-07-10",
        ["note"] = "",
    },
    ["annotations"] = {
        [1] = {
            ["datetime"] = "2026-07-09 21:14:03",
            ["drawer"] = "lighten",
            ["chapter"] = "Chapitre 1",
            ["text"] = "Une phrase surlignée",
        },
        [2] = {
            ["datetime"] = "2026-07-10 08:02:55",
            ["chapter"] = "Chapitre 2",
            ["text"] = "Un passage",
            ["note"] = "ma note perso",
        },
    },
    ["stats"] = {
        ["total_time_in_sec"] = 3600,
    },
}`

// A legacy sidecar: highlight table keyed by page + a bookmark note.
const legacySidecar = `return {
    ["percent_finished"] = 1,
    ["summary"] = { ["status"] = "complete" },
    ["doc_props"] = { ["title"] = "Old Book", ["authors"] = "Jane Doe\nJohn Roe" },
    ["highlight"] = {
        [12] = {
            [1] = { ["text"] = "legacy highlight", ["chapter"] = "Ch. 3", ["datetime"] = "2020-01-01 00:00:00" },
        },
    },
    ["bookmarks"] = {
        [1] = { ["notes"] = "a legacy note", ["text"] = "Page 12", ["chapter"] = "Ch. 3" },
    },
}`

func TestParseModern(t *testing.T) {
	sc, err := parseString(t, modernSidecar)
	if err != nil {
		t.Fatal(err)
	}
	if sc.Title != "L'assassin royal" || sc.Language != "fr" {
		t.Errorf("doc_props wrong: %+v", sc)
	}
	if len(sc.Authors) != 1 || sc.Authors[0] != "Robin Hobb" {
		t.Errorf("authors = %v", sc.Authors)
	}
	if sc.Status != StatusReading {
		t.Errorf("status = %q, want reading", sc.Status)
	}
	if sc.PercentFinished < 0.42 || sc.PercentFinished > 0.43 {
		t.Errorf("percent = %v", sc.PercentFinished)
	}
	if sc.TotalPages != 312 {
		t.Errorf("total pages = %d, want 312", sc.TotalPages)
	}
	if len(sc.Annotations) != 2 {
		t.Fatalf("got %d annotations, want 2", len(sc.Annotations))
	}
	if sc.Annotations[0].Text != "Une phrase surlignée" || sc.Annotations[0].Drawer != "lighten" {
		t.Errorf("annotation[0] = %+v", sc.Annotations[0])
	}
	if sc.Annotations[1].Note != "ma note perso" {
		t.Errorf("annotation[1] note = %q", sc.Annotations[1].Note)
	}
}

func TestParseLegacy(t *testing.T) {
	sc, err := parseString(t, legacySidecar)
	if err != nil {
		t.Fatal(err)
	}
	if sc.Status != StatusComplete || sc.PercentFinished != 1 {
		t.Errorf("progress wrong: %+v", sc)
	}
	if len(sc.Authors) != 2 {
		t.Errorf("authors = %v, want two", sc.Authors)
	}
	// One highlight (from `highlight`) + one note (from `bookmarks`).
	if len(sc.Annotations) != 2 {
		t.Fatalf("got %d annotations, want 2", len(sc.Annotations))
	}
	var haveHL, haveNote bool
	for _, a := range sc.Annotations {
		if a.Text == "legacy highlight" {
			haveHL = true
		}
		if a.Note == "a legacy note" {
			haveNote = true
		}
	}
	if !haveHL || !haveNote {
		t.Errorf("legacy annotations incomplete: %+v", sc.Annotations)
	}
}

func TestParseMalformedIsError(t *testing.T) {
	if _, err := parseString(t, `return 42`); err == nil {
		t.Error("expected error for non-table sidecar")
	}
	if _, err := parseString(t, `this is not lua {{{`); err == nil {
		t.Error("expected error for invalid lua")
	}
}

// TestSandboxNoOS proves the standard library is unavailable to a sidecar, so it
// cannot touch the host even if it tries.
func TestSandboxNoOS(t *testing.T) {
	if _, err := parseString(t, `os.exit(1) return {}`); err == nil {
		t.Error("expected error: os library must not be available in the sandbox")
	}
}

func TestScan(t *testing.T) {
	root := t.TempDir()
	sdr := filepath.Join(root, "Books", "L'assassin royal.sdr")
	if err := os.MkdirAll(sdr, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sdr, "metadata.epub.lua"), []byte(modernSidecar), 0o644); err != nil {
		t.Fatal(err)
	}
	// A non-sidecar file that must be ignored.
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	scs, errs := Scan(root)
	if len(errs) != 0 {
		t.Fatalf("scan errors: %v", errs)
	}
	if len(scs) != 1 {
		t.Fatalf("got %d sidecars, want 1", len(scs))
	}
	sc := scs[0]
	if sc.Format != "epub" {
		t.Errorf("format = %q, want epub", sc.Format)
	}
	if sc.DocBasename != "L'assassin royal.epub" {
		t.Errorf("docBasename = %q", sc.DocBasename)
	}
}

func parseString(t *testing.T, src string) (*Sidecar, error) {
	t.Helper()
	p := newParser()
	defer p.close()
	return p.parse(src)
}

package main

import (
	"strings"
	"testing"
)

func TestLooksLikeSQLite(t *testing.T) {
	valid := append([]byte("SQLite format 3\x00"), make([]byte, 100)...)
	if !looksLikeSQLite(valid) {
		t.Error("valid SQLite header not recognised")
	}
	if looksLikeSQLite([]byte("not a database")) {
		t.Error("non-SQLite data wrongly accepted")
	}
	if looksLikeSQLite([]byte("SQLite")) {
		t.Error("too-short data wrongly accepted")
	}
}

func TestStatisticsCandidates(t *testing.T) {
	cands := statisticsCandidates()
	if len(cands) < 4 {
		t.Fatalf("want several candidate paths, got %d", len(cands))
	}
	for _, c := range cands {
		if !strings.HasSuffix(c, "statistics.sqlite3") {
			t.Errorf("candidate %q does not end with the db name", c)
		}
	}
	// Must cover both the traversal case and the koreader-prefixed case.
	var hasTraversal, hasPrefixed bool
	for _, c := range cands {
		if strings.HasPrefix(c, "..") {
			hasTraversal = true
		}
		if strings.Contains(c, "koreader/settings/") {
			hasPrefixed = true
		}
	}
	if !hasTraversal || !hasPrefixed {
		t.Errorf("candidates miss a case: traversal=%v prefixed=%v", hasTraversal, hasPrefixed)
	}
}

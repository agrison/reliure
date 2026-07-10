package library

import (
	"testing"

	"github.com/agrison/reliure/internal/core"
)

func TestRenderRemotePath(t *testing.T) {
	idx := 3.0
	got := RenderRemotePath("{authors}/{series}/{series_index} {title}", &core.Book{
		Title:       "Dune: Messiah",
		Language:    "fr",
		SeriesIndex: &idx,
		Series:      &core.Series{Name: "Dune"},
		Authors: []core.Contribution{
			{Author: core.Author{Name: "Frank Herbert"}},
		},
		Tags: []core.Tag{{Name: "SF"}},
	})
	want := "Frank Herbert/Dune/03 Dune_ Messiah"
	if got != want {
		t.Fatalf("RenderRemotePath = %q, want %q", got, want)
	}
}

func TestRenderRemotePathDropsEmptySegments(t *testing.T) {
	got := RenderRemotePath("{authors}/{series}/{series_index} {title}", &core.Book{
		Title: "Standalone",
	})
	want := "Standalone"
	if got != want {
		t.Fatalf("RenderRemotePath = %q, want %q", got, want)
	}
}

package library

import (
	"fmt"
	"path"
	"strings"

	"github.com/agrison/reliure/internal/core"
)

// RenderRemotePath expands the KOReader remote path template for a book.
// Supported variables: {title}, {authors}, {author}, {series},
// {series_index}, {tags}, {language}. Empty path segments are removed.
func RenderRemotePath(tmpl string, b *core.Book) string {
	if strings.TrimSpace(tmpl) == "" {
		tmpl = "{authors}/{series}/{series_index} {title}"
	}
	values := map[string]string{
		"title":        b.Title,
		"authors":      strings.Join(b.AuthorNames(), ", "),
		"author":       first(b.AuthorNames()),
		"series":       "",
		"series_index": formatSeriesIndex(b.SeriesIndex),
		"tags":         tagNames(b.Tags),
		"language":     b.Language,
	}
	if b.Series != nil {
		values["series"] = b.Series.Name
	}
	out := tmpl
	for k, v := range values {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return cleanRemotePath(out, b.Title)
}

func cleanRemotePath(in, fallback string) string {
	var parts []string
	for _, p := range strings.Split(in, "/") {
		p = sanitizeRemotePart(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		parts = append(parts, sanitizeRemotePart(fallback))
	}
	return path.Join(parts...)
}

func sanitizeRemotePart(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		if r < 0x20 {
			return '_'
		}
		return r
	}, s)
	return strings.Trim(s, " .")
}

func formatSeriesIndex(v *float64) string {
	if v == nil {
		return ""
	}
	if *v == float64(int64(*v)) {
		return fmt.Sprintf("%02d", int64(*v))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", *v), "0"), ".")
}

func tagNames(tags []core.Tag) string {
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		names = append(names, t.Name)
	}
	return strings.Join(names, ", ")
}

func first(in []string) string {
	if len(in) == 0 {
		return ""
	}
	return in[0]
}

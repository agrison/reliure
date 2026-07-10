package opds

import (
	"encoding/xml"
	"strconv"
	"strings"
	"time"

	"github.com/agrison/reliure/internal/core"
)

type feed struct {
	XMLName xml.Name `xml:"feed"`
	Xmlns   string   `xml:"xmlns,attr"`
	ID      string   `xml:"id"`
	Title   string   `xml:"title"`
	Updated string   `xml:"updated"`
	Links   []link   `xml:"link,omitempty"`
	Entries []entry  `xml:"entry,omitempty"`
}

type link struct {
	Rel    string `xml:"rel,attr,omitempty"`
	Type   string `xml:"type,attr,omitempty"`
	Href   string `xml:"href,attr"`
	Title  string `xml:"title,attr,omitempty"`
	Length int64  `xml:"length,attr,omitempty"`
}

type entry struct {
	ID        string `xml:"id"`
	Title     string `xml:"title"`
	Updated   string `xml:"updated"`
	Summary   string `xml:"summary,omitempty"`
	Content   string `xml:"content,omitempty"`
	Authors   []name `xml:"author,omitempty"`
	Links     []link `xml:"link,omitempty"`
	Published string `xml:"published,omitempty"`
}

type name struct {
	Name string `xml:"name"`
}

func navFeed(title, self string, now time.Time) feed {
	return feed{
		Xmlns:   "http://www.w3.org/2005/Atom",
		ID:      self,
		Title:   title,
		Updated: now.UTC().Format(time.RFC3339),
		Links: []link{
			{Rel: "self", Type: navType, Href: self},
			{Rel: "start", Type: navType, Href: catalogRoot(self)},
		},
	}
}

func acquisitionFeed(title, self string, now time.Time) feed {
	f := navFeed(title, self, now)
	if len(f.Links) > 0 {
		f.Links[0].Type = atomType
	}
	return f
}

func navEntry(id, title, href string) entry {
	return entry{
		ID:      href + "#" + id,
		Title:   title,
		Updated: time.Now().UTC().Format(time.RFC3339),
		Links:   []link{{Rel: "subsection", Type: navType, Href: href}},
	}
}

func bookEntry(base string, b *core.Book, f core.File) entry {
	e := entry{
		ID:        base + "/books/" + strconv.FormatInt(b.ID, 10),
		Title:     b.Title,
		Updated:   b.UpdatedAt.UTC().Format(time.RFC3339),
		Summary:   b.Description,
		Published: b.PublishedAt,
		Links: []link{{
			Rel:    "http://opds-spec.org/acquisition",
			Type:   epubType,
			Href:   base + "/books/" + strconv.FormatInt(b.ID, 10) + "/files/" + strconv.FormatInt(f.ID, 10),
			Length: f.Size,
		}},
	}
	for _, c := range b.Authors {
		e.Authors = append(e.Authors, name{Name: c.Author.Name})
	}
	if b.CoverPath != "" {
		href := base + "/covers/" + b.CoverPath
		e.Links = append(e.Links,
			link{Rel: "http://opds-spec.org/image/thumbnail", Href: href},
			link{Rel: "http://opds-spec.org/image", Href: href},
		)
	}
	if b.Series != nil {
		if b.SeriesIndex != nil {
			e.Content = b.Series.Name + " #" + strconv.FormatFloat(*b.SeriesIndex, 'f', -1, 64)
		} else {
			e.Content = b.Series.Name
		}
	}
	return e
}

func catalogRoot(self string) string {
	withoutScheme := strings.TrimPrefix(strings.TrimPrefix(self, "https://"), "http://")
	if i := strings.IndexByte(withoutScheme, '/'); i >= 0 {
		return strings.TrimSuffix(self[:len(self)-len(withoutScheme)+i], "/") + "/"
	}
	return strings.TrimSuffix(self, "/") + "/"
}

type openSearchDescription struct {
	XMLName     xml.Name      `xml:"OpenSearchDescription"`
	XMLNS       string        `xml:"xmlns,attr"`
	ShortName   string        `xml:"ShortName"`
	Description string        `xml:"Description"`
	URL         openSearchURL `xml:"Url"`
}

type openSearchURL struct {
	Type     string `xml:"type,attr"`
	Template string `xml:"template,attr"`
}

package epub

import (
	"strconv"
	"strings"

	"github.com/agrison/reliure/internal/formats"
)

// The OPF package document is XML. We match elements and attributes by local
// name only (no namespace in the struct tags), which lets Go's encoding/xml
// transparently handle the dc: and opf: prefixes that vary across files.

type opfPackage struct {
	Version  string      `xml:"version,attr"`
	Metadata opfMetadata `xml:"metadata"`
	Manifest struct {
		Items []opfItem `xml:"item"`
	} `xml:"manifest"`
}

type opfMetadata struct {
	Titles       []dcTitle      `xml:"title"`
	Languages    []string       `xml:"language"`
	Descriptions []string       `xml:"description"`
	Publishers   []string       `xml:"publisher"`
	Dates        []string       `xml:"date"`
	Subjects     []string       `xml:"subject"`
	Creators     []dcAgent      `xml:"creator"`
	Contributors []dcAgent      `xml:"contributor"`
	Identifiers  []dcIdentifier `xml:"identifier"`
	Metas        []opfMeta      `xml:"meta"`
}

type dcTitle struct {
	Value string `xml:",chardata"`
	ID    string `xml:"id,attr"`
}

// dcAgent is a dc:creator or dc:contributor. Role/FileAs come from the EPUB2
// opf:role / opf:file-as attributes; EPUB3 expresses the same via <meta refines>
// elements resolved through the agent's ID.
type dcAgent struct {
	Name   string `xml:",chardata"`
	Role   string `xml:"role,attr"`
	FileAs string `xml:"file-as,attr"`
	ID     string `xml:"id,attr"`
}

type dcIdentifier struct {
	Value  string `xml:",chardata"`
	Scheme string `xml:"scheme,attr"`
	ID     string `xml:"id,attr"`
}

// opfMeta covers both EPUB2 (<meta name=".." content=".."/>) and EPUB3
// (<meta property=".." refines="#id">value</meta>) metadata shapes.
type opfMeta struct {
	Name     string `xml:"name,attr"`
	Content  string `xml:"content,attr"`
	Property string `xml:"property,attr"`
	Refines  string `xml:"refines,attr"`
	ID       string `xml:"id,attr"`
	Value    string `xml:",chardata"`
}

type opfItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

// toMetadata maps a parsed package onto the format-neutral BookMetadata. It is
// tolerant: missing fields stay zero, it never fails. Title may be empty here;
// the caller substitutes the filename fallback.
func (p *opfPackage) toMetadata() formats.BookMetadata {
	md := formats.BookMetadata{Identifiers: map[string]string{}}

	// Index <meta refines="#id"> elements by the id they refine (EPUB3).
	refines := map[string][]opfMeta{}
	for _, m := range p.Metadata.Metas {
		if m.Refines != "" {
			refines[strings.TrimPrefix(m.Refines, "#")] = append(refines[strings.TrimPrefix(m.Refines, "#")], m)
		}
	}

	// Title (+ EPUB3 file-as → sort title).
	if len(p.Metadata.Titles) > 0 {
		md.Title = strings.TrimSpace(p.Metadata.Titles[0].Value)
		for _, rm := range refines[p.Metadata.Titles[0].ID] {
			if localName(rm.Property) == "file-as" {
				md.TitleSort = strings.TrimSpace(rm.Value)
			}
		}
	}

	md.Contributors = append(md.Contributors, agents(p.Metadata.Creators, refines, "aut")...)
	md.Contributors = append(md.Contributors, agents(p.Metadata.Contributors, refines, "ctb")...)

	md.Language = firstNonEmpty(p.Metadata.Languages)
	md.Description = firstNonEmpty(p.Metadata.Descriptions)
	md.Publisher = firstNonEmpty(p.Metadata.Publishers)
	md.Published = firstNonEmpty(p.Metadata.Dates)
	for _, s := range p.Metadata.Subjects {
		if s = strings.TrimSpace(s); s != "" {
			md.Tags = append(md.Tags, s)
		}
	}

	for _, id := range p.Metadata.Identifiers {
		scheme, val := normalizeIdentifier(id.Scheme, id.Value)
		if scheme != "" && val != "" {
			if _, exists := md.Identifiers[scheme]; !exists {
				md.Identifiers[scheme] = val
			}
		}
	}
	md.ISBN = md.Identifiers["isbn"]

	// Calibre metadata (very common in real-world EPUBs).
	for _, m := range p.Metadata.Metas {
		switch strings.ToLower(strings.TrimSpace(m.Name)) {
		case "calibre:series":
			md.Series = strings.TrimSpace(m.Content)
		case "calibre:series_index":
			md.SeriesIndex = parseFloatPtr(m.Content)
		case "calibre:title_sort":
			if md.TitleSort == "" {
				md.TitleSort = strings.TrimSpace(m.Content)
			}
		}
	}

	// EPUB3 collection as a series fallback when Calibre metadata is absent.
	if md.Series == "" {
		for _, m := range p.Metadata.Metas {
			if localName(m.Property) != "belongs-to-collection" {
				continue
			}
			name := strings.TrimSpace(m.Value)
			var ctype, pos string
			for _, rm := range refines[m.ID] {
				switch localName(rm.Property) {
				case "collection-type":
					ctype = strings.TrimSpace(rm.Value)
				case "group-position":
					pos = rm.Value
				}
			}
			if name != "" && (ctype == "series" || ctype == "") {
				md.Series = name
				if md.SeriesIndex == nil {
					md.SeriesIndex = parseFloatPtr(pos)
				}
				break
			}
		}
	}

	return md
}

// agents converts dc:creator/dc:contributor entries into Contributors,
// resolving EPUB3 refines and dropping obvious non-persons (software producers).
func agents(list []dcAgent, refines map[string][]opfMeta, defaultRole string) []formats.Contributor {
	var out []formats.Contributor
	for _, a := range list {
		name := strings.TrimSpace(a.Name)
		if name == "" {
			continue
		}
		role, fileAs := a.Role, a.FileAs
		for _, rm := range refines[a.ID] {
			switch localName(rm.Property) {
			case "role":
				if role == "" {
					role = strings.TrimSpace(rm.Value)
				}
			case "file-as":
				if fileAs == "" {
					fileAs = strings.TrimSpace(rm.Value)
				}
			}
		}
		// "bkp" (book producer) and Calibre-as-contributor are noise, not people.
		if role == "bkp" || strings.Contains(strings.ToLower(name), "calibre") {
			continue
		}
		if role == "" {
			role = defaultRole
		}
		out = append(out, formats.Contributor{Name: name, SortName: strings.TrimSpace(fileAs), Role: role})
	}
	return out
}

// coverHref returns the manifest href of the cover image (relative to the OPF),
// trying, in order: EPUB2 <meta name="cover">, EPUB3 cover-image property, a
// name/id heuristic, then the first image. Empty if the book has no image.
func (p *opfPackage) coverHref() string {
	items := p.Manifest.Items

	var coverID string
	for _, m := range p.Metadata.Metas {
		if strings.EqualFold(strings.TrimSpace(m.Name), "cover") && m.Content != "" {
			coverID = m.Content
			break
		}
	}
	if coverID != "" {
		for _, it := range items {
			if it.ID == coverID && isImage(it) {
				return it.Href
			}
		}
	}
	for _, it := range items {
		if hasProperty(it.Properties, "cover-image") {
			return it.Href
		}
	}
	for _, it := range items {
		if isImage(it) && (containsFold(it.ID, "cover") || containsFold(it.Href, "cover")) {
			return it.Href
		}
	}
	for _, it := range items {
		if isImage(it) {
			return it.Href
		}
	}
	return ""
}

// --- helpers ---

func firstNonEmpty(values []string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// localName strips any namespace prefix and lowercases: "dcterms:modified" →
// "modified", "belongs-to-collection" → "belongs-to-collection".
func localName(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	return strings.ToLower(s)
}

func parseFloatPtr(s string) *float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

// normalizeIdentifier resolves an identifier's scheme and value, unwrapping
// urn: forms and inferring ISBN/UUID when the scheme is unspecified.
func normalizeIdentifier(scheme, value string) (string, string) {
	value = strings.TrimSpace(value)
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	low := strings.ToLower(value)
	switch {
	case strings.HasPrefix(low, "urn:isbn:"):
		return "isbn", value[len("urn:isbn:"):]
	case strings.HasPrefix(low, "urn:uuid:"):
		return "uuid", value[len("urn:uuid:"):]
	case strings.HasPrefix(low, "isbn:"):
		return "isbn", value[len("isbn:"):]
	}
	if scheme == "" {
		switch {
		case looksISBN(value):
			scheme = "isbn"
		case looksUUID(value):
			scheme = "uuid"
		}
	}
	if scheme == "isbn" {
		value = strings.NewReplacer("-", "", " ", "").Replace(value)
	}
	return scheme, value
}

func looksISBN(s string) bool {
	s = strings.NewReplacer("-", "", " ", "").Replace(s)
	if len(s) != 10 && len(s) != 13 {
		return false
	}
	for i, r := range s {
		if r >= '0' && r <= '9' {
			continue
		}
		if (r == 'X' || r == 'x') && i == len(s)-1 && len(s) == 10 {
			continue
		}
		return false
	}
	return true
}

func looksUUID(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
			continue
		}
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func isImage(it opfItem) bool {
	if strings.HasPrefix(strings.ToLower(it.MediaType), "image/") {
		return true
	}
	switch strings.ToLower(extOf(it.Href)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg":
		return true
	}
	return false
}

func extOf(href string) string {
	href = stripFragment(href)
	if i := strings.LastIndex(href, "."); i >= 0 {
		return href[i:]
	}
	return ""
}

func hasProperty(props, want string) bool {
	for _, p := range strings.Fields(props) {
		if strings.EqualFold(p, want) {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

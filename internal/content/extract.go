// Package content extracts searchable plain text from ebook files. It is best
// effort by design: metadata search remains available even when content
// extraction fails or yields no text.
package content

import (
	"archive/zip"
	"fmt"
	"html"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agrison/reliure/internal/core"
)

const (
	maxEntryBytes = 2 << 20
	maxTextRunes  = 2_000_000
)

var (
	scriptStyleRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>`)
	tagRe         = regexp.MustCompile(`(?is)<[^>]+>`)
	entityRe      = regexp.MustCompile(`\s+`)
)

func Extract(path, format string) (string, error) {
	fragments, err := ExtractFragments(path, format)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, f := range fragments {
		appendText(&b, f.Text)
	}
	return trimRunes(b.String(), maxTextRunes), nil
}

func ExtractFragments(path, format string) ([]core.ContentFragment, error) {
	switch strings.ToLower(format) {
	case "epub":
		return EPUBFragments(path)
	case "pdf":
		return PDFFragments(path)
	default:
		return nil, fmt.Errorf("unsupported content format %q", format)
	}
}

func EPUB(filePath string) (string, error) {
	fragments, err := EPUBFragments(filePath)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, f := range fragments {
		appendText(&b, f.Text)
	}
	return trimRunes(b.String(), maxTextRunes), nil
}

func EPUBFragments(filePath string) ([]core.ContentFragment, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	var out []core.ContentFragment
	total := 0
	for _, f := range zr.File {
		if !textualEPUBEntry(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxEntryBytes))
		rc.Close()
		if err != nil {
			continue
		}
		text := htmlToText(data)
		if text == "" {
			continue
		}
		out = append(out, core.ContentFragment{Page: len(out) + 1, Text: text})
		total += runeLen(text)
		if total >= maxTextRunes {
			break
		}
	}
	return out, nil
}

func PDF(filePath string) (string, error) {
	fragments, err := PDFFragments(filePath)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, f := range fragments {
		appendText(&b, f.Text)
	}
	return trimRunes(b.String(), maxTextRunes), nil
}

func PDFFragments(filePath string) ([]core.ContentFragment, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	var out []core.ContentFragment
	page := 1
	total := 0
	for _, s := range pdfLiteralStrings(data) {
		appendText(&b, s)
		if runeLen(b.String()) >= 2500 {
			text := b.String()
			out = append(out, core.ContentFragment{Page: page, Text: text})
			total += runeLen(text)
			page++
			b.Reset()
		}
		if total >= maxTextRunes {
			break
		}
	}
	if strings.TrimSpace(b.String()) != "" && total < maxTextRunes {
		out = append(out, core.ContentFragment{Page: page, Text: b.String()})
	}
	return out, nil
}

func textualEPUBEntry(name string) bool {
	ext := strings.ToLower(path.Ext(name))
	switch ext {
	case ".xhtml", ".html", ".htm":
		return true
	default:
		return false
	}
}

func htmlToText(data []byte) string {
	s := string(data)
	s = scriptStyleRe.ReplaceAllString(s, " ")
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return normalizeSpace(s)
}

func pdfLiteralStrings(data []byte) []string {
	var out []string
	for i := 0; i < len(data); i++ {
		if data[i] != '(' {
			continue
		}
		s, next, ok := readPDFLiteral(data, i+1)
		if !ok {
			continue
		}
		i = next
		s = normalizeSpace(s)
		if usefulText(s) {
			out = append(out, s)
		}
	}
	return out
}

func readPDFLiteral(data []byte, i int) (string, int, bool) {
	var out []byte
	depth := 1
	for i < len(data) {
		c := data[i]
		i++
		switch c {
		case '\\':
			if i >= len(data) {
				return string(out), i, true
			}
			esc := data[i]
			i++
			switch esc {
			case 'n', 'r':
				out = append(out, '\n')
			case 't':
				out = append(out, '\t')
			case 'b', 'f':
			case '(', ')', '\\':
				out = append(out, esc)
			default:
				if esc >= '0' && esc <= '7' {
					val := int(esc - '0')
					for j := 0; j < 2 && i < len(data) && data[i] >= '0' && data[i] <= '7'; j++ {
						val = val*8 + int(data[i]-'0')
						i++
					}
					out = append(out, byte(val))
				}
			}
		case '(':
			depth++
			out = append(out, c)
		case ')':
			depth--
			if depth == 0 {
				return string(out), i, true
			}
			out = append(out, c)
		default:
			if c >= 0x20 || c == '\n' || c == '\r' || c == '\t' {
				out = append(out, c)
			}
		}
	}
	return "", i, false
}

func usefulText(s string) bool {
	letters := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
			if letters >= 3 {
				return true
			}
		}
	}
	return false
}

func appendText(b *strings.Builder, s string) {
	if strings.TrimSpace(s) == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteByte('\n')
	}
	b.WriteString(s)
}

func normalizeSpace(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return ' '
		}
		return r
	}, s)
	return strings.TrimSpace(entityRe.ReplaceAllString(s, " "))
}

func trimRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if runeLen(s) <= max {
		return s
	}
	var b strings.Builder
	count := 0
	for _, r := range s {
		if count >= max {
			break
		}
		b.WriteRune(r)
		count++
	}
	return b.String()
}

func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

func FormatFromPath(p string) string {
	ext := strings.ToLower(filepath.Ext(p))
	if ext == ".pdf" {
		return "pdf"
	}
	return "epub"
}

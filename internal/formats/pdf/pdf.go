// Package pdf parses basic PDF document metadata behind a formats.FormatHandler.
// It is deliberately tolerant: a PDF without readable Info metadata still
// imports with the filename as title.
package pdf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/agrison/reliure/internal/formats"
)

const scanWindow = 4 << 20 // 4 MiB from both the beginning and end.

// Handler implements formats.FormatHandler for PDF.
type Handler struct{}

// New returns a PDF handler.
func New() Handler { return Handler{} }

// Register the handler in the default registry on import.
func init() { formats.Default.Register(New()) }

func (Handler) Format() string { return "pdf" }

// CanHandle recognises PDF files by extension.
func (Handler) CanHandle(path string) bool { return formats.HasExt(path, ".pdf") }

// Metadata reads the standard PDF Info dictionary fields when they are present.
func (Handler) Metadata(path string) (formats.BookMetadata, error) {
	md := formats.BookMetadata{
		Title:       fallbackTitle(path),
		Identifiers: map[string]string{},
	}
	data, err := readScanBytes(path)
	if err != nil {
		return md, fmt.Errorf("pdf %q: metadata scan: %w", filepath.Base(path), err)
	}
	if title, ok := infoString(data, "Title"); ok && strings.TrimSpace(title) != "" {
		md.Title = strings.TrimSpace(title)
	}
	if author, ok := infoString(data, "Author"); ok && strings.TrimSpace(author) != "" {
		md.Contributors = []formats.Contributor{{Name: strings.TrimSpace(author), Role: "aut"}}
	}
	if subject, ok := infoString(data, "Subject"); ok {
		md.Description = strings.TrimSpace(subject)
	}
	if keywords, ok := infoString(data, "Keywords"); ok {
		md.Tags = splitKeywords(keywords)
	}
	if created, ok := infoString(data, "CreationDate"); ok {
		md.Published = pdfDate(created)
	}
	return md, nil
}

// Cover returns nil: PDF cover rasterization would require a rendering engine,
// which is intentionally outside the current lightweight parser.
func (Handler) Cover(string) ([]byte, error) { return nil, nil }

func readScanBytes(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()
	if size <= scanWindow*2 {
		return os.ReadFile(path)
	}
	head := make([]byte, scanWindow)
	if _, err := f.ReadAt(head, 0); err != nil {
		return nil, err
	}
	tail := make([]byte, scanWindow)
	if _, err := f.ReadAt(tail, size-scanWindow); err != nil {
		return nil, err
	}
	return append(append(head, '\n'), tail...), nil
}

func infoString(data []byte, key string) (string, bool) {
	needle := []byte("/" + key)
	for off := 0; ; {
		idx := bytes.Index(data[off:], needle)
		if idx < 0 {
			return "", false
		}
		i := off + idx + len(needle)
		if i < len(data) && isNameChar(data[i]) {
			off += idx + len(needle)
			continue
		}
		i = skipSpace(data, i)
		if i >= len(data) {
			return "", false
		}
		switch data[i] {
		case '(':
			return literalString(data, i+1)
		case '<':
			if i+1 < len(data) && data[i+1] == '<' {
				off += idx + len(needle)
				continue
			}
			return hexString(data, i+1)
		default:
			off += idx + len(needle)
		}
	}
}

func literalString(data []byte, i int) (string, bool) {
	var out []byte
	depth := 1
	for i < len(data) {
		c := data[i]
		i++
		switch c {
		case '\\':
			if i >= len(data) {
				return string(out), true
			}
			esc := data[i]
			i++
			switch esc {
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case '(', ')', '\\':
				out = append(out, esc)
			case '\r':
				if i < len(data) && data[i] == '\n' {
					i++
				}
			case '\n':
			default:
				if esc >= '0' && esc <= '7' {
					oct := []byte{esc}
					for len(oct) < 3 && i < len(data) && data[i] >= '0' && data[i] <= '7' {
						oct = append(oct, data[i])
						i++
					}
					if v, err := strconv.ParseUint(string(oct), 8, 8); err == nil {
						out = append(out, byte(v))
					}
				} else {
					out = append(out, esc)
				}
			}
		case '(':
			depth++
			out = append(out, c)
		case ')':
			depth--
			if depth == 0 {
				return decodePDFString(out), true
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return "", false
}

func hexString(data []byte, i int) (string, bool) {
	var hex []byte
	for i < len(data) && data[i] != '>' {
		if !isSpace(data[i]) {
			hex = append(hex, data[i])
		}
		i++
	}
	if i >= len(data) {
		return "", false
	}
	if len(hex)%2 == 1 {
		hex = append(hex, '0')
	}
	out := make([]byte, len(hex)/2)
	for j := range out {
		v, err := strconv.ParseUint(string(hex[j*2:j*2+2]), 16, 8)
		if err != nil {
			return "", false
		}
		out[j] = byte(v)
	}
	return decodePDFString(out), true
}

func decodePDFString(b []byte) string {
	if len(b) >= 2 {
		switch {
		case b[0] == 0xfe && b[1] == 0xff:
			return utf16String(b[2:], binary.BigEndian)
		case b[0] == 0xff && b[1] == 0xfe:
			return utf16String(b[2:], binary.LittleEndian)
		}
	}
	return strings.TrimSpace(string(b))
}

func utf16String(b []byte, order binary.ByteOrder) string {
	if len(b)%2 == 1 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = order.Uint16(b[i*2 : i*2+2])
	}
	return strings.TrimSpace(string(utf16.Decode(u)))
}

func splitKeywords(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, f := range fields {
		f = strings.TrimSpace(f)
		key := strings.ToLower(f)
		if f == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
}

func pdfDate(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(s, "D:"))
	if len(s) >= 8 && digits(s[:8]) {
		return s[:4] + "-" + s[4:6] + "-" + s[6:8]
	}
	if len(s) >= 4 && digits(s[:4]) {
		return s[:4]
	}
	return s
}

func fallbackTitle(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSpace(strings.TrimSuffix(base, ext))
}

func skipSpace(data []byte, i int) int {
	for i < len(data) && isSpace(data[i]) {
		i++
	}
	return i
}

func isSpace(b byte) bool {
	switch b {
	case 0, '\t', '\n', '\f', '\r', ' ':
		return true
	default:
		return false
	}
}

func isNameChar(b byte) bool {
	return !isSpace(b) && !strings.ContainsRune("[]<>{}()/%", rune(b))
}

func digits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

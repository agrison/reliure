package metadata

import "strings"

// marcToISO maps the common MARC/ISO 639-2 three-letter language codes that
// OpenLibrary returns onto the ISO 639-1 two-letter codes EPUBs and Google Books
// use, so the same book in French compares equal whatever the source.
var marcToISO = map[string]string{
	"fre": "fr", "fra": "fr",
	"eng": "en",
	"ger": "de", "deu": "de",
	"spa": "es",
	"ita": "it",
	"por": "pt",
	"dut": "nl", "nld": "nl",
	"rus": "ru",
	"jpn": "ja",
	"chi": "zh", "zho": "zh",
	"lat": "la",
	"pol": "pl",
	"swe": "sv",
	"nor": "no",
	"dan": "da",
	"fin": "fi",
	"gre": "el", "ell": "el",
	"ara": "ar",
	"heb": "he",
	"kor": "ko",
	"tur": "tr",
	"cze": "cs", "ces": "cs",
	"hun": "hu",
	"ukr": "uk",
}

// normalizeLang reduces a language tag to a lower-case ISO 639-1 code: it strips
// a region subtag ("fr-FR" → "fr") and maps known three-letter codes ("fre" →
// "fr"). Unknown values are returned lower-cased and trimmed as-is.
func normalizeLang(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, "-_"); i > 0 {
		s = s[:i]
	}
	if len(s) == 3 {
		if iso, ok := marcToISO[s]; ok {
			return iso
		}
	}
	return s
}

// digitsOnly keeps only the digits of s (used to compare ISBNs regardless of
// hyphenation); a trailing "X" checksum is dropped, which is fine for keying.
func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

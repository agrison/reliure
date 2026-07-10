package pdf

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// makeJPEG returns encoded JPEG bytes of a w×h image.
func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// writePDFWithImage builds a minimal byte blob shaped like a PDF image XObject
// (enough for the byte-scanning extractor) and writes it to a temp .pdf.
func writePDFWithImage(t *testing.T, jpegBytes []byte, w, h int, filter string) string {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("%PDF-1.5\n")
	b.WriteString("1 0 obj\n<< /Type /XObject /Subtype /Image /Width ")
	b.WriteString(strconv.Itoa(w))
	b.WriteString(" /Height ")
	b.WriteString(strconv.Itoa(h))
	b.WriteString(" /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /")
	b.WriteString(filter)
	b.WriteString(" /Length ")
	b.WriteString(strconv.Itoa(len(jpegBytes)))
	b.WriteString(" >>\nstream\n")
	b.Write(jpegBytes)
	b.WriteString("\nendstream\nendobj\n%%EOF")

	path := filepath.Join(t.TempDir(), "doc.pdf")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCoverExtractsEmbeddedJPEG(t *testing.T) {
	jpg := makeJPEG(t, 300, 420)
	path := writePDFWithImage(t, jpg, 300, 420, "DCTDecode")

	cover, err := New().Cover(path)
	if err != nil {
		t.Fatalf("Cover: %v", err)
	}
	if cover == nil {
		t.Fatal("expected a cover, got nil")
	}
	// The extracted bytes must be a decodable JPEG of the right size.
	cfg, format, err := image.DecodeConfig(bytes.NewReader(cover))
	if err != nil {
		t.Fatalf("cover is not a valid image: %v", err)
	}
	if format != "jpeg" || cfg.Width != 300 || cfg.Height != 420 {
		t.Errorf("cover = %s %dx%d, want jpeg 300x420", format, cfg.Width, cfg.Height)
	}
}

func TestCoverSkipsTinyImages(t *testing.T) {
	// A small logo (below the dimension threshold) must not be used as a cover.
	jpg := makeJPEG(t, 32, 32)
	path := writePDFWithImage(t, jpg, 32, 32, "DCTDecode")

	cover, err := New().Cover(path)
	if err != nil {
		t.Fatal(err)
	}
	if cover != nil {
		t.Errorf("tiny image should be ignored, got %d bytes", len(cover))
	}
}

func TestCoverNoImage(t *testing.T) {
	// A text-only PDF (no image XObject) yields no cover, not an error.
	path := filepath.Join(t.TempDir(), "text.pdf")
	os.WriteFile(path, []byte("%PDF-1.5\n1 0 obj\n<< /Type /Page >>\nendobj\n%%EOF"), 0o644)

	cover, err := New().Cover(path)
	if err != nil || cover != nil {
		t.Fatalf("expected (nil, nil), got (%v, %v)", cover, err)
	}
}

func TestCoverIgnoresNonJPEGFilter(t *testing.T) {
	// A CCITT/JBIG2/Flate image isn't a JPEG stream; the extractor should skip it.
	jpg := makeJPEG(t, 300, 420) // bytes present but declared as a different filter
	path := writePDFWithImage(t, jpg, 300, 420, "FlateDecode")

	cover, err := New().Cover(path)
	if err != nil {
		t.Fatal(err)
	}
	if cover != nil {
		t.Errorf("non-DCTDecode image should be skipped, got %d bytes", len(cover))
	}
}

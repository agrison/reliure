package formats

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestThumbnailDownscalesPreservingAspect(t *testing.T) {
	// 200×300 source (portrait) → max side 100 → 66×100.
	src := image.NewRGBA(image.Rect(0, 0, 200, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 200; x++ {
			src.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var raw bytes.Buffer
	if err := png.Encode(&raw, src); err != nil {
		t.Fatal(err)
	}

	out, err := Thumbnail(raw.Bytes(), 100)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode thumbnail: %v", err)
	}
	if format != "jpeg" {
		t.Errorf("thumbnail format = %q, want jpeg", format)
	}
	if cfg.Height != 100 || cfg.Width != 66 {
		t.Errorf("thumbnail dims = %dx%d, want 66x100", cfg.Width, cfg.Height)
	}
}

func TestThumbnailNoUpscale(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 40, 50))
	var raw bytes.Buffer
	png.Encode(&raw, src)
	out, err := Thumbnail(raw.Bytes(), 100)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 40 || cfg.Height != 50 {
		t.Errorf("small image should not upscale: %dx%d", cfg.Width, cfg.Height)
	}
}

func TestThumbnailAcceptsWebP(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Thumbnail(raw, 100)
	if err != nil {
		t.Fatalf("Thumbnail WebP: %v", err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode thumbnail: %v", err)
	}
	if format != "jpeg" {
		t.Errorf("thumbnail format = %q, want jpeg", format)
	}
	if cfg.Width != 1 || cfg.Height != 1 {
		t.Errorf("thumbnail dims = %dx%d, want 1x1", cfg.Width, cfg.Height)
	}
}

func TestThumbnailRejectsGarbage(t *testing.T) {
	if _, err := Thumbnail([]byte("not an image"), 100); err == nil {
		t.Error("expected decode error for non-image input")
	}
}

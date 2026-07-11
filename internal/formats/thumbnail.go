package formats

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"

	// Register decoders so image.Decode handles the formats covers use.
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/png"

	xdraw "golang.org/x/image/draw"
)

// thumbnailQuality is the JPEG quality for generated thumbnails: a good
// size/quality trade-off for cover grids.
const thumbnailQuality = 82

// Thumbnail decodes a raw cover image and returns a JPEG thumbnail scaled so its
// largest side is at most maxDim pixels, preserving aspect ratio. Images already
// within bounds are re-encoded without upscaling. Input may be JPEG, PNG, GIF,
// or WebP; output is always JPEG. It is format-agnostic so every FormatHandler's
// covers share one thumbnail path.
func Thumbnail(raw []byte, maxDim int) ([]byte, error) {
	if maxDim <= 0 {
		return nil, errors.New("thumbnail: maxDim must be positive")
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, errors.New("thumbnail: empty source image")
	}

	nw, nh := fitWithin(w, h, maxDim)
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	// Catmull-Rom gives noticeably crisper downscales than bilinear for text-
	// heavy book covers, at negligible cost for these small sizes.
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: thumbnailQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// fitWithin returns the largest w×h scaled so both sides fit in maxDim, never
// upscaling.
func fitWithin(w, h, maxDim int) (int, int) {
	if w <= maxDim && h <= maxDim {
		return w, h
	}
	if w >= h {
		return maxDim, max(1, h*maxDim/w)
	}
	return max(1, w*maxDim/h), maxDim
}

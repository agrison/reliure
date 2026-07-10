package pdf

import (
	"bytes"
	"io"
	"os"
	"strconv"
)

const (
	// coverScanBytes bounds how much of the file we scan for the cover image.
	// A cover is essentially always within the first pages; this keeps memory
	// and time bounded on huge PDFs.
	coverScanBytes = 48 << 20
	// minCoverDim / minCoverBytes filter out logos and tiny decorations so we
	// land on a real page image.
	minCoverDim   = 200
	minCoverBytes = 20 << 10
)

// Cover extracts a cover image for the PDF: the first sufficiently large
// embedded JPEG (a /DCTDecode image XObject), which for scanned books and
// comics is the first page. It rasterizes nothing, so text-only PDFs (or scans
// using CCITT/JBIG2/JPXDecode rather than JPEG) yield no cover — the UI then
// shows a placeholder. Pure Go, tolerant: any problem returns (nil, nil).
func (Handler) Cover(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	n := st.Size()
	if n > coverScanBytes {
		n = coverScanBytes
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(f, data); err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return firstEmbeddedJPEG(data), nil
}

// firstEmbeddedJPEG scans for image XObjects encoded with DCTDecode (i.e. a
// JPEG) and returns the raw bytes of the first one large enough to be a cover.
func firstEmbeddedJPEG(data []byte) []byte {
	for off := 0; off < len(data); {
		idx := bytes.Index(data[off:], []byte("/Subtype"))
		if idx < 0 {
			return nil
		}
		p := off + idx
		off = p + len("/Subtype")

		streamAt := bytes.Index(data[p:], []byte("stream"))
		if streamAt < 0 {
			return nil
		}
		dict := data[p : p+streamAt]
		if !bytes.Contains(dict, []byte("/Image")) || !bytes.Contains(dict, []byte("/DCTDecode")) {
			continue
		}

		// The JPEG begins with SOI (FF D8) right after the "stream" keyword
		// (past its trailing EOL); it ends at EOI (FF D9).
		body := data[p+streamAt+len("stream"):]
		soi := bytes.Index(body, []byte{0xFF, 0xD8})
		if soi < 0 || soi > 64 {
			continue
		}
		eoi := bytes.Index(body[soi:], []byte{0xFF, 0xD9})
		if eoi < 0 {
			continue
		}
		jpeg := body[soi : soi+eoi+2]

		w, h := intAfter(dict, "/Width"), intAfter(dict, "/Height")
		bigEnough := (w >= minCoverDim && h >= minCoverDim) ||
			((w == 0 || h == 0) && len(jpeg) >= minCoverBytes)
		if !bigEnough {
			continue
		}
		return append([]byte(nil), jpeg...)
	}
	return nil
}

// intAfter returns the integer value following key in dict, or 0.
func intAfter(dict []byte, key string) int {
	idx := bytes.Index(dict, []byte(key))
	if idx < 0 {
		return 0
	}
	i := idx + len(key)
	for i < len(dict) && isSpace(dict[i]) {
		i++
	}
	j := i
	for j < len(dict) && dict[j] >= '0' && dict[j] <= '9' {
		j++
	}
	if j == i {
		return 0
	}
	v, _ := strconv.Atoi(string(dict[i:j]))
	return v
}

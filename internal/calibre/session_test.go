package calibre

import (
	"bufio"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/agrison/reliure/internal/core"
)

// captured is what the simulated device received.
type captured struct {
	lpath string
	body  []byte
	title string
	// files is the device's virtual filesystem for GET_BOOK_FILE_SEGMENT.
	files map[string][]byte
}

// fakeDevice plays a minimal KOReader: it answers the handshake and stores the
// SEND_BOOK it receives, then returns when told to eject.
func fakeDevice(conn net.Conn, out *captured) error {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		msg, err := readMessage(r)
		if err != nil {
			return nil // connection closed
		}
		switch msg.Op {
		case OpGetInitializationInfo:
			_ = writeMessage(conn, OpOK, map[string]any{
				"versionOK":               true,
				"maxBookContentPacketLen": 4096,
				"canStreamBooks":          true,
				"canStreamMetadata":       true,
				"canReceiveBookBinary":    true,
				"canDeleteMultipleBooks":  true,
				"canSendOkToSendbook":     true,
			})
		case OpGetDeviceInformation:
			_ = writeMessage(conn, OpOK, map[string]any{
				"device_info":    map[string]any{"device_name": "KOReader Sim", "device_store_uuid": "u-1"},
				"device_version": "1",
				"version":        "1",
			})
		case OpSetCalibreDeviceInfo:
			_ = writeMessage(conn, OpOK, map[string]any{})
		case OpSendBook:
			lpath, _ := msg.Payload["lpath"].(string)
			length := int(msg.Payload["length"].(float64))
			if meta, ok := msg.Payload["metadata"].(map[string]any); ok {
				out.title, _ = meta["title"].(string)
			}
			_ = writeMessage(conn, OpOK, map[string]any{"lpath": lpath})
			body := make([]byte, length)
			if _, err := io.ReadFull(r, body); err != nil {
				return err
			}
			out.lpath = lpath
			out.body = body
		case OpGetBookFileSegment:
			lpath, _ := msg.Payload["lpath"].(string)
			data, ok := out.files[lpath]
			if !ok {
				_ = writeMessage(conn, OpNoop, map[string]any{}) // not found
				continue
			}
			_ = writeMessage(conn, OpOK, map[string]any{"fileLength": len(data)})
			_, _ = conn.Write(data)
		case OpNoop:
			if ejecting, _ := msg.Payload["ejecting"].(bool); ejecting {
				return nil
			}
		}
	}
}

func TestSessionHandshakeAndSendBook(t *testing.T) {
	clientConn, deviceConn := net.Pipe()
	var got captured
	deviceErr := make(chan error, 1)
	go func() { deviceErr <- fakeDevice(deviceConn, &got) }()

	sess := newSession(clientConn)
	if err := sess.Handshake("Ma Bibliothèque", "uuid-1"); err != nil {
		t.Fatalf("Handshake: %v", err)
	}
	if sess.DeviceName != "KOReader Sim" {
		t.Errorf("device name = %q, want KOReader Sim", sess.DeviceName)
	}
	if !sess.canSendOK {
		t.Error("canSendOK should be negotiated true")
	}

	// A real file to stream.
	path := filepath.Join(t.TempDir(), "book.epub")
	content := []byte("PK\x03\x04 fake epub payload")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	b := &core.Book{
		ID:      7,
		Title:   "La Main gauche de la nuit",
		Authors: []core.Contribution{{Author: core.Author{Name: "Ursula K. Le Guin"}}},
	}

	lpath, err := sess.SendBook(b, path, "Le Guin/Ekumen/04 Main gauche.epub", 1, 1)
	if err != nil {
		t.Fatalf("SendBook: %v", err)
	}
	if lpath != "Le Guin/Ekumen/04 Main gauche.epub" {
		t.Errorf("lpath = %q", lpath)
	}
	sess.Close()

	if err := <-deviceErr; err != nil {
		t.Fatalf("device error: %v", err)
	}
	// Verify the device got the subfoldered path, the file bytes and metadata.
	if got.lpath != "Le Guin/Ekumen/04 Main gauche.epub" {
		t.Errorf("device lpath = %q", got.lpath)
	}
	if string(got.body) != string(content) {
		t.Errorf("device body = %q, want %q", got.body, content)
	}
	if got.title != "La Main gauche de la nuit" {
		t.Errorf("device metadata title = %q", got.title)
	}
}

func TestSessionGetFile(t *testing.T) {
	clientConn, deviceConn := net.Pipe()
	sidecar := []byte(`return { ["percent_finished"] = 0.5 }`)
	got := captured{files: map[string][]byte{
		"Le Guin/Ekumen/04 Main gauche.sdr/metadata.epub.lua": sidecar,
	}}
	deviceErr := make(chan error, 1)
	go func() { deviceErr <- fakeDevice(deviceConn, &got) }()

	sess := newSession(clientConn)
	if err := sess.Handshake("Lib", "uuid"); err != nil {
		t.Fatalf("Handshake: %v", err)
	}

	// An existing file streams back verbatim.
	data, found, err := sess.GetFile("Le Guin/Ekumen/04 Main gauche.sdr/metadata.epub.lua")
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if !found || string(data) != string(sidecar) {
		t.Fatalf("found=%v data=%q", found, data)
	}

	// A missing file replies NOOP → found=false, no hang, and the session stays
	// usable for the next request.
	_, found, err = sess.GetFile("nope/metadata.epub.lua")
	if err != nil {
		t.Fatalf("GetFile(missing): %v", err)
	}
	if found {
		t.Error("expected found=false for a missing file")
	}
	// Prove the stream is still aligned after the two exchanges.
	if data, found, err := sess.GetFile("Le Guin/Ekumen/04 Main gauche.sdr/metadata.epub.lua"); err != nil || !found || string(data) != string(sidecar) {
		t.Fatalf("stream misaligned: found=%v err=%v", found, err)
	}
	sess.Close()
	if err := <-deviceErr; err != nil {
		t.Fatalf("device error: %v", err)
	}
}

func TestSessionSendFile(t *testing.T) {
	clientConn, deviceConn := net.Pipe()
	var got captured
	deviceErr := make(chan error, 1)
	go func() { deviceErr <- fakeDevice(deviceConn, &got) }()

	sess := newSession(clientConn)
	if err := sess.Handshake("Ma Bibliothèque", "uuid-1"); err != nil {
		t.Fatalf("Handshake: %v", err)
	}

	path := filepath.Join(t.TempDir(), ".reliure")
	content := []byte(`{"schema_version":1}`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	lpath, err := sess.SendFile(path, ".reliure", nil, 1, 1)
	if err != nil {
		t.Fatalf("SendFile: %v", err)
	}
	if lpath != ".reliure" {
		t.Errorf("lpath = %q", lpath)
	}
	sess.Close()

	if err := <-deviceErr; err != nil {
		t.Fatalf("device error: %v", err)
	}
	if got.lpath != ".reliure" {
		t.Errorf("device lpath = %q", got.lpath)
	}
	if string(got.body) != string(content) {
		t.Errorf("device body = %q", got.body)
	}
}

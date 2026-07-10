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

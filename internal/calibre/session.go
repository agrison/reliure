package calibre

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/agrison/reliure/internal/core"
)

// Session is a connected device conversation. Reliure drives it: it sends
// command opcodes and reads the device's replies. All exchanges are serialized
// so a book transfer and a keep-alive can't interleave on the socket.
type Session struct {
	conn net.Conn
	r    *bufio.Reader
	mu   sync.Mutex

	DeviceName string
	maxPacket  int
	canSendOK  bool
}

func newSession(conn net.Conn) *Session {
	return &Session{conn: conn, r: bufio.NewReader(conn), maxPacket: 4096, DeviceName: "Liseuse"}
}

// call writes a command and reads its reply. Callers hold s.mu (or run before
// any concurrency, as in Handshake).
func (s *Session) call(op Opcode, payload map[string]any) (Message, error) {
	if err := writeMessage(s.conn, op, payload); err != nil {
		return Message{}, err
	}
	return readMessage(s.r)
}

// Handshake performs the initial exchange that "opens" the device: it
// negotiates the protocol, learns the device's capabilities and name, and
// acknowledges the device info. Must be called once, before any SendBook.
func (s *Session) Handshake(libraryName, libraryUUID string) error {
	reply, err := s.call(OpGetInitializationInfo, map[string]any{
		"serverProtocolVersion":  protocolVersion,
		"validExtensions":        []string{"epub", "pdf"},
		"passwordChallenge":      "",
		"currentLibraryName":     libraryName,
		"currentLibraryUUID":     libraryUUID,
		"pubdateFormat":          "",
		"timestampFormat":        "",
		"lastModifiedFormat":     "",
		"calibre_version":        []int{7, 0, 0},
		"canSupportUpdateBooks":  true,
		"canSupportLpathChanges": true,
	})
	if err != nil {
		return err
	}
	if reply.Op != OpOK {
		return fmt.Errorf("calibre: initialization rejected (opcode %d)", reply.Op)
	}
	if ok, _ := reply.Payload["versionOK"].(bool); !ok {
		return errors.New("calibre: protocol version mismatch with device")
	}
	if v, ok := reply.Payload["maxBookContentPacketLen"].(float64); ok && v > 0 {
		s.maxPacket = int(v)
	}
	s.canSendOK, _ = reply.Payload["canSendOkToSendbook"].(bool)

	// Learn the device name and acknowledge its info.
	info, err := s.call(OpGetDeviceInformation, map[string]any{})
	if err != nil {
		return err
	}
	if info.Op == OpOK {
		if di, ok := info.Payload["device_info"].(map[string]any); ok {
			if name, ok := di["device_name"].(string); ok && name != "" {
				s.DeviceName = name
			}
			if _, err := s.call(OpSetCalibreDeviceInfo, di); err != nil {
				return err
			}
		}
	}
	return nil
}

// SendBook transfers one file to the device at lpath (which may contain
// subfolders). thisBook/totalBooks drive the device's progress display. It
// returns the effective lpath (the device may rewrite it).
func (s *Session) SendBook(b *core.Book, filePath, lpath string, thisBook, totalBooks int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	length := st.Size()

	payload := map[string]any{
		"lpath":                  lpath,
		"length":                 length,
		"metadata":               bookMetadata(b, lpath, length),
		"thisBook":               thisBook,
		"totalBooks":             totalBooks,
		"willStreamBooks":        true,
		"willStreamBinary":       true,
		"wantsSendOkToSendbook":  s.canSendOK,
		"canSupportLpathChanges": true,
	}

	// When the device supports it, it acknowledges SEND_BOOK (and may rewrite
	// lpath) before we stream. Otherwise we stream immediately.
	if s.canSendOK {
		reply, err := s.call(OpSendBook, payload)
		if err != nil {
			return "", err
		}
		if reply.Op == OpError {
			return "", fmt.Errorf("calibre: device rejected %q: %v", lpath, reply.Payload["message"])
		}
		if lp, ok := reply.Payload["lpath"].(string); ok && lp != "" {
			lpath = lp
		}
	} else if err := writeMessage(s.conn, OpSendBook, payload); err != nil {
		return "", err
	}

	// Stream the raw file bytes (unframed), exactly `length` bytes.
	if _, err := io.CopyBuffer(s.conn, f, make([]byte, 64*1024)); err != nil {
		return "", err
	}
	return lpath, nil
}

// Noop sends a keep-alive and reads the reply, keeping the connection warm.
func (s *Session) Noop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.call(OpNoop, map[string]any{})
	return err
}

// Close tells the device we are ejecting and closes the socket.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = writeMessage(s.conn, OpNoop, map[string]any{"ejecting": true})
	return s.conn.Close()
}

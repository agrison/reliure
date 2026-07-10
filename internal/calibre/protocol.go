// Package calibre implements the client side of Calibre's "smart device"
// wireless protocol so Reliure can push books to KOReader over WiFi.
//
// Despite the naming, roles are subtle: Calibre (and therefore Reliure) is the
// protocol *master* — it opens a TCP listener, answers the device's UDP
// discovery pings, and once the device (KOReader) connects it drives the
// exchange by sending command opcodes to which the device replies. The wire
// format and opcode numbers mirror calibre's driver.py.
//
// The headline advantage over OPDS: SEND_BOOK carries an `lpath` that may
// include subfolders, so Reliure can dictate the on-device file layout.
package calibre

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

// Opcode identifies a protocol message type. Values are fixed by calibre.
type Opcode int

const (
	OpNoop                 Opcode = 12
	OpOK                   Opcode = 0
	OpBookDone             Opcode = 11
	OpCalibreBusy          Opcode = 18
	OpSetLibraryInfo       Opcode = 19
	OpDeleteBook           Opcode = 13
	OpDisplayMessage       Opcode = 17
	OpError                Opcode = 20
	OpFreeSpace            Opcode = 5
	OpGetBookFileSegment   Opcode = 14
	OpGetBookMetadata      Opcode = 15
	OpGetBookCount         Opcode = 6
	OpGetDeviceInformation Opcode = 3
	OpGetInitializationInfo Opcode = 9
	OpSendBooklists        Opcode = 7
	OpSendBook             Opcode = 8
	OpSendBookMetadata     Opcode = 16
	OpSetCalibreDeviceInfo Opcode = 1
	OpSetCalibreDeviceName Opcode = 2
	OpTotalSpace           Opcode = 4
)

const (
	// protocolVersion is calibre's PROTOCOL_VERSION.
	protocolVersion = 1
	// clientString is what calibre answers discovery pings with; KOReader keys
	// off it to recognise a calibre server.
	clientString = "calibre wireless device client"
)

// broadcastPorts are the UDP ports KOReader pings to locate a calibre server.
var broadcastPorts = []int{54982, 48123, 39001, 44044, 59678}

// Message is a decoded protocol message: an opcode plus its JSON payload.
type Message struct {
	Op      Opcode
	Payload map[string]any
}

// writeMessage frames and writes a message on w. The frame is the decimal
// ASCII byte length of the JSON immediately followed by the JSON array
// `[opcode, payload]` — exactly calibre's `(b'%d' % len(s)) + s`.
func writeMessage(w io.Writer, op Opcode, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	body, err := json.Marshal([]any{int(op), payload})
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, strconv.Itoa(len(body))); err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

// readMessage reads one framed message from r. It consumes the decimal length
// prefix (digits up to the opening '['), then exactly that many JSON bytes.
func readMessage(r *bufio.Reader) (Message, error) {
	var digits []byte
	for {
		b, err := r.ReadByte()
		if err != nil {
			return Message{}, err
		}
		if b == '[' {
			_ = r.UnreadByte()
			break
		}
		if b < '0' || b > '9' {
			return Message{}, fmt.Errorf("calibre: unexpected length byte %q", b)
		}
		digits = append(digits, b)
	}
	if len(digits) == 0 {
		return Message{}, fmt.Errorf("calibre: missing length prefix")
	}
	n, err := strconv.Atoi(string(digits))
	if err != nil {
		return Message{}, fmt.Errorf("calibre: bad length %q: %w", digits, err)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return Message{}, err
	}

	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return Message{}, fmt.Errorf("calibre: decoding message: %w", err)
	}
	if len(raw) != 2 {
		return Message{}, fmt.Errorf("calibre: message has %d elements, want 2", len(raw))
	}
	var op int
	if err := json.Unmarshal(raw[0], &op); err != nil {
		return Message{}, fmt.Errorf("calibre: decoding opcode: %w", err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw[1], &payload); err != nil {
		return Message{}, fmt.Errorf("calibre: decoding payload: %w", err)
	}
	return Message{Op: Opcode(op), Payload: payload}, nil
}

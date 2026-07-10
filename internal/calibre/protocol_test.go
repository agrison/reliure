package calibre

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestWriteMessageFraming(t *testing.T) {
	var buf bytes.Buffer
	if err := writeMessage(&buf, OpNoop, map[string]any{"count": 3}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	// Frame is "<len>[json]"; the JSON is [12,{"count":3}] (16 bytes).
	body := `[12,{"count":3}]`
	want := "16" + body
	if got != want {
		t.Fatalf("framed = %q, want %q", got, want)
	}
	// The length prefix must equal the JSON byte length.
	if !strings.HasPrefix(got, "16[") || len(body) != 16 {
		t.Fatalf("length prefix mismatch: %q (body len %d)", got, len(body))
	}
}

func TestReadMessageRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	writeMessage(&buf, OpSendBook, map[string]any{"lpath": "Author/Book.epub", "length": 42})

	msg, err := readMessage(bufio.NewReader(&buf))
	if err != nil {
		t.Fatal(err)
	}
	if msg.Op != OpSendBook {
		t.Errorf("opcode = %d, want %d", msg.Op, OpSendBook)
	}
	if msg.Payload["lpath"] != "Author/Book.epub" {
		t.Errorf("lpath = %v", msg.Payload["lpath"])
	}
	// JSON numbers decode to float64.
	if msg.Payload["length"].(float64) != 42 {
		t.Errorf("length = %v", msg.Payload["length"])
	}
}

func TestReadMessageParsesCalibreWireFormat(t *testing.T) {
	// A message exactly as calibre would put it on the wire. The body
	// [0,{"versionOK":true,"x":1}] is 28 bytes.
	raw := `28[0,{"versionOK":true,"x":1}]`
	msg, err := readMessage(bufio.NewReader(strings.NewReader(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if msg.Op != OpOK {
		t.Errorf("opcode = %d, want OK(0)", msg.Op)
	}
	if ok, _ := msg.Payload["versionOK"].(bool); !ok {
		t.Errorf("versionOK not parsed: %v", msg.Payload)
	}
}

func TestReadMessageRejectsGarbage(t *testing.T) {
	if _, err := readMessage(bufio.NewReader(strings.NewReader("nonsense"))); err == nil {
		t.Error("expected error on non-numeric length prefix")
	}
}

func TestReadMessageBackToBack(t *testing.T) {
	// Two framed messages in a row must decode independently.
	var buf bytes.Buffer
	writeMessage(&buf, OpNoop, map[string]any{"a": 1})
	writeMessage(&buf, OpOK, map[string]any{"b": 2})
	r := bufio.NewReader(&buf)

	m1, err := readMessage(r)
	if err != nil || m1.Op != OpNoop {
		t.Fatalf("first message wrong: %+v err=%v", m1, err)
	}
	m2, err := readMessage(r)
	if err != nil || m2.Op != OpOK {
		t.Fatalf("second message wrong: %+v err=%v", m2, err)
	}
}

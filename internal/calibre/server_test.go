package calibre

import (
	"strconv"
	"strings"
	"testing"
)

func TestAnnounceReplyFormat(t *testing.T) {
	reply := announceReply(9090)
	// "<client> (on <host>);,<port>" — KOReader reads the port after the comma.
	if !strings.HasPrefix(reply, clientString+" (on ") {
		t.Errorf("reply missing client prefix: %q", reply)
	}
	if !strings.HasSuffix(reply, ";,9090") {
		t.Errorf("reply must end with the TCP port after a comma: %q", reply)
	}
}

func TestServerStartStop(t *testing.T) {
	srv := NewServer(ServerConfig{LibraryName: "Test"})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { srv.Stop() })

	if !srv.Running() {
		t.Error("server should be running")
	}
	if p := srv.Port(); p <= 0 {
		t.Errorf("port = %d, want > 0", p)
	}
	if _, connected := srv.Device(); connected {
		t.Error("no device should be connected yet")
	}

	// Start is idempotent; the port stays stable.
	port := srv.Port()
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	if srv.Port() != port {
		t.Errorf("port changed on second Start: %d → %d", port, srv.Port())
	}

	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if srv.Running() || srv.Port() != 0 {
		t.Error("server should be stopped and report port 0")
	}
	// Stop is idempotent.
	if err := srv.Stop(); err != nil {
		t.Errorf("second Stop: %v", err)
	}
}

func TestServerPortIsNumeric(t *testing.T) {
	srv := NewServer(ServerConfig{})
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()
	// The announce reply must contain the actual bound port.
	if !strings.HasSuffix(announceReply(srv.Port()), ";,"+strconv.Itoa(srv.Port())) {
		t.Error("announce reply port mismatch")
	}
}

package calibre

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// keepAliveInterval is how often we ping a connected device to keep the socket
// warm and to notice disconnections. Kept short so a disconnect surfaces in the
// UI within a few seconds.
const keepAliveInterval = 10 * time.Second

// defaultTCPPort is calibre's conventional smart-device port. We prefer it so
// the address is stable and can be entered manually in KOReader; if it is busy
// we fall back to an OS-assigned port (discovery still advertises the real one).
const defaultTCPPort = 9090

// ServerConfig configures a Server.
type ServerConfig struct {
	LibraryName  string
	LibraryUUID  string
	OnConnect    func(name string) // called when a device finishes the handshake
	OnDisconnect func()            // called when the device goes away
}

// Server answers KOReader's UDP discovery pings and accepts its TCP connection,
// then holds the resulting Session. Only one device is handled at a time.
type Server struct {
	cfg ServerConfig

	mu      sync.Mutex
	tcpLn   net.Listener
	udp     []*net.UDPConn
	session *Session
	port    int
	running bool
}

// NewServer creates an idle server.
func NewServer(cfg ServerConfig) *Server {
	return &Server{cfg: cfg}
}

// Start opens the TCP listener and the UDP discovery responders. Idempotent.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil
	}
	ln, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", defaultTCPPort))
	if err != nil {
		// Preferred port busy: let the OS pick one. Discovery still advertises
		// whatever we end up bound to, so KOReader can find it.
		if ln, err = net.Listen("tcp4", "0.0.0.0:0"); err != nil {
			return err
		}
	}
	s.tcpLn = ln
	s.port = ln.Addr().(*net.TCPAddr).Port

	// Best-effort: bind every discovery port we can. KOReader pings all of
	// them, so answering on any one suffices; a port already in use is skipped.
	for _, p := range broadcastPorts {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: p})
		if err != nil {
			continue
		}
		s.udp = append(s.udp, c)
		go s.serveUDP(c, s.port)
	}
	s.running = true
	go s.acceptLoop(ln)
	slog.Info("calibre: server started", "tcpPort", s.port, "udpPorts", len(s.udp))
	return nil
}

// Stop closes the listeners and any active session. Idempotent.
func (s *Server) Stop() error {
	s.mu.Lock()
	running := s.running
	ln, udp, sess := s.tcpLn, s.udp, s.session
	s.tcpLn, s.udp, s.session, s.running = nil, nil, nil, false
	s.mu.Unlock()
	if !running {
		return nil
	}
	if ln != nil {
		_ = ln.Close()
	}
	for _, c := range udp {
		_ = c.Close()
	}
	if sess != nil {
		_ = sess.Close()
	}
	slog.Info("calibre: server stopped")
	return nil
}

// Running reports whether the server is accepting connections.
func (s *Server) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Port returns the TCP port the device connects to (0 when stopped).
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return 0
	}
	return s.port
}

// Device returns the connected device's name, or ("", false) if none.
func (s *Server) Device() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return "", false
	}
	return s.session.DeviceName, true
}

// Session returns the connected session, or nil if no device is connected.
// Callers use it to SendBook; it is safe to hold across sends (transfers are
// internally serialized).
func (s *Server) Session() *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session
}

func (s *Server) currentSession() *Session { return s.Session() }

// announceReply is what we answer discovery pings with. It encodes the TCP port
// after the comma (the content-server port is left empty), matching calibre.
func announceReply(tcpPort int) string {
	return fmt.Sprintf("%s (on %s);,%d", clientString, shortHostname(), tcpPort)
}

// serveUDP answers discovery pings so KOReader learns our TCP port.
func (s *Server) serveUDP(c *net.UDPConn, tcpPort int) {
	reply := []byte(announceReply(tcpPort))
	buf := make([]byte, 1024)
	for {
		_, remote, err := c.ReadFromUDP(buf)
		if err != nil {
			return // socket closed on Stop
		}
		_, _ = c.WriteToUDP(reply, remote)
	}
}

func (s *Server) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return // listener closed on Stop
		}
		s.mu.Lock()
		busy := s.session != nil
		s.mu.Unlock()
		if busy {
			_ = conn.Close()
			continue
		}

		sess := newSession(conn)
		if err := sess.Handshake(s.cfg.LibraryName, s.cfg.LibraryUUID); err != nil {
			slog.Warn("calibre: handshake failed", "err", err)
			_ = conn.Close()
			continue
		}
		s.mu.Lock()
		s.session = sess
		s.mu.Unlock()
		slog.Info("calibre: device connected", "name", sess.DeviceName)
		if s.cfg.OnConnect != nil {
			s.cfg.OnConnect(sess.DeviceName)
		}
		go s.keepAlive(sess)
	}
}

func (s *Server) keepAlive(sess *Session) {
	ticker := time.NewTicker(keepAliveInterval)
	defer ticker.Stop()
	for range ticker.C {
		if s.currentSession() != sess {
			return // replaced or server stopped
		}
		if err := sess.Noop(); err != nil {
			s.dropSession(sess)
			return
		}
	}
}

func (s *Server) dropSession(sess *Session) {
	s.mu.Lock()
	dropped := s.session == sess
	if dropped {
		s.session = nil
	}
	s.mu.Unlock()
	if !dropped {
		return
	}
	_ = sess.Close()
	slog.Info("calibre: device disconnected", "name", sess.DeviceName)
	if s.cfg.OnDisconnect != nil {
		s.cfg.OnDisconnect()
	}
}

// shortHostname returns the host's first label, defaulting to "reliure".
func shortHostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "reliure"
	}
	if i := strings.IndexByte(h, '.'); i > 0 {
		h = h[:i]
	}
	return h
}

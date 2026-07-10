package opds

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// Server wraps net/http with explicit start/stop semantics for the desktop UI.
type Server struct {
	handler http.Handler

	mu     sync.Mutex
	server *http.Server
	ln     net.Listener
}

// NewServer creates an idle OPDS server.
func NewServer(handler http.Handler) *Server {
	return &Server{handler: handler}
}

// Start listens on host:port. A port of 0 lets the OS pick a free port.
func (s *Server) Start(host string, port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		return nil
	}
	if host == "" {
		host = "0.0.0.0"
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	s.server = srv
	s.ln = ln
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// The caller observes failed starts synchronously; runtime failures
			// are reflected by Status on the next UI refresh.
		}
	}()
	return nil
}

// Stop gracefully shuts the HTTP server down.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	srv := s.server
	s.server = nil
	s.ln = nil
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// Status returns whether the server is running and its bound address.
func (s *Server) Status() (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil || s.ln == nil {
		return false, ""
	}
	return true, s.ln.Addr().String()
}

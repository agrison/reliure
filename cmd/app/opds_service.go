package main

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/agrison/reliure/internal/opds"
	"github.com/agrison/reliure/internal/settings"
)

// OPDSService controls the local OPDS pull server exposed to KOReader.
type OPDSService struct {
	store  *settings.Store
	server *opds.Server
}

// OPDSStatus is the frontend-facing server state.
type OPDSStatus struct {
	Enabled bool   `json:"enabled"`
	Running bool   `json:"running"`
	Port    int    `json:"port"`
	URL     string `json:"url"`
	Error   string `json:"error,omitempty"`
}

// startFromSettings starts OPDS at app boot when the persisted preference says
// it should be running.
func (s *OPDSService) startFromSettings() {
	cfg := s.store.Get()
	if cfg.OPDSEnabled {
		_ = s.server.Start("", cfg.OPDSPort)
	}
}

// shutdown stops the server. It is safe to call even when it is already idle.
func (s *OPDSService) shutdown(ctx context.Context) error {
	return s.server.Stop(ctx)
}

// Status returns the persisted preference plus runtime state.
func (s *OPDSService) Status() OPDSStatus {
	cfg := s.store.Get()
	running, addr := s.server.Status()
	return OPDSStatus{
		Enabled: cfg.OPDSEnabled,
		Running: running,
		Port:    cfg.OPDSPort,
		URL:     publicURL(addr),
	}
}

// SetEnabled persists the on/off preference and starts or stops the server.
func (s *OPDSService) SetEnabled(enabled bool) (OPDSStatus, error) {
	cfg := s.store.Get()
	cfg.OPDSEnabled = enabled
	next, err := s.store.Update(cfg)
	if err != nil {
		return s.Status(), err
	}
	if enabled {
		if err := s.server.Start("", next.OPDSPort); err != nil {
			st := s.Status()
			st.Error = err.Error()
			return st, err
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.server.Stop(ctx); err != nil {
			return s.Status(), err
		}
	}
	return s.Status(), nil
}

// SetPort persists the TCP port. If OPDS is running, it is restarted on the new
// port immediately.
func (s *OPDSService) SetPort(port int) (OPDSStatus, error) {
	cfg := s.store.Get()
	cfg.OPDSPort = port
	next, err := s.store.Update(cfg)
	if err != nil {
		return s.Status(), err
	}
	running, _ := s.server.Status()
	if running {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.server.Stop(ctx); err != nil {
			return s.Status(), err
		}
		if err := s.server.Start("", next.OPDSPort); err != nil {
			st := s.Status()
			st.Error = err.Error()
			return st, err
		}
	}
	return s.Status(), nil
}

func publicURL(addr string) string {
	if addr == "" {
		return ""
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	if host == "" || host == "::" || strings.HasPrefix(host, "0.0.0.0") {
		if ip := firstLANIPv4(); ip != "" {
			host = ip
		} else {
			host = "127.0.0.1"
		}
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return "http://" + host + ":" + port + "/"
}

func firstLANIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		ip := ipNet.IP.To4()
		if ip == nil {
			continue
		}
		return ip.String()
	}
	return ""
}

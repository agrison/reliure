package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/agrison/reliure/internal/calibre"
	"github.com/agrison/reliure/internal/core"
	"github.com/agrison/reliure/internal/device"
	"github.com/agrison/reliure/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Frontend event names for the Calibre wireless push server.
const (
	calibreStatusEvent   = "calibre:status"
	calibreProgressEvent = "calibre:progress"
)

func init() {
	application.RegisterEvent[CalibreStatus](calibreStatusEvent)
	application.RegisterEvent[CalibreSendProgress](calibreProgressEvent)
}

// CalibreService controls the Calibre wireless (push) server that lets KOReader
// connect over WiFi, and sends books to the connected device.
type CalibreService struct {
	db        *core.DB
	settings  *settings.Store
	server    *calibre.Server
	inventory *device.Store
}

// CalibreStatus is the frontend-facing push-server state.
type CalibreStatus struct {
	Running   bool   `json:"running"`
	Connected bool   `json:"connected"`
	Device    string `json:"device"`
	Port      int    `json:"port"`
	// Address is the LAN host:port KOReader can be pointed at manually if
	// UDP auto-discovery does not work (e.g. across subnets).
	Address string `json:"address"`
}

// CalibreSendProgress is emitted once per book while sending to the device.
type CalibreSendProgress struct {
	Total int    `json:"total"`
	Done  int    `json:"done"`
	Title string `json:"title"`
	Ok    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// SendResult summarizes a send-to-device run.
type SendResult struct {
	Sent           int    `json:"sent"`
	Failed         int    `json:"failed"`
	InventorySent  bool   `json:"inventorySent"`
	InventoryError string `json:"inventoryError,omitempty"`
}

// DeviceBookState tells the UI whether a local book is known to be on the
// currently connected device according to the last `.reliure` inventory.
type DeviceBookState struct {
	BookID     int64  `json:"bookId"`
	Status     string `json:"status"` // "unknown" | "absent" | "present"
	RemotePath string `json:"remotePath,omitempty"`
	SentAt     string `json:"sentAt,omitempty"`
}

// startFromSettings starts the push server at boot if the preference says so.
func (s *CalibreService) startFromSettings() {
	if s.settings.Get().CalibreEnabled {
		_ = s.server.Start()
	}
}

// shutdown stops the push server.
func (s *CalibreService) shutdown() { _ = s.server.Stop() }

// Status returns the current push-server and connection state.
func (s *CalibreService) Status() CalibreStatus {
	name, connected := s.server.Device()
	st := CalibreStatus{
		Running:   s.server.Running(),
		Connected: connected,
		Device:    name,
		Port:      s.server.Port(),
	}
	if st.Running && st.Port > 0 {
		host := firstLANIPv4()
		if host == "" {
			host = "127.0.0.1"
		}
		st.Address = fmt.Sprintf("%s:%d", host, st.Port)
	}
	return st
}

// BookStates returns per-book presence information for the connected device.
func (s *CalibreService) BookStates(ids []int64) ([]DeviceBookState, error) {
	if s.inventory == nil {
		return unknownStates(ids), nil
	}
	deviceName, connected := s.server.Device()
	if !connected {
		return unknownStates(ids), nil
	}
	inv, err := s.inventory.Load(deviceName)
	if err != nil {
		return nil, err
	}
	byBook := inv.ByBookID()
	out := make([]DeviceBookState, 0, len(ids))
	for _, id := range ids {
		state := DeviceBookState{BookID: id, Status: "absent"}
		if e, ok := byBook[id]; ok {
			state.Status = "present"
			state.RemotePath = e.RemotePath
			if !e.SentAt.IsZero() {
				state.SentAt = e.SentAt.UTC().Format(time.RFC3339)
			}
		}
		out = append(out, state)
	}
	return out, nil
}

// SetEnabled starts or stops the push server and persists the preference.
func (s *CalibreService) SetEnabled(enabled bool) (CalibreStatus, error) {
	cfg := s.settings.Get()
	cfg.CalibreEnabled = enabled
	if _, err := s.settings.Update(cfg); err != nil {
		return s.Status(), err
	}
	if enabled {
		if err := s.server.Start(); err != nil {
			return s.Status(), err
		}
	} else {
		_ = s.server.Stop()
	}
	return s.Status(), nil
}

// SendBooks pushes the given books to the connected device, honoring each
// book's KOReader remote-path template/override (which may include subfolders).
// Progress is emitted per book as "calibre:progress".
func (s *CalibreService) SendBooks(ids []int64) (SendResult, error) {
	sess := s.server.Session()
	if sess == nil {
		return SendResult{}, errors.New("aucune liseuse connectée")
	}
	deviceName, _ := s.server.Device()
	tmpl := s.settings.Get().RemotePathTemplate
	total := len(ids)
	var res SendResult
	var sentEntries []device.Entry
	slog.Info("calibre: send starting", "books", total, "device", deviceName)

	for i, id := range ids {
		prog := CalibreSendProgress{Total: total, Done: i + 1}
		b, err := s.db.Books.ByID(id)
		if err != nil {
			res.Failed++
			prog.Error = err.Error()
			slog.Warn("calibre: send failed", "book", id, "err", err)
			application.Get().Event.Emit(calibreProgressEvent, prog)
			continue
		}
		prog.Title = b.Title

		file := sendableFile(b)
		if file == nil {
			res.Failed++
			prog.Error = "aucun fichier envoyable"
			slog.Warn("calibre: send skipped (no file)", "title", b.Title)
			application.Get().Event.Emit(calibreProgressEvent, prog)
			continue
		}
		lpath := remoteLpath(tmpl, b, file.Format)
		slog.Info("calibre: sending",
			"title", b.Title,
			"from", file.Path,
			"to", lpath,
			"folder", path.Dir(lpath),
			"format", file.Format,
			"bytes", file.Size,
			"n", fmt.Sprintf("%d/%d", i+1, total))
		finalLpath, err := sess.SendBook(b, file.Path, lpath, i+1, total)
		if err != nil {
			res.Failed++
			prog.Error = err.Error()
			slog.Warn("calibre: send failed", "title", b.Title, "from", file.Path, "to", lpath, "err", err)
		} else {
			res.Sent++
			prog.Ok = true
			sentEntries = append(sentEntries, inventoryEntry(b, *file, finalLpath))
			slog.Info("calibre: sent", "title", b.Title, "from", file.Path, "to", finalLpath, "bytes", file.Size)
		}
		application.Get().Event.Emit(calibreProgressEvent, prog)
	}
	if len(sentEntries) > 0 {
		if err := s.updateAndSendInventory(sess, deviceName, sentEntries); err != nil {
			res.InventoryError = err.Error()
			slog.Warn("calibre: inventory update failed", "device", deviceName, "err", err)
		} else {
			res.InventorySent = true
		}
	}
	slog.Info("calibre: send finished", "sent", res.Sent, "failed", res.Failed, "device", deviceName)
	return res, nil
}

func (s *CalibreService) updateAndSendInventory(sess *calibre.Session, deviceName string, entries []device.Entry) error {
	if s.inventory == nil {
		return nil
	}
	inv, err := s.inventory.Load(deviceName)
	if err != nil {
		return err
	}
	inv.Upsert(entries...)
	if err := s.inventory.Save(inv); err != nil {
		return err
	}
	data, err := device.MarshalDeviceFile(inv)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp("", "reliure-inventory-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	_, err = sess.SendFile(
		tmpPath,
		device.InventoryFilename,
		nil,
		1,
		1,
	)
	return err
}

func inventoryEntry(b *core.Book, f core.File, remotePath string) device.Entry {
	return device.Entry{
		BookID:     b.ID,
		FileID:     f.ID,
		RemotePath: remotePath,
		Format:     strings.ToLower(f.Format),
		Size:       f.Size,
		SHA256:     f.SHA256,
		SentAt:     time.Now().UTC(),
		Title:      b.Title,
		Authors:    b.AuthorNames(),
	}
}

func unknownStates(ids []int64) []DeviceBookState {
	out := make([]DeviceBookState, 0, len(ids))
	for _, id := range ids {
		out = append(out, DeviceBookState{BookID: id, Status: "unknown"})
	}
	return out
}

// sendableFile picks the file to send: EPUB is preferred, otherwise the first.
func sendableFile(b *core.Book) *core.File {
	for i := range b.Files {
		if strings.EqualFold(b.Files[i].Format, "epub") {
			return &b.Files[i]
		}
	}
	if len(b.Files) > 0 {
		return &b.Files[0]
	}
	return nil
}

// remoteLpath computes the device path, ensuring it ends with the file's
// extension so KOReader stores it with the right type.
func remoteLpath(tmpl string, b *core.Book, format string) string {
	p := remotePath(tmpl, b) // reused from catalog.go; honors per-book override
	ext := "." + strings.ToLower(format)
	if !strings.EqualFold(path.Ext(p), ext) {
		p += ext
	}
	return filepath.ToSlash(p)
}

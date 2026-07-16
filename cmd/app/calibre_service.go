package main

import (
	"encoding/json"
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
	"github.com/agrison/reliure/internal/koreader"
	"github.com/agrison/reliure/internal/koreaderstats"
	"github.com/agrison/reliure/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Frontend event names for the Calibre wireless push server.
const (
	calibreStatusEvent   = "calibre:status"
	calibreProgressEvent = "calibre:progress"
	readingStatsEvent    = "reading:statsUpdated"
)

func init() {
	application.RegisterEvent[CalibreStatus](calibreStatusEvent)
	application.RegisterEvent[CalibreSendProgress](calibreProgressEvent)
	application.RegisterEvent[ReadingStatsFetch](readingStatsEvent)
}

// CalibreService controls the Calibre wireless (push) server that lets KOReader
// connect over WiFi, and sends books to the connected device.
type CalibreService struct {
	db               *core.DB
	settings         *settings.Store
	server           *calibre.Server
	inventory        *device.Store
	readingStatsPath string // where fetched reading stats are cached (JSON)
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
	// LastDevice is the connected device name, or the last one seen when
	// disconnected. It signals the UI that cached on-device presence is available
	// (the `.reliure` inventory) even without a live connection.
	LastDevice string `json:"lastDevice"`
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
	last := s.settings.Get().LastDeviceName
	if connected && name != "" {
		last = name
	}
	st := CalibreStatus{
		Running:    s.server.Running(),
		Connected:  connected,
		Device:     name,
		LastDevice: last,
		Port:       s.server.Port(),
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

// rememberDevice persists the name of a device that just connected, so the UI
// can show its cached inventory after it disconnects.
func (s *CalibreService) rememberDevice(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	cfg := s.settings.Get()
	if cfg.LastDeviceName == name {
		return
	}
	cfg.LastDeviceName = name
	if _, err := s.settings.Update(cfg); err != nil {
		slog.Warn("calibre: remember device failed", "device", name, "err", err)
	}
}

// BookStates returns per-book presence information for the connected device, or
// — when disconnected — for the last device seen, so the UI keeps showing which
// books are on the reader from cache. Statuses refresh on the next connection.
func (s *CalibreService) BookStates(ids []int64) ([]DeviceBookState, error) {
	if s.inventory == nil {
		return unknownStates(ids), nil
	}
	deviceName, connected := s.server.Device()
	if !connected {
		deviceName = s.settings.Get().LastDeviceName
	}
	if strings.TrimSpace(deviceName) == "" {
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

// SyncReadingFromDevice pulls KOReader reading progress and annotations over the
// live Calibre connection — no USB needed. For every book Reliure has sent (per
// the `.reliure` inventory) it requests the book's `.sdr` sidecar by lpath via
// GET_BOOK_FILE_SEGMENT, parses it and stores progress + annotations. The
// inventory maps each lpath to a book id, so matching is exact.
func (s *CalibreService) SyncReadingFromDevice() (KoreaderSyncResult, error) {
	var res KoreaderSyncResult
	sess := s.server.Session()
	if sess == nil {
		return res, errors.New("aucune liseuse connectée")
	}
	deviceName, _ := s.server.Device()
	res.Dir = deviceName
	if s.inventory == nil {
		return res, errors.New("inventaire liseuse indisponible")
	}
	inv, err := s.inventory.Load(deviceName)
	if err != nil {
		return res, err
	}

	for _, e := range inv.Entries {
		data, ok := s.fetchSidecar(sess, e)
		if !ok {
			res.Unmatched++ // on the device but not opened/read yet
			continue
		}
		res.Scanned++
		sc, err := koreader.Parse(data)
		if err != nil {
			slog.Warn("calibre: parse sidecar failed", "book", e.BookID, "err", err)
			continue
		}
		if err := s.applyDeviceReading(e.BookID, deviceName, sc, &res); err != nil {
			return res, err
		}
	}
	slog.Info("calibre: reading sync from device", "device", deviceName,
		"read", res.Scanned, "annotations", res.Annotations, "unread", res.Unmatched)
	return res, nil
}

// ReadingStatsProbe reports whether KOReader's statistics database could be
// fetched over the Calibre connection, and from which relative path.
type ReadingStatsProbe struct {
	Found bool   `json:"found"`
	Lpath string `json:"lpath"` // the candidate path that returned the file
	Bytes int64  `json:"bytes"`
	Valid bool   `json:"valid"` // true when the bytes start with the SQLite magic
	Tried int    `json:"tried"`
}

// ProbeReadingStats tries to fetch KOReader's `statistics.sqlite3` over the live
// connection. It lives under `koreader/settings/`, OUTSIDE the Calibre inbox, so
// we probe a handful of relative paths (KOReader's GET_BOOK_FILE_SEGMENT resolves
// them under the inbox with no sanitising, so `..` traversal may reach it). This
// confirms feasibility on real hardware before we build the full stats feature.
func (s *CalibreService) ProbeReadingStats() (ReadingStatsProbe, error) {
	sess := s.server.Session()
	if sess == nil {
		return ReadingStatsProbe{}, errors.New("aucune liseuse connectée")
	}
	var res ReadingStatsProbe
	for _, lp := range statisticsCandidates() {
		res.Tried++
		data, ok, err := sess.GetFile(lp)
		if err != nil {
			slog.Warn("calibre: stats probe failed", "lpath", lp, "err", err)
			continue
		}
		if !ok {
			continue
		}
		res.Found = true
		res.Lpath = lp
		res.Bytes = int64(len(data))
		res.Valid = looksLikeSQLite(data)
		slog.Info("calibre: stats probe hit", "lpath", lp, "bytes", res.Bytes, "valid", res.Valid)
		return res, nil
	}
	slog.Info("calibre: stats probe found nothing", "tried", res.Tried)
	return res, nil
}

// ReadingStatsFetch summarizes a fetch of the statistics database.
type ReadingStatsFetch struct {
	Found        bool   `json:"found"`
	Lpath        string `json:"lpath"`
	Bytes        int64  `json:"bytes"`
	Tried        int    `json:"tried"`
	Parsed       bool   `json:"parsed"`
	TotalSeconds int64  `json:"totalSeconds"`
	DaysRead     int    `json:"daysRead"`
	Error        string `json:"error,omitempty"`
}

// FetchReadingStats fetches KOReader's statistics database over the live
// connection (dynamic path search), computes the reading aggregates and caches
// them so the dashboard can show them offline. Refreshed each time it runs.
func (s *CalibreService) FetchReadingStats() (ReadingStatsFetch, error) {
	sess := s.server.Session()
	if sess == nil {
		return ReadingStatsFetch{}, errors.New("aucune liseuse connectée")
	}
	var res ReadingStatsFetch
	var data []byte
	for _, lp := range statisticsCandidates() {
		res.Tried++
		d, ok, err := sess.GetFile(lp)
		if err != nil {
			slog.Warn("calibre: stats fetch failed", "lpath", lp, "err", err)
			continue
		}
		if ok {
			res.Found, res.Lpath, res.Bytes, data = true, lp, int64(len(d)), d
			break
		}
	}
	if !res.Found {
		slog.Info("calibre: stats not found", "tried", res.Tried)
		return res, nil
	}
	if !looksLikeSQLite(data) {
		res.Error = "le fichier récupéré n'est pas une base SQLite"
		return res, nil
	}

	tmp, err := os.CreateTemp("", "koreader-stats-*.sqlite3")
	if err != nil {
		return res, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return res, err
	}
	if err := tmp.Close(); err != nil {
		return res, err
	}

	stats, err := koreaderstats.Read(tmpPath)
	if err != nil {
		res.Error = err.Error()
		return res, nil
	}
	stats.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.saveReadingStats(stats); err != nil {
		res.Error = err.Error()
		return res, nil
	}
	res.Parsed = true
	res.TotalSeconds = stats.TotalSeconds
	res.DaysRead = stats.DaysRead
	slog.Info("calibre: reading stats fetched", "lpath", res.Lpath, "seconds", stats.TotalSeconds, "days", stats.DaysRead)
	application.Get().Event.Emit(readingStatsEvent, res) // refresh an open dashboard
	return res, nil
}

// autoFetchReadingStats fetches the reading statistics right after a device
// connects, when the feature is enabled. Runs in the background so it never
// delays the connection; a short pause lets the session settle first.
func (s *CalibreService) autoFetchReadingStats() {
	if !s.settings.Get().ReadingStatsEnabled {
		return
	}
	time.Sleep(2 * time.Second)
	if s.server.Session() == nil {
		return // disconnected in the meantime
	}
	if _, err := s.FetchReadingStats(); err != nil {
		slog.Warn("calibre: auto reading-stats fetch failed", "err", err)
	}
}

// saveReadingStats caches the computed stats as JSON for the dashboard.
func (s *CalibreService) saveReadingStats(stats *koreaderstats.ReadingStats) error {
	if s.readingStatsPath == "" {
		return nil
	}
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.readingStatsPath), 0o755); err != nil {
		return err
	}
	tmp := s.readingStatsPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.readingStatsPath)
}

// statisticsCandidates are the relative paths under which KOReader's statistics
// database might sit, depending on where the user pointed the Calibre inbox.
func statisticsCandidates() []string {
	const f = "statistics.sqlite3"
	return []string{
		"settings/" + f,                   // inbox == koreader dir
		"../settings/" + f,                // inbox is a subdir of koreader (e.g. clipboard)
		"../../settings/" + f,             // deeper subdir
		"koreader/settings/" + f,          // inbox is the mount root
		"../koreader/settings/" + f,       // inbox is a sibling of koreader (e.g. documents)
		"../../koreader/settings/" + f,    // one level deeper
		"../../../koreader/settings/" + f, // two levels deeper
	}
}

// looksLikeSQLite checks the 16-byte SQLite file header.
func looksLikeSQLite(data []byte) bool {
	const magic = "SQLite format 3\x00"
	return len(data) >= len(magic) && string(data[:len(magic)]) == magic
}

// fetchSidecar tries the KOReader sidecar path(s) for a sent book and returns
// the first that exists on the device.
func (s *CalibreService) fetchSidecar(sess *calibre.Session, e device.Entry) ([]byte, bool) {
	for _, lp := range sidecarLpaths(e.RemotePath, e.Format) {
		data, ok, err := sess.GetFile(lp)
		if err != nil {
			slog.Warn("calibre: get sidecar failed", "lpath", lp, "err", err)
			continue
		}
		if ok {
			return data, true
		}
	}
	return nil, false
}

func (s *CalibreService) applyDeviceReading(bookID int64, deviceName string, sc *koreader.Sidecar, res *KoreaderSyncResult) error {
	if err := s.db.Reading.MergeDeviceState(core.ReadingState{
		BookID:     bookID,
		Percent:    sc.PercentFinished,
		Pages:      sc.TotalPages,
		Status:     string(sc.Status),
		Device:     deviceName,
		LastReadAt: sc.ModifiedAt,
		Rating:     sc.Rating,
	}); err != nil {
		return err
	}
	res.Matched++
	if sc.PercentFinished > 0 || sc.Status != "" {
		res.WithProgress++
	}
	anns := make([]core.Annotation, 0, len(sc.Annotations))
	for _, a := range sc.Annotations {
		anns = append(anns, core.Annotation{
			BookID: bookID, Text: a.Text, Note: a.Note,
			Chapter: a.Chapter, Drawer: a.Drawer, CreatedAt: a.Datetime,
		})
	}
	if err := s.db.Reading.ReplaceAnnotations(bookID, anns); err != nil {
		return err
	}
	res.Annotations += len(anns)
	return nil
}

// sidecarLpaths builds the candidate KOReader sidecar paths for a book's remote
// path. KOReader's default (getSidecarDir) strips the extension — "Book.epub" →
// "Book.sdr/metadata.epub.lua"; the extension-kept form is tried as a fallback.
func sidecarLpaths(remotePath, format string) []string {
	rp := filepath.ToSlash(remotePath)
	ext := path.Ext(rp)
	format = strings.ToLower(strings.TrimPrefix(format, "."))
	if format == "" {
		format = strings.TrimPrefix(strings.ToLower(ext), ".")
	}
	meta := "metadata." + format + ".lua"
	stem := strings.TrimSuffix(rp, ext)
	return []string{
		stem + ".sdr/" + meta, // extension stripped (KOReader default)
		rp + ".sdr/" + meta,   // extension kept (older/alternate layouts)
	}
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

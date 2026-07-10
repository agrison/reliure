// Package device stores Reliure's per-reader inventory manifest. The manifest
// is written both to the local config cache and to the device as `.reliure`.
package device

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// InventoryFilename is the on-device manifest filename.
	InventoryFilename = ".reliure"
	// SchemaVersion is bumped only for incompatible manifest shape changes.
	SchemaVersion = 1
)

// Inventory is the versioned JSON document stored as `.reliure`.
type Inventory struct {
	SchemaVersion int       `json:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at"`
	DeviceName    string    `json:"device_name,omitempty"`
	Entries       []Entry   `json:"entries"`
}

// Entry records one file Reliure has successfully sent to a device.
type Entry struct {
	BookID     int64     `json:"book_id"`
	FileID     int64     `json:"file_id"`
	RemotePath string    `json:"remote_path"`
	Format     string    `json:"format"`
	Size       int64     `json:"size"`
	SHA256     string    `json:"sha256,omitempty"`
	SentAt     time.Time `json:"sent_at"`
	Title      string    `json:"title"`
	Authors    []string  `json:"authors,omitempty"`
}

// Store persists one inventory file per device in Reliure's config directory.
type Store struct {
	dir string
}

// NewStore creates a store rooted at dir.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Load returns the known inventory for a device. Missing inventories return an
// empty document; corrupt JSON is reported so callers can surface/log it.
func (s *Store) Load(deviceName string) (*Inventory, error) {
	path := s.path(deviceName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return newInventory(deviceName), nil
	}
	if err != nil {
		return nil, err
	}
	var inv Inventory
	if err := json.Unmarshal(data, &inv); err != nil {
		return nil, err
	}
	if inv.SchemaVersion == 0 {
		inv.SchemaVersion = SchemaVersion
	}
	if inv.DeviceName == "" {
		inv.DeviceName = deviceName
	}
	return &inv, nil
}

// Save atomically writes an inventory to the local cache.
func (s *Store) Save(inv *Inventory) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	inv.SchemaVersion = SchemaVersion
	inv.GeneratedAt = time.Now().UTC()
	sortEntries(inv.Entries)
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	path := s.path(inv.DeviceName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// MarshalDeviceFile returns the JSON bytes to send to the device.
func MarshalDeviceFile(inv *Inventory) ([]byte, error) {
	cp := *inv
	cp.SchemaVersion = SchemaVersion
	cp.GeneratedAt = time.Now().UTC()
	sortEntries(cp.Entries)
	return json.MarshalIndent(cp, "", "  ")
}

// Upsert merges entries by file id when present, otherwise by remote path.
func (inv *Inventory) Upsert(entries ...Entry) {
	if inv.SchemaVersion == 0 {
		inv.SchemaVersion = SchemaVersion
	}
	index := make(map[string]int, len(inv.Entries))
	for i, e := range inv.Entries {
		index[entryKey(e)] = i
	}
	for _, e := range entries {
		if e.SentAt.IsZero() {
			e.SentAt = time.Now().UTC()
		}
		key := entryKey(e)
		if i, ok := index[key]; ok {
			inv.Entries[i] = e
			continue
		}
		index[key] = len(inv.Entries)
		inv.Entries = append(inv.Entries, e)
	}
}

// ByBookID indexes entries by local book id.
func (inv *Inventory) ByBookID() map[int64]Entry {
	out := make(map[int64]Entry, len(inv.Entries))
	for _, e := range inv.Entries {
		out[e.BookID] = e
	}
	return out
}

func newInventory(deviceName string) *Inventory {
	return &Inventory{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		DeviceName:    deviceName,
	}
}

func (s *Store) path(deviceName string) string {
	return filepath.Join(s.dir, safeDeviceName(deviceName)+".json")
}

func entryKey(e Entry) string {
	if e.FileID > 0 {
		return "file:" + strconv.FormatInt(e.FileID, 10)
	}
	return "path:" + strings.ToLower(e.RemotePath)
}

func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].RemotePath == entries[j].RemotePath {
			return entries[i].FileID < entries[j].FileID
		}
		return strings.ToLower(entries[i].RemotePath) < strings.ToLower(entries[j].RemotePath)
	})
}

func safeDeviceName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unknown-device"
	}
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		if r < 0x20 {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" {
		return "unknown-device"
	}
	return name
}

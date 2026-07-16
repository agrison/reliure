package main

import (
	"path/filepath"
	"testing"

	"github.com/agrison/reliure/internal/calibre"
	"github.com/agrison/reliure/internal/device"
	"github.com/agrison/reliure/internal/settings"
)

// When no reader is connected but one was seen before, BookStates must still
// report on-device presence from the cached `.reliure` inventory.
func TestBookStatesUsesCachedDeviceWhenDisconnected(t *testing.T) {
	dir := t.TempDir()
	store, err := settings.Open(filepath.Join(dir, "settings.json"), filepath.Join(dir, "lib"))
	if err != nil {
		t.Fatal(err)
	}

	inv := device.NewStore(filepath.Join(dir, "devices"))
	manifest := &device.Inventory{DeviceName: "KOReader Kindle"}
	manifest.Upsert(device.Entry{BookID: 42, RemotePath: "Author/Book.epub", Format: "epub"})
	if err := inv.Save(manifest); err != nil {
		t.Fatal(err)
	}

	svc := &CalibreService{
		settings:  store,
		inventory: inv,
		// A server that is not started → Device() reports disconnected.
		server: calibre.NewServer(calibre.ServerConfig{}),
	}

	// Without a remembered device: everything is unknown.
	states, err := svc.BookStates([]int64{42, 7})
	if err != nil {
		t.Fatal(err)
	}
	if states[0].Status != "unknown" {
		t.Fatalf("without cache, status = %q, want unknown", states[0].Status)
	}

	// Remember the device (as OnConnect would), then re-query while disconnected.
	svc.rememberDevice("KOReader Kindle")
	states, err = svc.BookStates([]int64{42, 7})
	if err != nil {
		t.Fatal(err)
	}
	if states[0].Status != "present" {
		t.Errorf("book 42 status = %q, want present (from cache)", states[0].Status)
	}
	if states[1].Status != "absent" {
		t.Errorf("book 7 status = %q, want absent", states[1].Status)
	}

	// Status() exposes the last device so the UI can offer the filter offline.
	if lg := svc.Status().LastDevice; lg != "KOReader Kindle" {
		t.Errorf("Status().LastDevice = %q, want the remembered device", lg)
	}
}

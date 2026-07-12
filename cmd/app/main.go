// Command app is the Wails v3 desktop entry point for Reliure. It wires the
// embedded frontend to the Go services and opens the main window; the domain
// logic it exposes lives in the framework-agnostic internal/ packages.
package main

import (
	"context"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/agrison/reliure/frontend"
	"github.com/agrison/reliure/internal/calibre"
	"github.com/agrison/reliure/internal/core"
	"github.com/agrison/reliure/internal/device"
	"github.com/agrison/reliure/internal/gutenberg"
	"github.com/agrison/reliure/internal/metadata"
	"github.com/agrison/reliure/internal/opds"
	"github.com/agrison/reliure/internal/settings"
	"github.com/agrison/reliure/internal/standardebooks"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	// Register format handlers into formats.Default.
	_ "github.com/agrison/reliure/internal/formats/epub"
	_ "github.com/agrison/reliure/internal/formats/pdf"
)

func main() {
	// Database, cover cache and settings live under the OS config dir.
	configDir, err := core.ConfigDir()
	if err != nil {
		log.Fatalf("config dir: %v", err)
	}
	dbPath, err := core.DefaultDBPath()
	if err != nil {
		log.Fatalf("db path: %v", err)
	}
	db, err := core.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	// Preferences (import mode, managed-library location). The default library
	// dir sits under the config dir until the user picks another.
	store, err := settings.Open(
		filepath.Join(configDir, "settings.json"),
		filepath.Join(configDir, "Library"),
	)
	if err != nil {
		log.Fatalf("open settings: %v", err)
	}
	coverDir := filepath.Join(configDir, "covers")

	libSvc := &LibraryService{
		db:        db,
		settings:  store,
		coverDir:  coverDir,
		meta:      metadata.NewClient(),
		gutenberg: gutenberg.NewCatalog(filepath.Join(configDir, "gutenberg", "pg_catalog.csv")),
		standard:  standardebooks.NewCatalog(filepath.Join(configDir, "standardebooks", "repos.json")),
	}
	watchFolderSvc := &WatchFolderService{store: store, library: libSvc}
	watchFolderSvc.startFromSettings()
	defer watchFolderSvc.shutdown()

	opdsServer := opds.NewServer(opds.NewHandler(opds.HandlerConfig{
		Catalog:  opds.CoreCatalog{DB: db},
		CoverDir: coverDir,
		Title:    "Reliure",
	}))
	opdsSvc := &OPDSService{store: store, server: opdsServer}
	opdsSvc.startFromSettings()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = opdsSvc.shutdown(ctx)
	}()

	// Calibre wireless (push) server. Connection changes are pushed to the UI
	// as "calibre:status" events (full status, incl. port/address) so the
	// sidebar reflects the connected reader.
	inventoryStore := device.NewStore(filepath.Join(configDir, "devices"))
	calibreSvc := &CalibreService{
		db:        db,
		settings:  store,
		inventory: inventoryStore,
	}
	libSvc.calibre = calibreSvc
	emitCalibreStatus := func() {
		application.Get().Event.Emit(calibreStatusEvent, calibreSvc.Status())
	}
	calibreSvc.server = calibre.NewServer(calibre.ServerConfig{
		LibraryName:  "Reliure",
		LibraryUUID:  "reliure-library",
		OnConnect:    func(string) { emitCalibreStatus() },
		OnDisconnect: func() { emitCalibreStatus() },
	})
	calibreSvc.startFromSettings()
	defer calibreSvc.shutdown()

	app := application.New(application.Options{
		Name:        "Reliure",
		Description: "Bibliothèque ebook multiplateforme",
		Services: []application.Service{
			application.NewService(&App{}),
			application.NewService(libSvc),
			application.NewService(&SettingsService{store: store, library: libSvc}),
			application.NewService(watchFolderSvc),
			application.NewService(opdsSvc),
			application.NewService(calibreSvc),
			application.NewService(&KOReaderService{db: db, store: store}),
			application.NewService(&StatsService{db: db, inventory: inventoryStore, settings: store}),
		},
		Assets: application.AssetOptions{
			Handler: assetHandler(coverDir),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:          "Reliure",
		Width:          1180,
		Height:         760,
		MinWidth:       880,
		MinHeight:      560,
		EnableFileDrop: true,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(14, 15, 19),
		URL:              "/",
	})

	// Drag-and-drop import: books dropped on the window are imported in the
	// background (progress + completion are pushed to the frontend as events).
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		files := e.Context().DroppedFiles()
		if len(files) == 0 {
			return
		}
		go libSvc.ImportPaths(files)
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// assetHandler serves cached cover thumbnails from coverDir under /covers/, and
// delegates everything else to the embedded frontend. Covers are served as
// files (by URL), never inlined into JSON. http.Dir prevents path traversal.
func assetHandler(coverDir string) http.Handler {
	frontendFS := application.AssetFileServerFS(frontend.Assets)
	covers := http.StripPrefix(coverURLPrefix, http.FileServer(http.Dir(coverDir)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, coverURLPrefix) {
			covers.ServeHTTP(w, r)
			return
		}
		frontendFS.ServeHTTP(w, r)
	})
}

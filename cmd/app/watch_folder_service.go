package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/agrison/reliure/internal/library"
	"github.com/agrison/reliure/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const watchFolderStatusEvent = "watch-folder:status"

func init() {
	application.RegisterEvent[WatchFolderStatus](watchFolderStatusEvent)
}

// WatchFolderService imports new ebook files dropped in a configured folder.
// It intentionally reuses LibraryService.ImportPaths so automatic imports follow
// the same mode, deduplication, cover cache and UI events as manual imports.
type WatchFolderService struct {
	store   *settings.Store
	library *LibraryService

	mu        sync.Mutex
	cancel    context.CancelFunc
	running   bool
	lastError string
}

type WatchFolderStatus struct {
	Enabled      bool   `json:"enabled"`
	Running      bool   `json:"running"`
	Dir          string `json:"dir"`
	DelaySeconds int    `json:"delaySeconds"`
	DeleteSource bool   `json:"deleteSource"`
	LastError    string `json:"lastError,omitempty"`
}

type watchedFile struct {
	size    int64
	modTime time.Time
	since   time.Time
}

func (s *WatchFolderService) startFromSettings() {
	if cfg := s.store.Get(); cfg.WatchFolderEnabled {
		s.restart(cfg)
	}
}

func (s *WatchFolderService) shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

func (s *WatchFolderService) Status() WatchFolderStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

func (s *WatchFolderService) ChooseFolder() (AppSettings, error) {
	dir, err := application.Get().Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		SetTitle("Choisir le dossier surveillé").
		PromptForSingleSelection()
	if err != nil || dir == "" {
		return toAppSettings(s.store.Get()), err
	}
	cur := s.store.Get()
	cur.WatchFolderDir = dir
	cur.WatchFolderEnabled = true
	next, err := s.store.Update(cur)
	if err == nil {
		s.restart(next)
	}
	return toAppSettings(next), err
}

func (s *WatchFolderService) SetEnabled(enabled bool) (AppSettings, error) {
	cur := s.store.Get()
	cur.WatchFolderEnabled = enabled
	next, err := s.store.Update(cur)
	if err == nil {
		s.restart(next)
	}
	return toAppSettings(next), err
}

func (s *WatchFolderService) SetDelaySeconds(delay int) (AppSettings, error) {
	cur := s.store.Get()
	cur.WatchFolderDelaySeconds = delay
	next, err := s.store.Update(cur)
	if err == nil {
		s.restart(next)
	}
	return toAppSettings(next), err
}

func (s *WatchFolderService) SetDeleteSource(enabled bool) (AppSettings, error) {
	cur := s.store.Get()
	cur.WatchFolderDeleteSource = enabled
	next, err := s.store.Update(cur)
	if err == nil {
		s.emitStatus()
	}
	return toAppSettings(next), err
}

func (s *WatchFolderService) ClearFolder() (AppSettings, error) {
	cur := s.store.Get()
	cur.WatchFolderEnabled = false
	cur.WatchFolderDir = ""
	next, err := s.store.Update(cur)
	if err == nil {
		s.restart(next)
	}
	return toAppSettings(next), err
}

func (s *WatchFolderService) restart(cfg settings.Settings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
	s.lastError = ""
	if !cfg.WatchFolderEnabled || cfg.WatchFolderDir == "" {
		s.emitStatusLocked()
		return
	}
	if info, err := os.Stat(cfg.WatchFolderDir); err != nil || !info.IsDir() {
		if err != nil {
			s.lastError = err.Error()
		} else {
			s.lastError = "le chemin n'est pas un dossier"
		}
		s.emitStatusLocked()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true
	s.emitStatusLocked()
	go s.run(ctx, cfg)
}

func (s *WatchFolderService) stopLocked() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.running = false
}

func (s *WatchFolderService) run(ctx context.Context, cfg settings.Settings) {
	pending := make(map[string]watchedFile)
	processed := make(map[string]watchedFile)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	s.scan(ctx, cfg, pending, processed)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scan(ctx, cfg, pending, processed)
		}
	}
}

func (s *WatchFolderService) scan(ctx context.Context, cfg settings.Settings, pending, processed map[string]watchedFile) {
	imp := library.New(s.library.db, library.Config{
		Mode:       cfg.ImportMode,
		LibraryDir: cfg.LibraryDir,
		CoverDir:   s.library.coverDir,
		Merge:      true,
	})
	files, err := imp.Scan(cfg.WatchFolderDir)
	if err != nil {
		s.setError(err)
		return
	}
	now := time.Now()
	for _, path := range files {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if cfg.ImportMode == library.ModeCopy && managedPath(path, cfg.LibraryDir) {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		current := watchedFile{size: info.Size(), modTime: info.ModTime()}
		if sameWatchedFile(processed[path], current) {
			continue
		}
		prev, ok := pending[path]
		if !ok || !sameWatchedFile(prev, current) {
			current.since = now
			pending[path] = current
			continue
		}
		if now.Sub(prev.since) < time.Duration(cfg.WatchFolderDelaySeconds)*time.Second {
			continue
		}
		delete(pending, path)
		s.importWatchedFile(cfg, path)
		current.since = now
		processed[path] = current
	}
}

func (s *WatchFolderService) importWatchedFile(cfg settings.Settings, path string) {
	log.Printf("WatchFolder: importing %s", path)
	sum, err := s.library.ImportPaths([]string{path})
	if err != nil {
		s.setError(err)
		log.Printf("WatchFolder: import failed %s: %v", path, err)
		return
	}
	if cfg.ImportMode == library.ModeCopy && cfg.WatchFolderDeleteSource && sum.Failed == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			s.setError(err)
			log.Printf("WatchFolder: delete source failed %s: %v", path, err)
		}
	}
}

func sameWatchedFile(a, b watchedFile) bool {
	return a.size == b.size && a.modTime.Equal(b.modTime)
}

func (s *WatchFolderService) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.lastError = err.Error()
	}
	s.emitStatusLocked()
}

func (s *WatchFolderService) emitStatus() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emitStatusLocked()
}

func (s *WatchFolderService) emitStatusLocked() {
	application.Get().Event.Emit(watchFolderStatusEvent, s.statusLocked())
}

func (s *WatchFolderService) statusLocked() WatchFolderStatus {
	cfg := s.store.Get()
	dir := ""
	if cfg.WatchFolderDir != "" {
		dir = filepath.Clean(cfg.WatchFolderDir)
	}
	return WatchFolderStatus{
		Enabled:      cfg.WatchFolderEnabled,
		Running:      s.running,
		Dir:          dir,
		DelaySeconds: cfg.WatchFolderDelaySeconds,
		DeleteSource: cfg.WatchFolderDeleteSource,
		LastError:    s.lastError,
	}
}

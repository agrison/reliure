package main

import (
	"github.com/agrison/reliure/internal/library"
	"github.com/agrison/reliure/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// SettingsService exposes user preferences to the frontend. The key preference
// is the import mode: "copy" (Reliure manages copies in its library folder) or
// "reference" (files are indexed where they already live).
type SettingsService struct {
	store   *settings.Store
	library *LibraryService
}

// AppSettings is the frontend-facing settings shape (flat, JSON-friendly).
type AppSettings struct {
	ImportMode           string `json:"importMode"` // "copy" | "reference"
	LibraryDir           string `json:"libraryDir"`
	RemotePathTemplate   string `json:"remotePathTemplate"`
	OPDSEnabled          bool   `json:"opdsEnabled"`
	OPDSPort             int    `json:"opdsPort"`
	WriteMetadataToFile  bool   `json:"writeMetadataToFile"`
	Theme                string `json:"theme"`
	Language             string `json:"language"`
	KoreaderSyncDir      string `json:"koreaderSyncDir"`
	FeatureDiscover      bool   `json:"featureDiscover"`
	FeatureSmartShelves  bool   `json:"featureSmartShelves"`
	WatchFolderEnabled   bool   `json:"watchFolderEnabled"`
	WatchFolderDir       string `json:"watchFolderDir"`
	WatchFolderDelay     int    `json:"watchFolderDelaySeconds"`
	WatchFolderDelete    bool   `json:"watchFolderDeleteSource"`
	ContentSearchEnabled bool   `json:"contentSearchEnabled"`
	ContentSearchContext string `json:"contentSearchContext"`
}

func toAppSettings(s settings.Settings) AppSettings {
	return AppSettings{
		ImportMode:           string(s.ImportMode),
		LibraryDir:           s.LibraryDir,
		RemotePathTemplate:   s.RemotePathTemplate,
		OPDSEnabled:          s.OPDSEnabled,
		OPDSPort:             s.OPDSPort,
		WriteMetadataToFile:  s.WriteMetadataToFile,
		Theme:                s.Theme,
		Language:             s.Language,
		KoreaderSyncDir:      s.KoreaderSyncDir,
		FeatureDiscover:      s.FeatureDiscover,
		FeatureSmartShelves:  s.FeatureSmartShelves,
		WatchFolderEnabled:   s.WatchFolderEnabled,
		WatchFolderDir:       s.WatchFolderDir,
		WatchFolderDelay:     s.WatchFolderDelaySeconds,
		WatchFolderDelete:    s.WatchFolderDeleteSource,
		ContentSearchEnabled: s.ContentSearchEnabled,
		ContentSearchContext: s.ContentSearchContext,
	}
}

// Get returns the current settings.
func (s *SettingsService) Get() AppSettings {
	return toAppSettings(s.store.Get())
}

// SetImportMode switches between "copy" and "reference". An unknown value is
// normalized to "copy" by the store.
func (s *SettingsService) SetImportMode(mode string) (AppSettings, error) {
	cur := s.store.Get()
	cur.ImportMode = library.Mode(mode)
	next, err := s.store.Update(cur)
	return toAppSettings(next), err
}

// ChooseLibraryFolder opens a native directory picker to set the managed
// library location (used in copy mode). A cancelled dialog leaves it unchanged.
func (s *SettingsService) ChooseLibraryFolder() (AppSettings, error) {
	dir, err := application.Get().Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		SetTitle("Choisir le dossier de la bibliothèque gérée").
		PromptForSingleSelection()
	if err != nil || dir == "" {
		return toAppSettings(s.store.Get()), err
	}
	cur := s.store.Get()
	cur.LibraryDir = dir
	next, err := s.store.Update(cur)
	return toAppSettings(next), err
}

// SetRemotePathTemplate stores the template used for future KOReader sends.
func (s *SettingsService) SetRemotePathTemplate(tmpl string) (AppSettings, error) {
	cur := s.store.Get()
	cur.RemotePathTemplate = tmpl
	next, err := s.store.Update(cur)
	return toAppSettings(next), err
}

// SetWriteMetadataToFile toggles writing edited metadata back into ebook files
// on save.
func (s *SettingsService) SetWriteMetadataToFile(enabled bool) (AppSettings, error) {
	cur := s.store.Get()
	cur.WriteMetadataToFile = enabled
	next, err := s.store.Update(cur)
	return toAppSettings(next), err
}

// SetTheme sets the UI appearance: "system", "light" or "dark".
func (s *SettingsService) SetTheme(theme string) (AppSettings, error) {
	cur := s.store.Get()
	cur.Theme = theme
	next, err := s.store.Update(cur)
	return toAppSettings(next), err
}

// SetLanguage sets the UI language. Unknown values are normalized to French by
// the settings store.
func (s *SettingsService) SetLanguage(language string) (AppSettings, error) {
	cur := s.store.Get()
	cur.Language = language
	next, err := s.store.Update(cur)
	return toAppSettings(next), err
}

// SetFeatureDiscover toggles the Project Gutenberg discovery view.
func (s *SettingsService) SetFeatureDiscover(enabled bool) (AppSettings, error) {
	cur := s.store.Get()
	cur.FeatureDiscover = enabled
	next, err := s.store.Update(cur)
	return toAppSettings(next), err
}

// SetFeatureSmartShelves toggles rule-based smart shelves.
func (s *SettingsService) SetFeatureSmartShelves(enabled bool) (AppSettings, error) {
	cur := s.store.Get()
	cur.FeatureSmartShelves = enabled
	next, err := s.store.Update(cur)
	return toAppSettings(next), err
}

// SetContentSearchEnabled toggles indexing/searching inside ebook contents.
func (s *SettingsService) SetContentSearchEnabled(enabled bool) (AppSettings, error) {
	cur := s.store.Get()
	cur.ContentSearchEnabled = enabled
	next, err := s.store.Update(cur)
	if err == nil && enabled && s.library != nil {
		go s.library.reindexContentQuietly()
	}
	return toAppSettings(next), err
}

// SetContentSearchContext stores how much context content search snippets show.
func (s *SettingsService) SetContentSearchContext(mode string) (AppSettings, error) {
	cur := s.store.Get()
	cur.ContentSearchContext = mode
	next, err := s.store.Update(cur)
	return toAppSettings(next), err
}

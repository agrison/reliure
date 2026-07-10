package opds

import (
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agrison/reliure/internal/core"
)

const (
	atomType       = `application/atom+xml;profile=opds-catalog;kind=acquisition`
	navType        = `application/atom+xml;profile=opds-catalog;kind=navigation`
	epubType       = "application/epub+zip"
	openSearchType = "application/opensearchdescription+xml"
)

// HandlerConfig configures the HTTP OPDS handler.
type HandlerConfig struct {
	Catalog  Catalog
	CoverDir string
	Title    string
	Now      func() time.Time
	// Logger receives one structured record per request plus download details.
	// Defaults to slog.Default().
	Logger *slog.Logger
}

// NewHandler returns a standalone OPDS HTTP handler. It owns only OPDS routes;
// mounting it in the desktop app or a CLI server is left to the caller.
func NewHandler(cfg HandlerConfig) http.Handler {
	if cfg.Title == "" {
		cfg.Title = "Reliure"
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	h := &handler{cfg: cfg, log: cfg.Logger.With("component", "opds")}
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.root)
	mux.HandleFunc("/recent", h.recent)
	mux.HandleFunc("/authors", h.authors)
	mux.HandleFunc("/authors/", h.authorBooks)
	mux.HandleFunc("/series", h.series)
	mux.HandleFunc("/series/", h.seriesBooks)
	mux.HandleFunc("/search", h.search)
	mux.HandleFunc("/opensearch.xml", h.openSearch)
	mux.HandleFunc("/books/", h.bookFile)
	mux.HandleFunc("/covers/", h.cover)
	return logging(h.log, mux)
}

type handler struct {
	cfg HandlerConfig
	log *slog.Logger
}

func (h *handler) root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	base := baseURL(r)
	f := navFeed(h.cfg.Title, base+"/", h.cfg.Now())
	f.Links = append(f.Links,
		link{Rel: "search", Type: openSearchType, Href: base + "/opensearch.xml", Title: "Recherche"},
	)
	f.Entries = []entry{
		navEntry("recent", "Ajouts récents", base+"/recent"),
		navEntry("authors", "Auteurs", base+"/authors"),
		navEntry("series", "Séries", base+"/series"),
	}
	writeFeed(w, f)
}

func (h *handler) recent(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/recent" {
		http.NotFound(w, r)
		return
	}
	books, err := h.cfg.Catalog.Recent(r.Context(), 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.writeBooks(w, r, "Ajouts récents", "/recent", books)
}

func (h *handler) authors(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/authors" {
		http.NotFound(w, r)
		return
	}
	items, err := h.cfg.Catalog.Authors(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.writeGroups(w, r, "Auteurs", "/authors", "/authors/", items)
}

func (h *handler) authorBooks(w http.ResponseWriter, r *http.Request) {
	id, ok := tailID(r.URL.Path, "/authors/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	books, err := h.cfg.Catalog.BooksByAuthor(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	title := "Auteur"
	if len(books) > 0 && len(books[0].Authors) > 0 {
		title = books[0].Authors[0].Author.Name
	}
	h.writeBooks(w, r, title, r.URL.Path, books)
}

func (h *handler) series(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/series" {
		http.NotFound(w, r)
		return
	}
	items, err := h.cfg.Catalog.Series(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.writeGroups(w, r, "Séries", "/series", "/series/", items)
}

func (h *handler) seriesBooks(w http.ResponseWriter, r *http.Request) {
	id, ok := tailID(r.URL.Path, "/series/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	books, err := h.cfg.Catalog.BooksBySeries(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	title := "Série"
	if len(books) > 0 && books[0].Series != nil {
		title = books[0].Series.Name
	}
	h.writeBooks(w, r, title, r.URL.Path, books)
}

func (h *handler) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	books, err := h.cfg.Catalog.Search(r.Context(), q, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.writeBooks(w, r, "Recherche: "+q, "/search?q="+q, books)
}

func (h *handler) openSearch(w http.ResponseWriter, r *http.Request) {
	base := baseURL(r)
	w.Header().Set("Content-Type", openSearchType+"; charset=utf-8")
	_, _ = w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	_ = enc.Encode(openSearchDescription{
		XMLNS:       "http://a9.com/-/spec/opensearch/1.1/",
		ShortName:   h.cfg.Title,
		Description: "Recherche dans la bibliothèque Reliure",
		URL: openSearchURL{
			Type:     atomType,
			Template: base + "/search?q={searchTerms}",
		},
	})
}

func (h *handler) bookFile(w http.ResponseWriter, r *http.Request) {
	bookID, fileID, ok := bookFileIDs(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	b, err := h.cfg.Catalog.Book(r.Context(), bookID)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, f := range b.Files {
		if f.ID != fileID || strings.ToLower(f.Format) != "epub" {
			continue
		}
		name := safeDownloadName(b.Title) + ".epub"
		w.Header().Set("Content-Type", epubType)
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
		h.log.Info("download", "book", b.ID, "file", f.ID, "title", b.Title,
			"name", name, "remote", clientIP(r))
		http.ServeFile(w, r, f.Path)
		return
	}
	http.NotFound(w, r)
}

func (h *handler) cover(w http.ResponseWriter, r *http.Request) {
	if h.cfg.CoverDir == "" {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/covers/")
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, `\`) {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(h.cfg.CoverDir, name)
	if _, err := os.Stat(path); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, path)
}

func (h *handler) writeGroups(w http.ResponseWriter, r *http.Request, title, self, prefix string, items []NamedCount) {
	base := baseURL(r)
	f := navFeed(title, base+self, h.cfg.Now())
	for _, item := range items {
		e := navEntry(strconv.FormatInt(item.ID, 10), fmt.Sprintf("%s (%d)", item.Name, item.Count), base+prefix+strconv.FormatInt(item.ID, 10))
		f.Entries = append(f.Entries, e)
	}
	writeFeed(w, f)
}

func (h *handler) writeBooks(w http.ResponseWriter, r *http.Request, title, self string, books []*core.Book) {
	base := baseURL(r)
	f := acquisitionFeed(title, base+self, h.cfg.Now())
	for _, b := range books {
		if epub := epubFile(b); epub != nil {
			f.Entries = append(f.Entries, bookEntry(base, b, *epub))
		}
	}
	writeFeed(w, f)
}

func epubFile(b *core.Book) *core.File {
	for i := range b.Files {
		if strings.ToLower(b.Files[i].Format) == "epub" {
			return &b.Files[i]
		}
	}
	return nil
}

func tailID(path, prefix string) (int64, bool) {
	raw := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if raw == "" || strings.Contains(raw, "/") {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil && id > 0
}

func bookFileIDs(path string) (int64, int64, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "books" || parts[2] != "files" {
		return 0, 0, false
	}
	bookID, err1 := strconv.ParseInt(parts[1], 10, 64)
	fileID, err2 := strconv.ParseInt(parts[3], 10, 64)
	return bookID, fileID, err1 == nil && err2 == nil && bookID > 0 && fileID > 0
}

func baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" || proto == "https" {
		scheme = proto
	}
	return scheme + "://" + r.Host
}

func safeDownloadName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "book"
	}
	replacer := strings.NewReplacer("/", "-", `\`, "-", ":", "-", "\x00", "")
	return replacer.Replace(name)
}

// logging wraps a handler, emitting one structured record per request with the
// method, path, response status, byte count, client and duration.
func logging(l *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		l.Info("request",
			"method", r.Method,
			"path", r.URL.RequestURI(),
			"status", rec.status,
			"bytes", rec.bytes,
			"remote", clientIP(r),
			"dur", time.Since(start).Round(time.Millisecond).String(),
		)
	})
}

// statusRecorder captures the status code and byte count of a response.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// clientIP returns the request's remote IP without the port.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func writeFeed(w http.ResponseWriter, f feed) {
	contentType := navType
	for _, l := range f.Links {
		if l.Rel == "self" && l.Type != "" {
			contentType = l.Type
			break
		}
	}
	w.Header().Set("Content-Type", contentType+"; charset=utf-8")
	_, _ = w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	_ = enc.Encode(f)
}

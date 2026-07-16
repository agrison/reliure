package main

import (
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/agrison/reliure/internal/core"
	"github.com/agrison/reliure/internal/device"
	"github.com/agrison/reliure/internal/koreaderstats"
	"github.com/agrison/reliure/internal/settings"
)

// StatsService computes the analytics dashboard. It is read-only and assembles
// its answer from a handful of aggregate queries plus the device inventory.
type StatsService struct {
	db               *core.DB
	inventory        *device.Store
	settings         *settings.Store
	readingStatsPath string // cached KOReader reading stats (JSON), if fetched
}

// NameCount is a labelled magnitude for the dashboard's bar charts.
type NameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// ReadingBreakdown splits the library by reading status (unread = everything
// not tracked or explicitly new).
type ReadingBreakdown struct {
	Complete  int `json:"complete"`
	Reading   int `json:"reading"`
	Abandoned int `json:"abandoned"`
	Unread    int `json:"unread"`
}

// RatingGroup counts the books that carry a given star rating and lists them
// (for the dashboard's expandable rating breakdown).
type RatingGroup struct {
	Rating int        `json:"rating"` // 1..5
	Count  int        `json:"count"`
	Books  []BookCard `json:"books"`
}

// Dashboard is the full analytics payload.
type Dashboard struct {
	Books        int              `json:"books"`
	Authors      int              `json:"authors"`
	Series       int              `json:"series"`
	Tags         int              `json:"tags"`
	Files        int              `json:"files"`
	TotalSize    int64            `json:"totalSize"`
	OnDevice     int              `json:"onDevice"`
	Annotations  int              `json:"annotations"`
	Reading      ReadingBreakdown `json:"reading"`
	Ratings      []RatingGroup    `json:"ratings"`
	Formats      []NameCount      `json:"formats"`
	Languages    []NameCount      `json:"languages"`
	TopAuthors   []NameCount      `json:"topAuthors"`
	TopTags      []NameCount      `json:"topTags"`
	AddedByMonth []NameCount      `json:"addedByMonth"`
	Recent       []BookCard       `json:"recent"`
	Content      ContentDashboard `json:"content"`
	// ReadingStats holds KOReader reading-time aggregates when the feature is on
	// and stats have been fetched; nil otherwise.
	ReadingStats *koreaderstats.ReadingStats `json:"readingStats"`
}

type ContentDashboard struct {
	Enabled      bool  `json:"enabled"`
	IndexedBooks int   `json:"indexedBooks"`
	EmptyBooks   int   `json:"emptyBooks"`
	FailedBooks  int   `json:"failedBooks"`
	PendingBooks int   `json:"pendingBooks"`
	IndexedChars int64 `json:"indexedChars"`
}

// Dashboard gathers every statistic shown on the dashboard.
func (s *StatsService) Dashboard() (Dashboard, error) {
	var d Dashboard
	var err error

	if d.Books, err = s.db.Books.Count(); err != nil {
		return d, err
	}
	if s.settings != nil && s.settings.Get().ContentSearchEnabled {
		d.Content.Enabled = true
		cst, err := s.db.Content.Stats()
		if err != nil {
			return d, err
		}
		d.Content.IndexedBooks = cst.IndexedBooks
		d.Content.EmptyBooks = cst.EmptyBooks
		d.Content.FailedBooks = cst.FailedBooks
		d.Content.IndexedChars = cst.IndexedChars
		covered := cst.IndexedBooks + cst.EmptyBooks + cst.FailedBooks
		if d.Content.PendingBooks = d.Books - covered; d.Content.PendingBooks < 0 {
			d.Content.PendingBooks = 0
		}
	}
	authorCounts, err := s.db.Authors.Counts()
	if err != nil {
		return d, err
	}
	seriesCounts, err := s.db.Series.Counts()
	if err != nil {
		return d, err
	}
	tagCounts, err := s.db.Tags.Counts()
	if err != nil {
		return d, err
	}
	d.Authors = len(authorCounts)
	d.Series = len(seriesCounts)
	d.Tags = len(tagCounts)
	d.TopAuthors = topNamed(authorCounts, 8)
	d.TopTags = topNamed(tagCounts, 8)

	if d.TotalSize, d.Files, err = s.db.Books.FileStats(); err != nil {
		return d, err
	}

	formats, err := s.db.Books.FormatCounts()
	if err != nil {
		return d, err
	}
	d.Formats = mapNamed(formats)

	languages, err := s.db.Books.LanguageCounts()
	if err != nil {
		return d, err
	}
	d.Languages = foldOthers(languages, 7)

	months, err := s.db.Books.AddedByMonth()
	if err != nil {
		return d, err
	}
	d.AddedByMonth = lastMonths(mapNamed(months), 12)

	statusCounts, err := s.db.Reading.StatusCounts()
	if err != nil {
		return d, err
	}
	d.Reading = ReadingBreakdown{
		Complete:  statusCounts["complete"],
		Reading:   statusCounts["reading"],
		Abandoned: statusCounts["abandoned"],
	}
	tracked := d.Reading.Complete + d.Reading.Reading + d.Reading.Abandoned
	if d.Reading.Unread = d.Books - tracked; d.Reading.Unread < 0 {
		d.Reading.Unread = 0
	}

	if d.Annotations, err = s.db.Reading.AnnotationTotal(); err != nil {
		return d, err
	}

	ratings, err := s.db.Reading.Ratings()
	if err != nil {
		return d, err
	}
	rated, err := s.db.Books.ListRated()
	if err != nil {
		return d, err
	}
	d.Ratings = ratingGroups(rated, ratings)

	if s.inventory != nil {
		if ids, err := s.inventory.AllBookIDs(); err == nil {
			d.OnDevice = len(ids)
		}
	}

	recent, err := s.db.Books.Browse("added", 8, 0)
	if err != nil {
		return d, err
	}
	d.Recent = cards(recent)
	d.ReadingStats = s.loadReadingStats()

	return d, nil
}

// loadReadingStats returns the cached KOReader reading stats when the feature is
// enabled and a fetch has happened; nil otherwise (section hidden).
func (s *StatsService) loadReadingStats() *koreaderstats.ReadingStats {
	if s.readingStatsPath == "" || !s.settings.Get().ReadingStatsEnabled {
		return nil
	}
	data, err := os.ReadFile(s.readingStatsPath)
	if err != nil {
		return nil
	}
	var rs koreaderstats.ReadingStats
	if json.Unmarshal(data, &rs) != nil {
		return nil
	}
	return &rs
}

// ratingGroups buckets the rated books into 5★…1★ rows (always all five, even
// when empty), preserving each book's rating→title order from ListRated.
func ratingGroups(rated []*core.Book, ratings map[int64]int) []RatingGroup {
	byRating := make(map[int][]BookCard, 5)
	for _, c := range cards(rated) {
		if r := ratings[c.ID]; r >= 1 && r <= 5 {
			byRating[r] = append(byRating[r], c)
		}
	}
	out := make([]RatingGroup, 0, 5)
	for r := 5; r >= 1; r-- {
		out = append(out, RatingGroup{Rating: r, Count: len(byRating[r]), Books: byRating[r]})
	}
	return out
}

// topNamed sorts entities by descending count (name breaks ties) and keeps n.
func topNamed(cs []core.NamedCount, n int) []NameCount {
	sorted := append([]core.NamedCount(nil), cs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
	})
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return mapNamed(sorted)
}

// foldOthers keeps the first n entries (already count-ordered) and sums the rest
// into a single "Autres" bucket, so a bar chart never sprouts an endless tail.
func foldOthers(cs []core.NamedCount, n int) []NameCount {
	out := mapNamed(cs)
	if len(out) <= n {
		return out
	}
	rest := 0
	for _, c := range out[n:] {
		rest += c.Count
	}
	out = out[:n]
	if rest > 0 {
		out = append(out, NameCount{Name: "Autres", Count: rest})
	}
	return out
}

// lastMonths keeps the most recent n entries of a chronologically-ordered slice.
func lastMonths(cs []NameCount, n int) []NameCount {
	if len(cs) > n {
		return cs[len(cs)-n:]
	}
	return cs
}

func mapNamed(cs []core.NamedCount) []NameCount {
	out := make([]NameCount, 0, len(cs))
	for _, c := range cs {
		out = append(out, NameCount{Name: c.Name, Count: c.Count})
	}
	return out
}

package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/agrison/reliure/internal/core"
)

// SmartShelfRule is one UI-editable predicate in a dynamic shelf.
type SmartShelfRule struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// SmartShelfInput creates or updates a smart shelf.
type SmartShelfInput struct {
	ID       int64            `json:"id"`
	Name     string           `json:"name"`
	Match    string           `json:"match"`
	Rules    []SmartShelfRule `json:"rules"`
	Position int              `json:"position"`
}

// SmartShelfSummary is used by the sidebar and shelf list.
type SmartShelfSummary struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// SmartShelfDetail is the full shelf definition plus its current book count.
type SmartShelfDetail struct {
	ID        int64            `json:"id"`
	Name      string           `json:"name"`
	Match     string           `json:"match"`
	Rules     []SmartShelfRule `json:"rules"`
	Position  int              `json:"position"`
	Count     int              `json:"count"`
	CreatedAt string           `json:"createdAt"`
	UpdatedAt string           `json:"updatedAt"`
}

// SmartShelves returns all dynamic shelves with live counts.
func (s *LibraryService) SmartShelves() ([]SmartShelfSummary, error) {
	shelves, err := s.db.Shelves.List()
	if err != nil {
		return nil, err
	}
	ctx, err := s.shelfContext()
	if err != nil {
		return nil, err
	}
	out := make([]SmartShelfSummary, 0, len(shelves))
	for _, sh := range shelves {
		out = append(out, SmartShelfSummary{ID: sh.ID, Name: sh.Name, Count: countShelf(ctx, sh)})
	}
	return out, nil
}

// SmartShelf returns one shelf definition.
func (s *LibraryService) SmartShelf(id int64) (SmartShelfDetail, error) {
	sh, err := s.db.Shelves.ByID(id)
	if err != nil {
		return SmartShelfDetail{}, err
	}
	ctx, err := s.shelfContext()
	if err != nil {
		return SmartShelfDetail{}, err
	}
	return smartShelfDetail(sh, countShelf(ctx, sh)), nil
}

// SaveSmartShelf creates or updates a dynamic shelf.
func (s *LibraryService) SaveSmartShelf(in SmartShelfInput) (SmartShelfDetail, error) {
	sh := core.SmartShelf{
		ID:       in.ID,
		Name:     in.Name,
		Match:    in.Match,
		Rules:    coreShelfRules(in.Rules),
		Position: in.Position,
	}
	if err := validateShelf(sh); err != nil {
		return SmartShelfDetail{}, err
	}
	saved, err := s.db.Shelves.Save(sh)
	if err != nil {
		return SmartShelfDetail{}, err
	}
	ctx, err := s.shelfContext()
	if err != nil {
		return SmartShelfDetail{}, err
	}
	return smartShelfDetail(saved, countShelf(ctx, saved)), nil
}

// DeleteSmartShelf removes a shelf definition.
func (s *LibraryService) DeleteSmartShelf(id int64) error {
	return s.db.Shelves.Delete(id)
}

// BooksBySmartShelf returns the books currently matching a shelf definition.
func (s *LibraryService) BooksBySmartShelf(id int64) ([]BookCard, error) {
	sh, err := s.db.Shelves.ByID(id)
	if err != nil {
		return nil, err
	}
	ctx, err := s.shelfContext()
	if err != nil {
		return nil, err
	}
	var books []*core.Book
	for _, b := range ctx.books {
		if shelfMatches(ctx, sh, b) {
			books = append(books, b)
		}
	}
	return cards(books), nil
}

type shelfContext struct {
	books     []*core.Book
	reading   map[int64]core.ReadingState
	device    map[int64]string
	hasDevice bool
	now       time.Time
}

func (s *LibraryService) shelfContext() (shelfContext, error) {
	books, err := s.db.Books.Browse("title", 0, 0)
	if err != nil {
		return shelfContext{}, err
	}
	reading, err := s.db.Reading.AllStates()
	if err != nil {
		return shelfContext{}, err
	}
	ctx := shelfContext{books: books, reading: reading, device: map[int64]string{}, now: time.Now().UTC()}
	if s.calibre != nil && s.calibre.inventory != nil {
		if deviceName, connected := s.calibre.server.Device(); connected {
			inv, err := s.calibre.inventory.Load(deviceName)
			if err != nil {
				return shelfContext{}, err
			}
			ctx.hasDevice = true
			byBook := inv.ByBookID()
			for _, b := range books {
				if _, ok := byBook[b.ID]; ok {
					ctx.device[b.ID] = "present"
				} else {
					ctx.device[b.ID] = "absent"
				}
			}
		}
	}
	return ctx, nil
}

func countShelf(ctx shelfContext, sh core.SmartShelf) int {
	n := 0
	for _, b := range ctx.books {
		if shelfMatches(ctx, sh, b) {
			n++
		}
	}
	return n
}

func shelfMatches(ctx shelfContext, sh core.SmartShelf, b *core.Book) bool {
	if len(sh.Rules) == 0 {
		return true
	}
	wantAny := sh.Match == "any"
	for _, r := range sh.Rules {
		ok := ruleMatches(ctx, r, b)
		if wantAny && ok {
			return true
		}
		if !wantAny && !ok {
			return false
		}
	}
	return !wantAny
}

func ruleMatches(ctx shelfContext, r core.ShelfRule, b *core.Book) bool {
	field := strings.TrimSpace(strings.ToLower(r.Field))
	op := strings.TrimSpace(strings.ToLower(r.Operator))
	value := strings.TrimSpace(r.Value)
	values := shelfValues(ctx, field, b)
	switch field {
	case "added_within_days":
		days, err := strconv.Atoi(value)
		if err != nil || days < 0 || b.AddedAt.IsZero() {
			return false
		}
		return !b.AddedAt.Before(ctx.now.AddDate(0, 0, -days))
	case "on_device":
		if !ctx.hasDevice {
			return strings.EqualFold(value, "unknown")
		}
		return matchStrings(values, op, value)
	default:
		return matchStrings(values, op, value)
	}
}

func shelfValues(ctx shelfContext, field string, b *core.Book) []string {
	switch field {
	case "title":
		return []string{b.Title}
	case "author":
		out := make([]string, 0, len(b.Authors))
		for _, c := range b.Authors {
			out = append(out, c.Author.Name)
		}
		return out
	case "series":
		if b.Series == nil {
			return nil
		}
		return []string{b.Series.Name}
	case "tag":
		out := make([]string, 0, len(b.Tags))
		for _, t := range b.Tags {
			out = append(out, t.Name)
		}
		return out
	case "language":
		return []string{b.Language}
	case "format":
		out := make([]string, 0, len(b.Files))
		for _, f := range b.Files {
			out = append(out, f.Format)
		}
		return out
	case "reading_status":
		if st, ok := ctx.reading[b.ID]; ok && st.Status != "" {
			return []string{st.Status}
		}
		return []string{"unread"}
	case "on_device":
		if st, ok := ctx.device[b.ID]; ok {
			return []string{st}
		}
		return []string{"unknown"}
	}
	return nil
}

func matchStrings(values []string, op, needle string) bool {
	if op == "" {
		op = "is"
	}
	switch op {
	case "exists":
		return len(nonEmpty(values)) > 0
	case "not_exists":
		return len(nonEmpty(values)) == 0
	}
	needle = strings.ToLower(strings.TrimSpace(needle))
	for _, v := range values {
		v = strings.ToLower(strings.TrimSpace(v))
		switch op {
		case "is":
			if v == needle {
				return true
			}
		case "is_not":
			if v == needle {
				return false
			}
		case "contains":
			if needle != "" && strings.Contains(v, needle) {
				return true
			}
		case "not_contains":
			if needle != "" && strings.Contains(v, needle) {
				return false
			}
		}
	}
	return op == "is_not" || op == "not_contains"
}

func nonEmpty(values []string) []string {
	var out []string
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

func validateShelf(sh core.SmartShelf) error {
	if strings.TrimSpace(sh.Name) == "" {
		return errors.New("nom d'étagère vide")
	}
	if len(sh.Rules) == 0 {
		return nil
	}
	for i, r := range sh.Rules {
		field := strings.TrimSpace(r.Field)
		if field == "" {
			return fmt.Errorf("règle %d: champ vide", i+1)
		}
	}
	return nil
}

func coreShelfRules(in []SmartShelfRule) []core.ShelfRule {
	out := make([]core.ShelfRule, 0, len(in))
	for _, r := range in {
		out = append(out, core.ShelfRule{
			Field:    strings.TrimSpace(r.Field),
			Operator: strings.TrimSpace(r.Operator),
			Value:    strings.TrimSpace(r.Value),
		})
	}
	return out
}

func appShelfRules(in []core.ShelfRule) []SmartShelfRule {
	out := make([]SmartShelfRule, 0, len(in))
	for _, r := range in {
		out = append(out, SmartShelfRule{Field: r.Field, Operator: r.Operator, Value: r.Value})
	}
	return out
}

func smartShelfDetail(sh core.SmartShelf, count int) SmartShelfDetail {
	return SmartShelfDetail{
		ID:        sh.ID,
		Name:      sh.Name,
		Match:     sh.Match,
		Rules:     appShelfRules(sh.Rules),
		Position:  sh.Position,
		Count:     count,
		CreatedAt: formatTime(sh.CreatedAt),
		UpdatedAt: formatTime(sh.UpdatedAt),
	}
}

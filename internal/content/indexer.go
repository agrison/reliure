package content

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/agrison/reliure/internal/core"
)

type Indexer struct {
	DB *core.DB
}

type ReindexResult struct {
	Total   int `json:"total"`
	Indexed int `json:"indexed"`
	Empty   int `json:"empty"`
	Failed  int `json:"failed"`
}

func (x Indexer) IndexBook(ctx context.Context, bookID int64) (string, error) {
	if x.DB == nil {
		return "", errors.New("nil database")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	b, err := x.DB.Books.ByID(bookID)
	if err != nil {
		return "", err
	}
	f, ok := bestFile(b.Files)
	if !ok {
		err := errors.New("no readable file")
		_ = x.DB.Content.MarkFailed(bookID, 0, err.Error())
		return "failed", err
	}
	fragments, err := ExtractFragments(f.Path, f.Format)
	if err != nil {
		_ = x.DB.Content.MarkFailed(bookID, f.ID, err.Error())
		return "failed", err
	}
	if err := x.DB.Content.Upsert(bookID, f.ID, fragments); err != nil {
		return "", err
	}
	if len(fragments) == 0 {
		return "empty", nil
	}
	return "indexed", nil
}

func (x Indexer) ReindexAll(ctx context.Context) (ReindexResult, error) {
	var res ReindexResult
	books, err := x.DB.Books.Browse("title", 0, 0)
	if err != nil {
		return res, err
	}
	res.Total = len(books)
	for _, b := range books {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		status, err := x.IndexBook(ctx, b.ID)
		switch status {
		case "indexed":
			res.Indexed++
		case "empty":
			res.Empty++
		default:
			res.Failed++
		}
		if err != nil {
			continue
		}
	}
	return res, nil
}

func bestFile(files []core.File) (core.File, bool) {
	for _, format := range []string{"epub", "pdf"} {
		for _, f := range files {
			if strings.EqualFold(f.Format, format) && fileExists(f.Path) {
				return f, true
			}
		}
	}
	for _, f := range files {
		if fileExists(f.Path) {
			return f, true
		}
	}
	return core.File{}, false
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

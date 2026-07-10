package opds

import (
	"context"

	"github.com/agrison/reliure/internal/core"
)

// NamedCount is a navigation group exposed by the OPDS catalog.
type NamedCount = core.NamedCount

// Catalog is the storage-facing contract needed by the OPDS HTTP layer.
// Keeping it narrow makes the handler easy to test and reusable from a future
// headless CLI entry point.
type Catalog interface {
	Recent(ctx context.Context, limit int) ([]*core.Book, error)
	Search(ctx context.Context, query string, limit int) ([]*core.Book, error)
	Book(ctx context.Context, id int64) (*core.Book, error)
	Authors(ctx context.Context) ([]NamedCount, error)
	BooksByAuthor(ctx context.Context, id int64) ([]*core.Book, error)
	Series(ctx context.Context) ([]NamedCount, error)
	BooksBySeries(ctx context.Context, id int64) ([]*core.Book, error)
}

// CoreCatalog adapts the core repositories to the OPDS catalog contract.
type CoreCatalog struct {
	DB *core.DB
}

func (c CoreCatalog) Recent(ctx context.Context, limit int) ([]*core.Book, error) {
	_ = ctx
	return c.DB.Books.Browse("added", limit, 0)
}

func (c CoreCatalog) Search(ctx context.Context, query string, limit int) ([]*core.Book, error) {
	_ = ctx
	return c.DB.Books.Search(query, limit)
}

func (c CoreCatalog) Book(ctx context.Context, id int64) (*core.Book, error) {
	_ = ctx
	return c.DB.Books.ByID(id)
}

func (c CoreCatalog) Authors(ctx context.Context) ([]NamedCount, error) {
	_ = ctx
	return c.DB.Authors.Counts()
}

func (c CoreCatalog) BooksByAuthor(ctx context.Context, id int64) ([]*core.Book, error) {
	_ = ctx
	return c.DB.Books.ListByAuthor(id)
}

func (c CoreCatalog) Series(ctx context.Context) ([]NamedCount, error) {
	_ = ctx
	return c.DB.Series.Counts()
}

func (c CoreCatalog) BooksBySeries(ctx context.Context, id int64) ([]*core.Book, error) {
	_ = ctx
	return c.DB.Books.ListBySeries(id)
}

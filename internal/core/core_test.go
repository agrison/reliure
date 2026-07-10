package core

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// newTestDB ouvre une base SQLite en mémoire migrée, fermée en fin de test.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func fptr(f float64) *float64 { return &f }

// sampleBook construit un livre de test avec série, auteurs, tags et un fichier.
func sampleBook() *Book {
	return &Book{
		Title:       "La Main gauche de la nuit",
		TitleSort:   "Main gauche de la nuit, La",
		Language:    "fr",
		Description: "Roman de science-fiction.",
		Series:      &Series{Name: "Cycle de l'Ekumen"},
		SeriesIndex: fptr(4),
		Authors: []Contribution{
			{Author: Author{Name: "Ursula K. Le Guin"}, Role: "aut", Position: 0},
			{Author: Author{Name: "Jean Bailhache"}, Role: "trl", Position: 1},
		},
		Tags: []Tag{{Name: "science-fiction"}, {Name: "classique"}},
		Files: []File{
			{Path: "/lib/LeGuin/MainGauche.epub", Format: "epub", Size: 1234, SHA256: "abc"},
		},
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db := newTestDB(t)
	// Ré-exécuter migrate ne doit rien casser ni ré-appliquer.
	if err := migrate(db.sql); err != nil {
		t.Fatalf("migrate rejouée: %v", err)
	}
	v, err := currentVersion(db.sql)
	if err != nil {
		t.Fatal(err)
	}
	migs, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	want := migs[len(migs)-1].version
	if v != want {
		t.Fatalf("version schéma = %d, attendu %d", v, want)
	}
}

func TestCreateAndByID(t *testing.T) {
	db := newTestDB(t)
	b := sampleBook()
	if err := db.Books.Create(b); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if b.ID == 0 {
		t.Fatal("ID non renseigné après Create")
	}
	if b.AddedAt.IsZero() || b.UpdatedAt.IsZero() {
		t.Fatal("timestamps non renseignés")
	}

	got, err := db.Books.ByID(b.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.Title != b.Title || got.Language != "fr" {
		t.Errorf("champs de base incorrects: %+v", got)
	}
	if got.Series == nil || got.Series.Name != "Cycle de l'Ekumen" {
		t.Errorf("série non chargée: %+v", got.Series)
	}
	if got.SeriesIndex == nil || *got.SeriesIndex != 4 {
		t.Errorf("series_index = %v, attendu 4", got.SeriesIndex)
	}
	if len(got.Authors) != 2 {
		t.Fatalf("nb auteurs = %d, attendu 2", len(got.Authors))
	}
	if got.Authors[0].Author.Name != "Ursula K. Le Guin" || got.Authors[0].Role != "aut" {
		t.Errorf("auteur[0] incorrect: %+v", got.Authors[0])
	}
	if got.Authors[1].Role != "trl" {
		t.Errorf("rôle traducteur attendu, got %q", got.Authors[1].Role)
	}
	if len(got.Tags) != 2 {
		t.Errorf("nb tags = %d, attendu 2", len(got.Tags))
	}
	if len(got.Files) != 1 || got.Files[0].SHA256 != "abc" {
		t.Errorf("fichiers incorrects: %+v", got.Files)
	}
}

func TestDeriveSortNameOnCreate(t *testing.T) {
	db := newTestDB(t)
	a, err := db.Authors.GetOrCreate("Ursula K. Le Guin")
	if err != nil {
		t.Fatal(err)
	}
	if a.SortName != "Guin, Ursula K. Le" {
		t.Errorf("sort_name dérivé = %q", a.SortName)
	}
}

func TestGetOrCreateDeduplicates(t *testing.T) {
	db := newTestDB(t)
	a1, _ := db.Authors.GetOrCreate("Isaac Asimov")
	a2, _ := db.Authors.GetOrCreate("Isaac Asimov")
	if a1.ID != a2.ID {
		t.Fatalf("get-or-create a dupliqué l'auteur: %d != %d", a1.ID, a2.ID)
	}
	authors, _ := db.Authors.List()
	if len(authors) != 1 {
		t.Fatalf("nb auteurs = %d, attendu 1", len(authors))
	}
}

func TestSharedAuthorAcrossBooks(t *testing.T) {
	db := newTestDB(t)
	b1 := &Book{Title: "Fondation", Authors: []Contribution{{Author: Author{Name: "Isaac Asimov"}}}}
	b2 := &Book{Title: "Les Robots", Authors: []Contribution{{Author: Author{Name: "Isaac Asimov"}}}}
	if err := db.Books.Create(b1); err != nil {
		t.Fatal(err)
	}
	if err := db.Books.Create(b2); err != nil {
		t.Fatal(err)
	}
	if b1.Authors[0].Author.ID != b2.Authors[0].Author.ID {
		t.Fatal("l'auteur partagé aurait dû être réutilisé")
	}
	books, err := db.Books.ListByAuthor(b1.Authors[0].Author.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 2 {
		t.Fatalf("ListByAuthor = %d, attendu 2", len(books))
	}
}

func TestListBySeriesOrdered(t *testing.T) {
	db := newTestDB(t)
	mk := func(title string, idx float64) {
		b := &Book{Title: title, Series: &Series{Name: "Dune"}, SeriesIndex: fptr(idx)}
		if err := db.Books.Create(b); err != nil {
			t.Fatal(err)
		}
	}
	mk("Le Messie de Dune", 2)
	mk("Dune", 1)
	mk("Les Enfants de Dune", 3)

	s, err := db.Series.ByName("Dune")
	if err != nil {
		t.Fatal(err)
	}
	books, err := db.Books.ListBySeries(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) != 3 {
		t.Fatalf("nb livres = %d", len(books))
	}
	want := []string{"Dune", "Le Messie de Dune", "Les Enfants de Dune"}
	for i, b := range books {
		if b.Title != want[i] {
			t.Errorf("ordre série position %d: got %q, want %q", i, b.Title, want[i])
		}
	}
}

func TestUpdateReplacesRelations(t *testing.T) {
	db := newTestDB(t)
	b := sampleBook()
	if err := db.Books.Create(b); err != nil {
		t.Fatal(err)
	}
	// On change titre, on retire le traducteur, on change les tags et la série.
	b.Title = "The Left Hand of Darkness"
	b.Language = "en"
	b.Authors = []Contribution{{Author: Author{Name: "Ursula K. Le Guin"}, Role: "aut"}}
	b.Tags = []Tag{{Name: "sci-fi"}}
	b.Series = nil
	if err := db.Books.Update(b); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := db.Books.ByID(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "The Left Hand of Darkness" || got.Language != "en" {
		t.Errorf("maj champs échouée: %+v", got)
	}
	if got.Series != nil {
		t.Errorf("série aurait dû être retirée: %+v", got.Series)
	}
	if len(got.Authors) != 1 {
		t.Errorf("nb auteurs après maj = %d, attendu 1", len(got.Authors))
	}
	if len(got.Tags) != 1 || got.Tags[0].Name != "sci-fi" {
		t.Errorf("tags après maj = %+v", got.Tags)
	}
	// Le fichier ne doit pas avoir été touché par Update.
	if len(got.Files) != 1 {
		t.Errorf("fichiers après maj = %d, attendu 1 (intacts)", len(got.Files))
	}
}

func TestDeleteCascades(t *testing.T) {
	db := newTestDB(t)
	b := sampleBook()
	if err := db.Books.Create(b); err != nil {
		t.Fatal(err)
	}
	if err := db.Books.Delete(b.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := db.Books.ByID(b.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("ByID après delete = %v, attendu ErrNoRows", err)
	}
	// Liens et fichiers partis en cascade.
	var n int
	db.sql.QueryRow(`SELECT COUNT(*) FROM book_author`).Scan(&n)
	if n != 0 {
		t.Errorf("book_author non nettoyé: %d", n)
	}
	db.sql.QueryRow(`SELECT COUNT(*) FROM file`).Scan(&n)
	if n != 0 {
		t.Errorf("file non nettoyé: %d", n)
	}
	// Entrée FTS supprimée.
	db.sql.QueryRow(`SELECT COUNT(*) FROM book_fts`).Scan(&n)
	if n != 0 {
		t.Errorf("book_fts non nettoyé: %d", n)
	}
}

func TestDeleteMissing(t *testing.T) {
	db := newTestDB(t)
	if err := db.Books.Delete(999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Delete inexistant = %v, attendu ErrNoRows", err)
	}
}

func TestSearchFTS(t *testing.T) {
	db := newTestDB(t)
	if err := db.Books.Create(sampleBook()); err != nil {
		t.Fatal(err)
	}
	if err := db.Books.Create(&Book{Title: "Fondation", Authors: []Contribution{{Author: Author{Name: "Isaac Asimov"}}}}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		query string
		want  string
	}{
		{"nuit", "La Main gauche de la nuit"},      // titre
		{"asimov", "Fondation"},                    // auteur
		{"Ekumen", "La Main gauche de la nuit"},    // série
		{"classique", "La Main gauche de la nuit"}, // tag
		{"fonda", "Fondation"},                     // préfixe
	}
	for _, c := range cases {
		res, err := db.Books.Search(c.query, 0)
		if err != nil {
			t.Fatalf("Search(%q): %v", c.query, err)
		}
		if len(res) != 1 || res[0].Title != c.want {
			t.Errorf("Search(%q) = %v, attendu [%q]", c.query, titles(res), c.want)
		}
	}
}

func TestSearchDiacriticsInsensitive(t *testing.T) {
	db := newTestDB(t)
	if err := db.Books.Create(&Book{Title: "L'Étranger", Authors: []Contribution{{Author: Author{Name: "Albert Camus"}}}}); err != nil {
		t.Fatal(err)
	}
	res, err := db.Books.Search("etranger", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("recherche sans accent = %d résultats, attendu 1", len(res))
	}
}

func TestSearchReflectsUpdate(t *testing.T) {
	db := newTestDB(t)
	b := &Book{Title: "Titre provisoire"}
	if err := db.Books.Create(b); err != nil {
		t.Fatal(err)
	}
	b.Title = "Neuromancien"
	if err := db.Books.Update(b); err != nil {
		t.Fatal(err)
	}
	if res, _ := db.Books.Search("provisoire", 0); len(res) != 0 {
		t.Errorf("l'ancien titre est encore indexé: %v", titles(res))
	}
	if res, _ := db.Books.Search("Neuromancien", 0); len(res) != 1 {
		t.Errorf("le nouveau titre n'est pas indexé")
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	db := newTestDB(t)
	if res, err := db.Books.Search("   ", 0); err != nil || res != nil {
		t.Fatalf("recherche vide = (%v, %v), attendu (nil, nil)", res, err)
	}
}

func TestCreateRejectsEmptyTitle(t *testing.T) {
	db := newTestDB(t)
	if err := db.Books.Create(&Book{Title: "  "}); err == nil {
		t.Fatal("titre vide aurait dû être rejeté")
	}
	if n, _ := db.Books.Count(); n != 0 {
		t.Fatalf("aucun livre ne devait être créé, count=%d", n)
	}
}

func TestFileUniquePathRollback(t *testing.T) {
	db := newTestDB(t)
	b1 := &Book{Title: "A", Files: []File{{Path: "/dup.epub", Format: "epub"}}}
	if err := db.Books.Create(b1); err != nil {
		t.Fatal(err)
	}
	// Même chemin de fichier → contrainte UNIQUE → Create doit échouer et
	// ne rien laisser (rollback complet).
	b2 := &Book{Title: "B", Files: []File{{Path: "/dup.epub", Format: "epub"}}}
	if err := db.Books.Create(b2); err == nil {
		t.Fatal("chemin dupliqué aurait dû échouer")
	}
	if n, _ := db.Books.Count(); n != 1 {
		t.Fatalf("rollback incomplet: count=%d, attendu 1", n)
	}
}

func TestListPagination(t *testing.T) {
	db := newTestDB(t)
	for _, ti := range []string{"C", "A", "B", "E", "D"} {
		if err := db.Books.Create(&Book{Title: ti}); err != nil {
			t.Fatal(err)
		}
	}
	page, err := db.Books.List(2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Title != "A" || page[1].Title != "B" {
		t.Fatalf("page 1 = %v", titles(page))
	}
	page2, _ := db.Books.List(2, 2)
	if len(page2) != 2 || page2[0].Title != "C" {
		t.Fatalf("page 2 = %v", titles(page2))
	}
}

func TestOpenFileBackedPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lib.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open fichier: %v", err)
	}
	b := &Book{Title: "Persistant", Authors: []Contribution{{Author: Author{Name: "Auteur"}}}}
	if err := db.Books.Create(b); err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Rouvrir : migrations idempotentes, données présentes.
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("réouverture: %v", err)
	}
	defer db2.Close()
	got, err := db2.Books.ByID(b.ID)
	if err != nil {
		t.Fatalf("ByID après réouverture: %v", err)
	}
	if got.Title != "Persistant" {
		t.Errorf("données non persistées: %+v", got)
	}
	if res, _ := db2.Books.Search("persistant", 0); len(res) != 1 {
		t.Errorf("index FTS non persisté")
	}
}

func TestDefaultDBPath(t *testing.T) {
	p, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("DefaultDBPath: %v", err)
	}
	if filepath.Base(p) != "library.db" || filepath.Base(filepath.Dir(p)) != appDir {
		t.Errorf("chemin DB inattendu: %s", p)
	}
}

func titles(bs []*Book) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Title
	}
	return out
}

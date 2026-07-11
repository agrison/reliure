package core

import "testing"

func TestSmartShelfRepoSaveListDelete(t *testing.T) {
	db := newTestDB(t)
	saved, err := db.Shelves.Save(SmartShelf{
		Name:  "SF en cours",
		Match: "all",
		Rules: []ShelfRule{
			{Field: "tag", Operator: "is", Value: "SF"},
			{Field: "reading_status", Operator: "is", Value: "reading"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 || saved.Name != "SF en cours" || len(saved.Rules) != 2 {
		t.Fatalf("saved shelf = %+v", saved)
	}

	list, err := db.Shelves.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != saved.ID {
		t.Fatalf("list = %+v", list)
	}

	saved.Name = "SF"
	saved.Match = "any"
	saved.Rules = saved.Rules[:1]
	updated, err := db.Shelves.Save(saved)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "SF" || updated.Match != "any" || len(updated.Rules) != 1 {
		t.Fatalf("updated shelf = %+v", updated)
	}
	if err := db.Shelves.Delete(saved.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ = db.Shelves.List(); len(list) != 0 {
		t.Fatalf("shelf not deleted: %+v", list)
	}
}

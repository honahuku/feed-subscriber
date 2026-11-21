package app

import "testing"

func TestUnseenItemsReturnsOnlyItemsAfterStoredID(t *testing.T) {
	items := []Item{
		{ID: "newest"},
		{ID: "newer"},
		{ID: "old"},
	}

	got, latest, found := unseenItems(items, "old")
	if !found {
		t.Fatal("found = false, want true")
	}
	if len(got) != 2 {
		t.Fatalf("len(unseenItems) = %d, want 2", len(got))
	}
	if got[0].ID != "newest" || got[1].ID != "newer" {
		t.Errorf("unseen IDs = %q, %q; want newest, newer", got[0].ID, got[1].ID)
	}
	if latest != "newest" {
		t.Errorf("latest = %q, want newest", latest)
	}
}

func TestUnseenItemsSeedsFirstRunWithoutSendingHistory(t *testing.T) {
	items := []Item{{ID: "latest"}, {ID: "older"}}

	got, latest, found := unseenItems(items, "")
	if found {
		t.Fatal("found = true, want false on first run")
	}
	if len(got) != 0 {
		t.Errorf("len(unseenItems) = %d, want 0 on first run", len(got))
	}
	if latest != "latest" {
		t.Errorf("latest = %q, want latest", latest)
	}
}

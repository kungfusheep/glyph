package glyph

import "testing"

func TestFilterListOnFilterChangeProgrammaticPaths(t *testing.T) {
	items := []string{"alpha", "beta", "gamma"}
	fired := 0
	fl := FilterList(&items, func(s *string) string { return *s }).
		OnFilterChange(func() { fired++ })

	fl.SetQuery("al")
	if fired != 1 {
		t.Fatalf("after SetQuery: fired = %d, want 1", fired)
	}

	fl.SetQuery("al")
	if fired != 1 {
		t.Fatalf("after no-op SetQuery: fired = %d, want 1 (unchanged query must not fire)", fired)
	}

	fl.DeleteCharBefore()
	if fired != 2 {
		t.Fatalf("after DeleteCharBefore: fired = %d, want 2", fired)
	}

	fl.Clear()
	if fired != 3 {
		t.Fatalf("after Clear with query: fired = %d, want 3", fired)
	}

	fl.Clear()
	if fired != 3 {
		t.Fatalf("after Clear when already empty: fired = %d, want 3 (no-op must not fire)", fired)
	}

	if fl.Query() != "" {
		t.Fatalf("query = %q, want empty", fl.Query())
	}
}

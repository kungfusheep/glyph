package main

import "testing"

func names(ps []*pane) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.name
	}
	return out
}

func TestSplitAndLeaves(t *testing.T) {
	a := &pane{name: "a"}
	tr := newTree(a)
	if got := names(tr.leaves()); len(got) != 1 || got[0] != "a" {
		t.Fatalf("initial leaves = %v, want [a]", got)
	}

	b := &pane{name: "b"}
	tr.splitFocused(true, b) // side by side
	if got := names(tr.leaves()); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("after split leaves = %v, want [a b]", got)
	}
	if tr.focused != b {
		t.Fatalf("focus should move to new pane b, got %v", tr.focused.name)
	}
}

func TestFocusNextWraps(t *testing.T) {
	a := &pane{name: "a"}
	tr := newTree(a)
	b := &pane{name: "b"}
	c := &pane{name: "c"}
	tr.splitFocused(true, b)  // focus b
	tr.splitFocused(false, c) // focus c; tree: a | (b / c)

	order := []string{}
	for i := 0; i < 4; i++ {
		order = append(order, tr.focused.name)
		tr.focusNext()
	}
	// leaves() order is a, b, c; focusNext from c wraps to a
	// starting focus is c, so: c, a, b, c
	want := []string{"c", "a", "b", "c"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("focus order = %v, want %v", order, want)
		}
	}
}

func TestPruneCollapsesSplit(t *testing.T) {
	a := &pane{name: "a"}
	tr := newTree(a)
	b := &pane{name: "b"}
	tr.splitFocused(true, b) // a | b, focus b

	b.dead = true
	tr.prune()
	if got := names(tr.leaves()); len(got) != 1 || got[0] != "a" {
		t.Fatalf("after pruning b, leaves = %v, want [a]", got)
	}
	// the split collapsed to a single leaf; focus fell back to the survivor
	if tr.focused != a {
		t.Fatalf("focus should fall back to a, got %v", tr.focused)
	}
	if tr.root.split != nil || tr.root.leaf != a {
		t.Fatal("root should collapse to leaf a")
	}
}

func TestPruneNestedCollapse(t *testing.T) {
	a := &pane{name: "a"}
	tr := newTree(a)
	b := &pane{name: "b"}
	c := &pane{name: "c"}
	tr.splitFocused(true, b)  // a | b
	tr.splitFocused(false, c) // a | (b / c), focus c

	// kill b and c: the inner split empties, then a remains
	b.dead, c.dead = true, true
	tr.prune()
	if got := names(tr.leaves()); len(got) != 1 || got[0] != "a" {
		t.Fatalf("leaves = %v, want [a]", got)
	}
	if tr.focused != a {
		t.Fatalf("focus = %v, want a", tr.focused)
	}
}

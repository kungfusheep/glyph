package glyph

import "testing"

// TestJumpScopeInScope covers the point-in-rect scope predicate: no rects means
// the whole screen; half-open bounds; an empty/zero rect matches nothing.
func TestJumpScopeInScope(t *testing.T) {
	jm := &JumpMode{}
	if !jm.inScope(0, 0) || !jm.inScope(99, 99) {
		t.Fatal("no scope rects should mean the whole screen is in scope")
	}

	r := &NodeRef{X: 2, Y: 2, W: 4, H: 3} // [2,6) x [2,5)
	jm.ScopeRects = []*NodeRef{r}
	cases := []struct {
		x, y int
		want bool
	}{
		{2, 2, true},  // top-left corner
		{5, 4, true},  // last in-bounds cell
		{1, 2, false}, // left of
		{6, 2, false}, // right (half-open upper X)
		{2, 5, false}, // below (half-open upper Y)
	}
	for _, c := range cases {
		if got := jm.inScope(c.x, c.y); got != c.want {
			t.Errorf("inScope(%d,%d) = %v, want %v", c.x, c.y, got, c.want)
		}
	}

	// union of regions
	jm.ScopeRects = []*NodeRef{{X: 0, Y: 0, W: 2, H: 2}, {X: 10, Y: 10, W: 2, H: 2}}
	if !jm.inScope(1, 1) || !jm.inScope(11, 11) {
		t.Error("union: a point inside any region should be in scope")
	}
	if jm.inScope(5, 5) {
		t.Error("union: a point in neither region should be out of scope")
	}

	// empty / unrendered-pane rect matches nothing
	jm.ScopeRects = []*NodeRef{{X: 0, Y: 0, W: 0, H: 0}}
	if jm.inScope(0, 0) {
		t.Error("an empty rect (W=0,H=0) must match nothing")
	}
}

// TestJumpScopeFiltersTargets: with a scope rect active, AddJumpTarget collects
// only targets that render inside the region.
func TestJumpScopeFiltersTargets(t *testing.T) {
	a := &App{jumpMode: &JumpMode{Active: true, ScopeRects: []*NodeRef{{X: 0, Y: 0, W: 10, H: 5}}}}
	a.AddJumpTarget(3, 2, func() {}, Style{}) // inside
	a.AddJumpTarget(20, 2, func() {}, Style{}) // right of region
	a.AddJumpTarget(3, 9, func() {}, Style{})  // below region
	if n := len(a.jumpMode.Targets); n != 1 {
		t.Fatalf("expected 1 in-scope target, got %d", n)
	}
	if tg := a.jumpMode.Targets[0]; tg.X != 3 || tg.Y != 2 {
		t.Fatalf("wrong target kept: %+v", tg)
	}
}

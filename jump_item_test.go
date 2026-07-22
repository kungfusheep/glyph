package glyph

import "testing"

// JumpItem targets inside ForEach must each resolve to the item their
// rendered instance was bound to — one compiled op, one target per item.
func TestJumpItemResolvesPerForEachItem(t *testing.T) {
	type row struct {
		Name string
	}
	rows := []row{{Name: "alpha"}, {Name: "beta"}, {Name: "gamma"}}

	var picked []string
	app := NewApp()
	app.JumpMode().setActive(true)
	tmpl := Build(VBox(
		ForEach(&rows, func(r *row) Component {
			return JumpItem(Text(&r.Name), func(r *row) {
				picked = append(picked, r.Name)
			})
		}),
	))
	tmpl.SetApp(app)
	buf := NewBuffer(20, 5)
	tmpl.Execute(buf, 20, 5)
	app.JumpMode().AssignLabels() // publish the frame's built targets (as render does)

	targets := app.JumpMode().Targets
	if len(targets) != 3 {
		t.Fatalf("jump targets = %d, want one per item", len(targets))
	}
	for _, tgt := range targets {
		tgt.OnSelect()
	}
	if len(picked) != 3 || picked[0] != "alpha" || picked[1] != "beta" || picked[2] != "gamma" {
		t.Fatalf("picked = %#v, want each target bound to its own item", picked)
	}
}

// the EXPORTED activation path: external packages collect jump targets at an exact
// size via a low-level Execute, which needs a public way to activate jump mode — the
// regression when Active (public field) became active (unexported atomic.Bool).
// JumpMode.Activate/Deactivate restore it race-safely.
func TestJumpModeActivateDrivesExactSizeCollection(t *testing.T) {
	type cell struct{ Label string }
	cells := []cell{{Label: "a"}, {Label: "b"}, {Label: "c"}}

	app := NewApp()
	app.JumpMode().Activate() // the exported activation external tests rely on
	if !app.JumpModeActive() {
		t.Fatal("Activate() should activate jump mode")
	}

	tmpl := Build(VBox(
		ForEach(&cells, func(c *cell) Component {
			return JumpItem(Text(&c.Label), func(c *cell) {})
		}),
	))
	tmpl.SetApp(app)
	buf := NewBuffer(120, 40) // an EXACT size of the caller's choosing, not the screen
	tmpl.Execute(buf, 120, 40)
	app.JumpMode().AssignLabels()

	if got := len(app.JumpMode().Targets); got != 3 {
		t.Fatalf("targets at exact size = %d, want 3 (exported activation path broken)", got)
	}

	app.JumpMode().Deactivate()
	if app.JumpModeActive() {
		t.Fatal("Deactivate() should deactivate jump mode")
	}
}

// JumpItemRef passes the rendered geometry alongside the item, so overlays
// can anchor beside the selected row.
func TestJumpItemRefPassesGeometry(t *testing.T) {
	type row struct {
		Name string
	}
	rows := []row{{Name: "alpha"}, {Name: "beta"}}

	var gotName string
	var gotRef NodeRef
	app := NewApp()
	app.JumpMode().setActive(true)
	tmpl := Build(VBox(
		ForEach(&rows, func(r *row) Component {
			return HBox.Height(1)(
				JumpItemRef(Text(&r.Name), func(r *row, ref NodeRef) {
					gotName = r.Name
					gotRef = ref
				}),
			)
		}),
	))
	tmpl.SetApp(app)
	buf := NewBuffer(20, 5)
	tmpl.Execute(buf, 20, 5)
	app.JumpMode().AssignLabels() // publish the frame's built targets (as render does)

	targets := app.JumpMode().Targets
	if len(targets) != 2 {
		t.Fatalf("jump targets = %d, want one per item", len(targets))
	}
	targets[1].OnSelect()
	if gotName != "beta" {
		t.Fatalf("selected item = %q, want beta", gotName)
	}
	if gotRef.Y != 1 || gotRef.H != 1 {
		t.Fatalf("ref = %+v, want second row geometry", gotRef)
	}
}

// A JumpItem over a slice of POINTERS must hand the callback the slice slot,
// not the pointed-to struct.
//
// []*T is the shape FilterList renders (func(pp **T)). Field bindings inside
// such an item compile at offsets within the POINTED-TO struct, so the item's
// base is the struct — but a whole-element callback is typed for the slice's
// element type. Handing it the base delivered the struct's first word: a wild
// pointer (the Name string's data pointer) that panicked when dereferenced.
func TestJumpItemPointerSliceResolvesSlot(t *testing.T) {
	type row struct{ Name string }
	a, b, c := &row{Name: "alpha"}, &row{Name: "beta"}, &row{Name: "gamma"}
	rows := []*row{a, b, c}
	sel := 0

	var got []*row
	app := NewApp()
	app.JumpMode().setActive(true)
	tmpl := Build(VBox(
		List(&rows).Selection(&sel).Marker("").Render(func(p **row) Component {
			return JumpItem(Text(&(*p).Name), func(p **row) { got = append(got, *p) })
		}),
	))
	tmpl.SetApp(app)
	tmpl.Execute(NewBuffer(40, 20), 40, 20)
	app.JumpMode().AssignLabels()

	for _, tgt := range app.JumpMode().Targets {
		tgt.OnSelect()
	}
	if len(got) != 3 {
		t.Fatalf("picks = %d, want one per item", len(got))
	}
	// dereferencing must be safe AND each pick must be its own element
	for i, want := range []*row{a, b, c} {
		if got[i] != want {
			t.Errorf("pick %d resolved to %p (Name %q), want %p (%q)",
				i, got[i], got[i].Name, want, want.Name)
		}
	}
}

// the same, with the JumpItem inside an If branch: a sub-template is bound to
// the same item as its parent, so the slot has to propagate down with the base.
func TestJumpItemPointerSliceInsideIfBranch(t *testing.T) {
	type row struct {
		Name string
		On   bool
	}
	a, b := &row{Name: "alpha", On: true}, &row{Name: "beta", On: true}
	rows := []*row{a, b}

	var got []*row
	app := NewApp()
	app.JumpMode().setActive(true)
	tmpl := Build(VBox(
		ForEach(&rows, func(p **row) Component {
			return VBox(
				If(&(*p).On).
					Then(JumpItem(Text(&(*p).Name), func(p **row) { got = append(got, *p) })).
					Else(Text("off")),
			)
		}),
	))
	tmpl.SetApp(app)
	tmpl.Execute(NewBuffer(40, 20), 40, 20)
	app.JumpMode().AssignLabels()

	for _, tgt := range app.JumpMode().Targets {
		tgt.OnSelect()
	}
	if len(got) != 2 {
		t.Fatalf("picks = %d, want 2", len(got))
	}
	for i, want := range []*row{a, b} {
		if got[i] != want {
			t.Errorf("pick %d inside If resolved to %p (%q), want %p (%q)",
				i, got[i], got[i].Name, want, want.Name)
		}
	}
}

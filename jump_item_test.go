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
	app.JumpMode().Active = true
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
	app.JumpMode().Active = true
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

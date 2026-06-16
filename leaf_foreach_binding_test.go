package glyph

import (
	"strings"
	"testing"
)

// guard tests for the per-item-binding sweep: a pointer Width read from a
// ForEach element field must rebind per item on the leaf components, not freeze
// on the compile-time placeholder. Every swept surface (Text/Progress/Leader/
// Sparkline Width, VRule/Sparkline Height, LayerView Grow, Input Width) now
// routes through the same compileDynInt16/compileDynFloat32 call that rebases a
// per-item pointer; these prove it observably for the int16-width leaf paths.

func TestForEachPerItemTextWidthRebinds(t *testing.T) {
	type row struct {
		w     int16
		label string
	}
	rows := []row{{4, "a"}, {12, "a"}}
	tmpl := Build(VBox(ForEach(&rows, func(r *row) Component {
		return HBox(Text(&r.label).Width(&r.w), Text("|"))
	})))
	buf := NewBuffer(24, 4)
	tmpl.Execute(buf, 24, 4)
	if c0, c1 := strings.Index(buf.GetLine(0), "|"), strings.Index(buf.GetLine(1), "|"); c0 == c1 {
		t.Errorf("per-item Text.Width frozen: '|' at col %d in both rows", c0)
	}
}

func TestForEachPerItemProgressWidthRebinds(t *testing.T) {
	type row struct {
		w int16
		v float64
	}
	rows := []row{{4, 1.0}, {16, 1.0}}
	tmpl := Build(VBox(ForEach(&rows, func(r *row) Component {
		return HBox(Progress(&r.v).Width(&r.w), Text("|"))
	})))
	buf := NewBuffer(28, 4)
	tmpl.Execute(buf, 28, 4)
	if c0, c1 := strings.Index(buf.GetLine(0), "|"), strings.Index(buf.GetLine(1), "|"); c0 == c1 {
		t.Errorf("per-item Progress.Width frozen: '|' at col %d in both rows", c0)
	}
}

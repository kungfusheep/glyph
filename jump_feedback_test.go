package glyph

import "testing"

// ADR 11: incremental multi-char jump-label feedback. Drive paintJumpLabels
// directly with a constructed jump mode and inspect the painted cells.

func paintJump(targets []JumpTarget, input string, style JumpStyle) *Buffer {
	app := &App{jumpMode: &JumpMode{Active: true, Targets: targets, Input: input}, jumpStyle: style}
	buf := NewBuffer(20, 4)
	app.paintJumpLabels(buf, 4)
	return buf
}

// at rest (nothing typed) the render is identical to LabelStyle alone — the
// property that makes default-on safe for existing consumers.
func TestJumpFeedbackAtRestUnchanged(t *testing.T) {
	st := DefaultJumpStyle
	buf := paintJump([]JumpTarget{{X: 0, Y: 0, Label: "as"}}, "", st)
	for i := 0; i < 2; i++ {
		if got := buf.Get(i, 0).Style; !got.Equal(st.LabelStyle) {
			t.Fatalf("at-rest char %d style = %+v, want LabelStyle %+v", i, got, st.LabelStyle)
		}
	}
}

// a typed prefix recedes (dim-derived by default) while the remainder keeps
// LabelStyle, so the next key to press stands out.
func TestJumpFeedbackPrefixDims(t *testing.T) {
	st := DefaultJumpStyle
	buf := paintJump([]JumpTarget{{X: 0, Y: 0, Label: "as"}}, "a", st)
	want := dimDerived(st.LabelStyle)
	if got := buf.Get(0, 0).Style; !got.Equal(want) {
		t.Fatalf("typed prefix 'a' style = %+v, want dim-derived %+v", got, want)
	}
	if got := buf.Get(1, 0).Style; !got.Equal(st.LabelStyle) {
		t.Fatalf("remainder 's' style = %+v, want LabelStyle", got)
	}
}

// a label whose prefix diverged from the input dims whole; a still-matching
// label keeps its prefix-dim + bright remainder.
func TestJumpFeedbackNonMatchingDimsWhole(t *testing.T) {
	st := DefaultJumpStyle
	buf := paintJump([]JumpTarget{
		{X: 0, Y: 0, Label: "as"},
		{X: 0, Y: 1, Label: "df"},
	}, "a", st)
	dim := dimDerived(st.LabelStyle)
	// row 1 "df" is dead: both chars dimmed
	if got := buf.Get(0, 1).Style; !got.Equal(dim) {
		t.Fatalf("dead label char 0 = %+v, want dim", got)
	}
	if got := buf.Get(1, 1).Style; !got.Equal(dim) {
		t.Fatalf("dead label char 1 = %+v, want dim", got)
	}
	// row 0 "as" still live: 'a' dim (typed), 's' bright
	if got := buf.Get(1, 0).Style; !got.Equal(st.LabelStyle) {
		t.Fatalf("live label remainder = %+v, want LabelStyle", got)
	}
}

// an explicit MatchedStyle overrides the derived dim.
func TestJumpFeedbackExplicitMatchedStyle(t *testing.T) {
	st := JumpStyle{LabelStyle: Style{FG: Magenta, Attr: AttrBold}, MatchedStyle: Style{FG: Green}}
	buf := paintJump([]JumpTarget{{X: 0, Y: 0, Label: "as"}}, "a", st)
	if got := buf.Get(0, 0).Style; !got.Equal(st.MatchedStyle) {
		t.Fatalf("typed prefix with explicit MatchedStyle = %+v, want %+v", got, st.MatchedStyle)
	}
	if got := buf.Get(1, 0).Style; !got.Equal(st.LabelStyle) {
		t.Fatalf("remainder = %+v, want LabelStyle", got)
	}
}

func TestDimDerivedDropsBoldAddsDim(t *testing.T) {
	got := dimDerived(Style{FG: Magenta, Attr: AttrBold})
	if got.Attr&AttrBold != 0 {
		t.Fatal("dimDerived kept bold")
	}
	if got.Attr&AttrDim == 0 {
		t.Fatal("dimDerived did not add dim")
	}
}

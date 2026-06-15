package glyph

import "testing"

// incremental multi-char jump-label feedback: drive paintJumpLabels directly
// with a constructed jump mode and inspect the painted cells.

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
	// the recede must be a real colour change, not attr-only — otherwise it is
	// invisible on labels that carry an explicit FG (the diff/calendar case).
	if got.FG != BrightBlack {
		t.Fatalf("dimDerived FG = %+v, want BrightBlack (a visible recede, not faint-only)", got.FG)
	}
}

// TestDimDerivedVisibleOnStyledLabel: a styled label (coloured FG on a coloured
// background band, bold) must produce a recede that is perceptible — not a
// same-colour bold->faint flip nor a foreground nudge the band can swamp.
func TestDimDerivedVisibleOnStyledLabel(t *testing.T) {
	// near-black foreground on a muted-blue band, bold.
	base := Style{FG: Hex(0x1c1c1c), BG: Hex(0x6f8fa8), Attr: AttrBold}
	got := dimDerived(base)
	if got.FG == base.FG {
		t.Fatal("dimDerived left FG unchanged on a styled label — feedback would be invisible")
	}
	// the categorical recede: the chip (BG band) is dropped, so a receded label
	// is plain text, not a coloured chip — unmistakable on any theme. A bare FG
	// nudge on the kept chip (near-black -> grey on the same blue) was too subtle.
	if got.BG != (Color{}) {
		t.Fatalf("dimDerived kept the BG chip (%+v) — recede too subtle on a styled label", got.BG)
	}
}

// TestJumpFeedbackEndToEndMultiChar drives the full path: many targets (>27) so
// GenerateLabels produces two-char labels, collected through Execute's
// AddJumpTarget and AssignLabels, then a partial first char typed. The matching
// labels' typed prefix must recede and the remainder keep LabelStyle;
// non-matching labels recede whole. Guards against the isolated-paint test
// passing while the live collection/assignment path shows nothing.
func TestJumpFeedbackEndToEndMultiChar(t *testing.T) {
	items := make([]string, 30) // >27 forces two-char labels
	for i := range items {
		items[i] = "row"
	}
	app := NewApp()
	tmpl := Build(VBox(ForEach(&items, func(s *string) Component {
		return Jump(Text(s), func() {})
	})))
	tmpl.SetApp(app)
	app.jumpMode.Active = true

	buf := NewBuffer(40, 40)
	app.jumpMode.ClearTargets()
	tmpl.Execute(buf, 40, 40)
	app.jumpMode.AssignLabels()

	if n := len(app.jumpMode.Targets); n < 28 {
		t.Fatalf("expected >27 collected targets for two-char labels, got %d", n)
	}
	// confirm we actually got multi-char labels (else there's no prefix to dim)
	multi := false
	for _, tg := range app.jumpMode.Targets {
		if len(tg.Label) >= 2 {
			multi = true
			break
		}
	}
	if !multi {
		t.Fatalf("expected at least one two-char label among %d targets", len(app.jumpMode.Targets))
	}

	// type the first label's first char; it must be a live partial match
	first := app.jumpMode.Targets[0].Label
	prefix := first[:1]
	if !app.jumpMode.HasPartialMatch(prefix) {
		t.Fatalf("typing %q should be a partial match for %q", prefix, first)
	}
	app.jumpMode.Input = prefix

	out := NewBuffer(40, 40)
	app.paintJumpLabels(out, 40)

	wantDim := dimDerived(DefaultJumpStyle.LabelStyle)
	sawMatchingDim := false
	sawNonMatchingDim := false
	for _, tg := range app.jumpMode.Targets {
		x, y := int(tg.X), int(tg.Y)
		if x >= out.Width() || y >= out.Height() {
			continue
		}
		head := out.Get(x, y).Style
		if tg.Label[:1] == prefix {
			// matching: first char dims, second char keeps LabelStyle
			if head.Equal(wantDim) {
				sawMatchingDim = true
			}
			if len(tg.Label) >= 2 {
				if rem := out.Get(x+1, y).Style; !rem.Equal(DefaultJumpStyle.LabelStyle) {
					t.Errorf("label %q remainder should keep LabelStyle, got %+v", tg.Label, rem)
				}
			}
		} else {
			// non-matching: whole label dims
			if head.Equal(wantDim) {
				sawNonMatchingDim = true
			} else {
				t.Errorf("non-matching label %q head should dim, got %+v", tg.Label, head)
			}
		}
	}
	if !sawMatchingDim {
		t.Error("expected at least one matching label with a dimmed typed prefix")
	}
	if !sawNonMatchingDim {
		t.Error("expected at least one non-matching label dimmed whole")
	}
}

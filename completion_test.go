package glyph

import (
	"testing"

	"github.com/kungfusheep/riffkey"
)

func navHandler(t *testing.T, c *CompletionC, pattern string) func() {
	t.Helper()
	for _, b := range c.bindings() {
		if b.pattern == pattern {
			if h, ok := b.handler.(func()); ok {
				return h
			}
		}
	}
	t.Fatalf("no func() handler for %q", pattern)
	return nil
}

// Trigger arms on the token after the last trigger before the caret; prefix-filters the
// source; opens when there are matches.
func TestCompletionTriggerAndMatches(t *testing.T) {
	src := []string{"Marlow", "Marley", "Quinn", "Sable"}
	c := Complete(&src).Trigger('@')
	c.input.field.Value = "@mar"
	c.input.field.Cursor = 4
	c.recompute()

	if !c.open {
		t.Fatal("should be open with matches for prefix 'mar'")
	}
	if len(c.matches) != 2 || c.matches[0] != "Marlow" || c.matches[1] != "Marley" {
		t.Fatalf("matches = %v, want [Marlow Marley]", c.matches)
	}
}

// A space between the caret and the trigger breaks the token — no completion across words.
func TestCompletionTokenBreaksOnSpace(t *testing.T) {
	src := []string{"Marlow"}
	c := Complete(&src).Trigger('@')
	c.input.field.Value = "@k done"
	c.input.field.Cursor = 7 // after "done"
	c.recompute()
	if c.open {
		t.Fatal("token should not span the space; expected closed")
	}
}

// Default pick replaces the active token (trigger + prefix) with trigger + picked + space.
func TestCompletionPickReplacesToken(t *testing.T) {
	src := []string{"Marlow", "Marley"}
	c := Complete(&src).Trigger('@')
	c.input.field.Value = "hi @mar"
	c.input.field.Cursor = 7
	c.recompute()
	c.moveSel(1) // select "Marley"
	navHandler(t, c, "<Enter>")()

	if c.input.field.Value != "hi @Marley " {
		t.Fatalf("value = %q, want %q", c.input.field.Value, "hi @Marley ")
	}
	if c.open {
		t.Error("pick should close the dropdown")
	}
}

// Enter while closed submits; while open it picks.
func TestCompletionClosedEnterSubmits(t *testing.T) {
	src := []string{"Marlow"}
	submitted := false
	c := Complete(&src).Trigger('@').OnSubmit(func() { submitted = true })
	c.input.field.Value = "no trigger here"
	c.input.field.Cursor = 15
	c.recompute()
	if c.open {
		t.Fatal("no @ → should be closed")
	}
	navHandler(t, c, "<Enter>")()
	if !submitted {
		t.Error("Enter while closed should call OnSubmit")
	}
}

// Esc closes an open dropdown.
func TestCompletionEscCloses(t *testing.T) {
	src := []string{"Marlow"}
	c := Complete(&src).Trigger('@')
	c.input.field.Value = "@m"
	c.input.field.Cursor = 2
	c.recompute()
	if !c.open {
		t.Fatal("should be open")
	}
	c.close()
	if c.open {
		t.Error("Esc should close the dropdown")
	}
}

// moveSel wraps around the match list.
func TestCompletionMoveSelWraps(t *testing.T) {
	src := []string{"a", "ab", "abc"}
	c := Complete(&src)
	c.input.field.Value = "a"
	c.input.field.Cursor = 1
	c.recompute() // 3 matches, sel 0
	c.moveSel(-1)
	if c.sel != 2 {
		t.Fatalf("Up from 0 should wrap to 2, got %d", c.sel)
	}
	c.moveSel(1)
	if c.sel != 0 {
		t.Fatalf("Down from 2 should wrap to 0, got %d", c.sel)
	}
}

// The component builds and expands without panic, and exposes its bindings + textBinding.
func TestCompletionBuildExpands(t *testing.T) {
	src := []string{"Marlow"}
	c := Complete(&src).Trigger('@').Placeholder("message…")
	if len(c.bindings()) != 1 {
		t.Fatalf("want 1 always-on binding (Enter), got %d", len(c.bindings()))
	}
	if c.textBinding() == nil {
		t.Fatal("textBinding must be non-nil so the field's text lands on the nav router")
	}
	tmpl := Build(VBox(c)) // compound components are used as children; the compiler expands templateTree
	buf := NewBuffer(40, 6)
	tmpl.Execute(buf, 40, 6) // must not panic (closed dropdown)
}

// Regression — the Esc-trap: a completion field must NOT swallow Esc when its dropdown
// is closed. Up/Down/Esc are gated to the open state, so when closed a parent (e.g. a
// composer's Esc-to-cancel) receives them; when open, the dropdown's Esc shadows the
// parent and closes the list.
func TestCompletionEscFallsThroughWhenClosed(t *testing.T) {
	src := []string{"Marlow"}
	c := Complete(&src).Trigger('@')
	parentEsc := 0
	app := NewApp()
	app.SetView(VBox(
		On(Key("<Esc>", func() { parentEsc++ })),
		c,
	))

	// dropdown CLOSED (no @ token): Esc must reach the parent.
	app.render()
	app.Input().Dispatch(riffkey.Key{Special: riffkey.SpecialEscape})
	if parentEsc != 1 {
		t.Fatalf("closed dropdown must let Esc through to the parent; parentEsc=%d", parentEsc)
	}

	// open the dropdown, render to activate the gated nav scope, then Esc.
	c.input.field.Value = "@m"
	c.input.field.Cursor = 2
	c.recompute()
	if !c.open {
		t.Fatal("dropdown should be open")
	}
	app.render()
	app.Input().Dispatch(riffkey.Key{Special: riffkey.SpecialEscape})
	if c.open {
		t.Error("open dropdown: Esc should close it")
	}
	if parentEsc != 1 {
		t.Errorf("open dropdown: Esc should be consumed by the dropdown, not reach the parent; parentEsc=%d", parentEsc)
	}
}

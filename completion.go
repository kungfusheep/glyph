package glyph

import "strings"

// CompletionC is an editable field with an anchored autocomplete dropdown — type a
// prefix, a list of matches appears below the caret, ↑/↓ move the selection, Enter
// picks (Enter when the dropdown is closed submits), Esc closes. The field stays
// editable throughout: the nav keys are explicit bindings on the field's own router, so
// they win over text capture while every other key keeps editing (riffkey matches
// explicit patterns before the TextInput HandleUnmatched — no modal capture, no router
// push/pop). See ADR 71.
//
//	Complete(&agentNames).Trigger('@').OnSubmit(send)
//
// With a Trigger rune, only the token after the last trigger before the caret is
// completed (e.g. "@kes" → "@Kestrel "); with no trigger, the whole value is the prefix.
type CompletionC struct {
	source   *[]string
	trigger  rune
	onPick   func(picked string)
	onSubmit func()
	maxRows  int
	rowStyle Style
	selStyle Style

	input   *InputC
	ref     NodeRef
	matches []string // recomputed on change
	sel     int      // highlighted index into matches
	open    bool     // trigger armed AND len(matches) > 0
}

// Complete builds an autocomplete field over a candidate pool the caller owns.
func Complete(source *[]string) *CompletionC {
	c := &CompletionC{
		source:   source,
		maxRows:  8,
		selStyle: Style{Attr: AttrInverse},
	}
	c.input = Input()
	c.input.declaredTIB = &textInputBinding{
		value:    &c.input.field.Value,
		cursor:   &c.input.field.Cursor,
		onChange: func(string) { c.recompute() },
	}
	return c
}

// Trigger limits completion to the token following the last occurrence of r before the
// caret (e.g. '@' for mentions). Zero (default) completes the whole value.
func (c *CompletionC) Trigger(r rune) *CompletionC { c.trigger = r; return c }

// OnPick overrides how a selected candidate is applied. The default replaces the active
// token (trigger + prefix) with trigger + picked + a trailing space.
func (c *CompletionC) OnPick(fn func(picked string)) *CompletionC { c.onPick = fn; return c }

// OnSubmit is invoked when Enter is pressed while the dropdown is closed.
func (c *CompletionC) OnSubmit(fn func()) *CompletionC { c.onSubmit = fn; return c }

// MaxRows caps the dropdown height (default 8).
func (c *CompletionC) MaxRows(n int) *CompletionC { c.maxRows = n; return c }

// Placeholder sets the field's empty-state text.
func (c *CompletionC) Placeholder(p string) *CompletionC { c.input.placeholder = p; return c }

// SelectedStyle sets the highlight style for the active dropdown row.
func (c *CompletionC) SelectedStyle(s Style) *CompletionC { c.selStyle = s; return c }

// Value returns the current field text.
func (c *CompletionC) Value() string { return c.input.field.Value }

// tokenStart returns the rune index where the active prefix begins (just after the last
// trigger before the caret), and whether a trigger is armed. With no trigger, the prefix
// is the whole value [0, cursor).
func (c *CompletionC) tokenStart() (int, bool) {
	runes := []rune(c.input.field.Value)
	cur := c.input.field.Cursor
	if cur > len(runes) {
		cur = len(runes)
	}
	if c.trigger == 0 {
		return 0, true
	}
	for i := cur - 1; i >= 0; i-- {
		if runes[i] == c.trigger {
			return i + 1, true
		}
		// a space between caret and trigger breaks the token (don't complete across words)
		if runes[i] == ' ' {
			return 0, false
		}
	}
	return 0, false
}

// recompute re-derives matches + open-state from the value (called on every edit).
func (c *CompletionC) recompute() {
	c.matches = c.matches[:0]
	start, armed := c.tokenStart()
	if !armed || c.source == nil {
		c.open = false
		return
	}
	runes := []rune(c.input.field.Value)
	cur := c.input.field.Cursor
	if cur > len(runes) {
		cur = len(runes)
	}
	prefix := strings.ToLower(string(runes[start:cur]))
	for _, cand := range *c.source {
		if strings.HasPrefix(strings.ToLower(cand), prefix) {
			c.matches = append(c.matches, cand)
		}
	}
	if c.sel >= len(c.matches) {
		c.sel = 0
	}
	c.open = len(c.matches) > 0
}

// pick applies the highlighted candidate and closes the dropdown.
func (c *CompletionC) pick() {
	if c.sel < 0 || c.sel >= len(c.matches) {
		return
	}
	picked := c.matches[c.sel]
	if c.onPick != nil {
		c.onPick(picked)
	} else {
		c.applyDefault(picked)
	}
	c.open = false
	c.matches = c.matches[:0]
	c.sel = 0
}

// applyDefault replaces the active token (trigger + prefix) with trigger + picked + " ".
func (c *CompletionC) applyDefault(picked string) {
	runes := []rune(c.input.field.Value)
	cur := c.input.field.Cursor
	if cur > len(runes) {
		cur = len(runes)
	}
	start, _ := c.tokenStart()
	insert := picked + " "
	if c.trigger != 0 {
		insert = string(c.trigger) + insert
		start-- // include the trigger rune itself in the replaced span
		if start < 0 {
			start = 0
		}
	}
	newRunes := append([]rune{}, runes[:start]...)
	newRunes = append(newRunes, []rune(insert)...)
	newRunes = append(newRunes, runes[cur:]...)
	c.input.field.Value = string(newRunes)
	c.input.field.Cursor = start + len([]rune(insert))
}

func (c *CompletionC) moveSel(delta int) {
	if !c.open || len(c.matches) == 0 {
		return
	}
	c.sel = (c.sel + delta + len(c.matches)) % len(c.matches)
}

// toTemplate expands to the field (wrapped so the dropdown can anchor to its rect)
// plus, when open, an anchored dropdown of matches.
func (c *CompletionC) toTemplate() Component {
	return VBox(
		HBox.NodeRef(&c.ref)(c.input),
		If(&c.open).Then(
			Overlay.Below(&c.ref)(c.renderDropdown()),
		),
	)
}

// renderDropdown is one Custom that draws the visible matches each frame, highlighting
// the selected row — read live, so it follows typing and ↑/↓ with no rebuild.
func (c *CompletionC) renderDropdown() Component {
	measure := func(availW int16) (int16, int16) {
		w := 0
		for _, m := range c.matches {
			if sw := StringWidth(m); sw > w {
				w = sw
			}
		}
		h := len(c.matches)
		if h > c.maxRows {
			h = c.maxRows
		}
		return int16(w), int16(h)
	}
	render := func(buf *Buffer, x, y, w, h int16) {
		for i := 0; i < int(h) && i < len(c.matches); i++ {
			st := c.rowStyle
			if i == c.sel {
				st = c.selStyle
			}
			buf.WriteStringClipped(int(x), int(y)+i, c.matches[i], st, int(w))
		}
	}
	return Custom(measure, render)
}

// bindings declares the nav keys on the field's router (explicit → they beat the text
// HandleUnmatched while open; every other key keeps editing). Handlers switch on
// open-state so the field behaves normally when the dropdown is closed.
func (c *CompletionC) bindings() []binding {
	return []binding{
		{pattern: "<Up>", handler: func() { c.moveSel(-1) }},
		{pattern: "<Down>", handler: func() { c.moveSel(1) }},
		{pattern: "<Enter>", handler: func() {
			if c.open {
				c.pick()
			} else if c.onSubmit != nil {
				c.onSubmit()
			}
		}},
		{pattern: "<Esc>", handler: func() {
			if c.open {
				c.open = false
				c.matches = c.matches[:0]
			}
		}},
	}
}

// textBinding hands the field's text input to the framework so it lands on the same
// router as the nav bindings above.
func (c *CompletionC) textBinding() *textInputBinding { return c.input.textBinding() }

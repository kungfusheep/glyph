package glyph

import (
	"strings"
	"unicode"

	"github.com/kungfusheep/riffkey"
)

// KeyHelpC renders the key bindings currently in effect, live. It reads its source
// every frame, so it always shows what's bound right now — switch view, push a
// modal, or move focus and it updates with no extra wiring. Only Named() bindings
// appear (see KeyC.Named and App.ActiveBindings); unnamed bindings aren't
// introspectable, by design.
type KeyHelpC struct {
	source    func() []riffkey.Binding
	keyStyle  Style
	descStyle Style
	gap       int  // columns between the key column and the label
	humanize  bool // render "scroll-down" as "Scroll down"
}

// KeyHelp builds a live key-help component from a binding source — typically
// app.ActiveBindings. Each frame it lists the active named bindings as aligned
// "<key>  <label>" rows.
//
// Modal vs non-modal — which source to pass:
//   - NON-MODAL surface (an always-on key bar / HUD over the live view): pass
//     app.ActiveBindings directly. It reads the active router every frame, so the help
//     follows view/pane/focus changes live.
//   - MODAL overlay (e.g. a "?" cheatsheet opened with On.Modal): the overlay's router
//     becomes the active one while it's up, so a live app.ActiveBindings would show the
//     OVERLAY's own keys, not the view beneath. Snapshot the bindings ONCE when the
//     overlay opens (before the modal pushes) and pass a source returning that snapshot:
//
//	snap := app.ActiveBindings()        // capture the view's keys at open
//	KeyHelp(func() []riffkey.Binding { return snap })
//
//     This is also better UX — the underlying view is frozen while you read, and
//     re-snapshotting on each open keeps it current.
func KeyHelp(source func() []riffkey.Binding) *KeyHelpC {
	return &KeyHelpC{source: source, gap: 2, humanize: true}
}

// KeyFG sets the foreground colour of the key column.
func (h *KeyHelpC) KeyFG(c Color) *KeyHelpC { h.keyStyle.FG = c; return h }

// DescFG sets the foreground colour of the label column.
func (h *KeyHelpC) DescFG(c Color) *KeyHelpC { h.descStyle.FG = c; return h }

// Gap sets the number of columns between the key and its label (default 2).
func (h *KeyHelpC) Gap(n int) *KeyHelpC { h.gap = n; return h }

// Raw shows the binding's raw name (e.g. "scroll-down") instead of humanising it.
func (h *KeyHelpC) Raw() *KeyHelpC { h.humanize = false; return h }

func (h *KeyHelpC) Build() Component { return h }

func (h *KeyHelpC) rows() []riffkey.Binding {
	if h.source == nil {
		return nil
	}
	return h.source()
}

func (h *KeyHelpC) label(name string) string {
	if !h.humanize {
		return name
	}
	return humanizeBindingName(name)
}

// keyColWidth is the display width of the widest key pattern, so labels align.
func (h *KeyHelpC) keyColWidth(rows []riffkey.Binding) int {
	w := 0
	for _, b := range rows {
		if kw := StringWidth(b.Pattern); kw > w {
			w = kw
		}
	}
	return w
}

func (h *KeyHelpC) MinSize() (int, int) {
	rows := h.rows()
	keyW := h.keyColWidth(rows)
	labelW := 0
	for _, b := range rows {
		if lw := StringWidth(h.label(b.Name)); lw > labelW {
			labelW = lw
		}
	}
	width := keyW
	if labelW > 0 {
		width += h.gap + labelW
	}
	return width, len(rows)
}

func (h *KeyHelpC) Render(buf *Buffer, x, y, w, height int) {
	rows := h.rows()
	keyW := h.keyColWidth(rows)
	labelX := keyW + h.gap
	for i, b := range rows {
		if i >= height {
			break // viewport-bounded: never paint past the allocated rows
		}
		row := y + i
		buf.WriteStringClipped(x, row, b.Pattern, h.keyStyle, w)
		if rem := w - labelX; rem > 0 {
			buf.WriteStringClipped(x+labelX, row, h.label(b.Name), h.descStyle, rem)
		}
	}
}

// humanizeBindingName turns a semantic binding name into a readable label:
// "scroll-down"/"scroll_down" -> "Scroll down". Already-spaced names pass through
// with just a leading capital.
func humanizeBindingName(name string) string {
	if name == "" {
		return ""
	}
	s := strings.Map(func(r rune) rune {
		if r == '-' || r == '_' {
			return ' '
		}
		return r
	}, name)
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

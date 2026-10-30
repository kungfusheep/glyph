package glyph

import "strings"

// Terminal replies. A query is a plain write. The terminal answers on the
// input stream, riffkey hands the answer up as a reply Key, and the input loop
// brings it here instead of to the routers. Nothing waits on a reply, so a
// terminal that stays quiet costs the app nothing, and the first frame never
// waits on the terminal.

// OnTerminalReply registers fn to receive every terminal reply, after the app
// has taken what it uses from it: for queries the program sends on its own,
// and for watching the wire. raw is the whole sequence, ESC to terminator.
// fn returns true to redraw.
func (a *App) OnTerminalReply(fn func(raw string) bool) *App {
	a.replyFn = fn
	return a
}

// handleReply routes one terminal reply. Default colours and the basic-16
// palette come from the startup colour query; the keyboard reply answers the
// OSC 523 query. Every reply then goes to OnTerminalReply.
func (a *App) handleReply(raw string) bool {
	handled := false
	switch {
	case strings.HasPrefix(raw, "\x1b]10;"), strings.HasPrefix(raw, "\x1b]11;"):
		handled = a.applyDefaultColor(raw)
	case strings.HasPrefix(raw, "\x1b]4;"):
		handled = a.applyPaletteColor(raw)
	case strings.HasPrefix(raw, softKeyboardOSC+"?"):
		body := strings.TrimSuffix(strings.TrimSuffix(raw, softKeyboardST), "\x07")
		a.handleKeyboardReply(body[len(softKeyboardOSC)+1:])
	}
	if a.replyFn != nil && a.replyFn(raw) {
		handled = true
	}
	return handled
}

// applyDefaultColor takes an OSC 10 or 11 reply into the default style so
// opacity blends and post-processing lerp toward the terminal's real colours.
// Caller-supplied SetDefaultStyle values still win.
func (a *App) applyDefaultColor(raw string) bool {
	c := parseOSCColor([]byte(raw), raw[3])
	if c.Mode != ColorRGB {
		return false
	}
	a.renderMu.Lock()
	defer a.renderMu.Unlock()
	if raw[3] == '0' {
		a.defaultFG = c
		if a.defaultStyle.FG.Mode == ColorDefault {
			a.defaultStyle.FG = c
		}
	} else {
		a.defaultBG = c
		if a.defaultStyle.BG.Mode == ColorDefault {
			a.defaultStyle.BG = c
		}
	}
	if a.pool != nil {
		for _, buf := range a.pool.buffers {
			buf.defaultStyle = a.defaultStyle
		}
	}
	a.RequestRender()
	return true
}

// applyPaletteColor takes an OSC 4 reply into the basic-16 table.
func (a *App) applyPaletteColor(raw string) bool {
	data := []byte(raw)
	for i := range 16 {
		if c := parseOSC4Color(data, i); c.Mode == ColorRGB {
			basic16RGB[i] = [3]uint8{c.R, c.G, c.B}
			refreshBasic16Vars()
			a.RequestRender()
			return true
		}
	}
	return false
}

package glyph

import (
	"strings"
	"sync"
)

// Soft keyboard protocol (OSC 523). A touch terminal raises its on-screen
// keyboard on a tap unless the program says otherwise. With Mouse(true) the app
// takes control at startup, then raises the keyboard for views that declare
// one, for focused Input components, and for explicit ShowKeyboard calls, and
// lowers it everywhere else. The app sends the detection query and its first
// report together and reads the reply through the input loop, so nothing
// waits at startup: a terminal without the protocol ignores the reports, and
// one that answers device=none stops them.
// Spec: https://github.com/kungfusheep/OSC-523

// Keyboard describes the on-screen keyboard a view or component wants. The
// zero value is a plain text keyboard with a Return key.
type Keyboard struct {
	Type      string // KeyboardText, KeyboardSearch, ... ; empty means text
	Enter     string // EnterReturn, EnterSearch, ... ; empty means return
	Secure    bool   // password keyboard: suggestions and learning off
	NoCorrect bool   // autocorrect and prediction off
}

// Keyboard types, the HTML inputmode vocabulary.
const (
	KeyboardText    = "text"
	KeyboardDecimal = "decimal"
	KeyboardNumeric = "numeric"
	KeyboardTel     = "tel"
	KeyboardSearch  = "search"
	KeyboardEmail   = "email"
	KeyboardURL     = "url"
)

// Return key labels.
const (
	EnterReturn = "return"
	EnterDone   = "done"
	EnterGo     = "go"
	EnterSend   = "send"
	EnterSearch = "search"
	EnterNext   = "next"
)

const (
	softKeyboardHide    = "control=app:keyboard=hide"
	softKeyboardRelease = "control=terminal"
	softKeyboardOSC     = "\x1b]523;"
	softKeyboardST      = "\x1b\\"
)

// report is the body of the OSC 523 sequence that asks for this keyboard.
func (k Keyboard) report() string {
	var b strings.Builder
	b.WriteString("control=app:keyboard=show")
	if k.Type != "" {
		b.WriteString(":type=")
		b.WriteString(k.Type)
	}
	if k.Enter != "" {
		b.WriteString(":enter=")
		b.WriteString(k.Enter)
	}
	if k.Secure {
		b.WriteString(":secure=on")
	}
	if k.NoCorrect {
		b.WriteString(":correct=off")
	}
	return b.String()
}

// softKeyboard is the app's side of the protocol: what it last asked for and
// the inputs that decide what it asks for next.
type softKeyboard struct {
	mu           sync.Mutex
	enabled      bool                // reports go out; set at startup when the mouse is on
	device       string              // the terminal's reply to the query: soft, none, or empty for no reply yet
	last         string              // last report body sent, so repeats cost nothing
	override     *Keyboard           // ShowKeyboard in effect until HideKeyboard
	inputFocused bool                // a FocusManager item has focus
	inputHint    *Keyboard           // that item's declared keyboard, if any
	views        map[string]Keyboard // keyboard declared per view name
	onReport     func(string)        // OnKeyboardReport: every body sent, query included
}

// OnKeyboardReport registers fn to receive the body of every OSC 523 sequence
// the app sends, the detection query included, as it goes out. For status
// lines and wire logs.
func (a *App) OnKeyboardReport(fn func(body string)) *App {
	a.kb.mu.Lock()
	a.kb.onReport = fn
	a.kb.mu.Unlock()
	return a
}

// sendKeyboard writes one body and tells OnKeyboardReport. Called with kb.mu
// held; the hook runs after it is released so it may read KeyboardReport.
func (a *App) sendKeyboard(body string) {
	a.screen.emitSoftKeyboard(body)
	fn := a.kb.onReport
	if fn == nil {
		return
	}
	a.kb.mu.Unlock()
	fn(body)
	a.kb.mu.Lock()
}

// Keyboard declares the on-screen keyboard this view wants while it is the
// active view. Views without a declaration lower the keyboard.
func (vb *ViewBuilder) Keyboard(k Keyboard) *ViewBuilder {
	vb.app.kb.mu.Lock()
	if vb.app.kb.views == nil {
		vb.app.kb.views = make(map[string]Keyboard)
	}
	vb.app.kb.views[vb.name] = k
	vb.app.kb.mu.Unlock()
	return vb
}

// ShowKeyboard raises the on-screen keyboard now, whatever the active view
// declares, until HideKeyboard. For overlays that take text, such as a command
// palette, where a view declaration does not fit.
func (a *App) ShowKeyboard(k Keyboard) {
	a.kb.mu.Lock()
	a.kb.override = &k
	a.kb.mu.Unlock()
	a.syncKeyboard()
}

// HideKeyboard ends a ShowKeyboard. The keyboard then follows the active view
// again, so it stays up inside a view that declares one.
func (a *App) HideKeyboard() {
	a.kb.mu.Lock()
	a.kb.override = nil
	a.kb.mu.Unlock()
	a.syncKeyboard()
}

// setInputFocused records whether a FocusManager item holds focus, and which
// keyboard it declared, and brings the keyboard in line.
func (a *App) setInputFocused(focused bool, hint *Keyboard) {
	a.kb.mu.Lock()
	a.kb.inputFocused = focused
	a.kb.inputHint = hint
	a.kb.mu.Unlock()
	a.syncKeyboard()
}

// KeyboardReport is the body of the last OSC 523 report sent, for status
// lines and debugging. Empty until the first report.
func (a *App) KeyboardReport() string {
	a.kb.mu.Lock()
	defer a.kb.mu.Unlock()
	return a.kb.last
}

// KeyboardDevice is the terminal's answer to the detection query: "soft" when
// the device has an on-screen keyboard, "none" when it has none and reports
// are accepted and ignored, and "" while no reply has arrived, which is also
// what a terminal without the protocol leaves it at.
func (a *App) KeyboardDevice() string {
	a.kb.mu.Lock()
	defer a.kb.mu.Unlock()
	return a.kb.device
}

// handleKeyboardReply takes the pairs after "?" in a detection reply. A
// device=none answer stops further reports; control is still handed back on
// exit because the screen remembers it was taken.
func (a *App) handleKeyboardReply(pairs string) {
	a.kb.mu.Lock()
	defer a.kb.mu.Unlock()
	for _, pair := range strings.Split(pairs, ":") {
		k, v, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok || strings.TrimSpace(k) != "device" {
			continue
		}
		a.kb.device = strings.TrimSpace(v)
		if a.kb.device == "none" {
			a.kb.enabled = false
		}
	}
}

// topView is the view the user sees: the top of the modal stack, else the
// current view.
func (a *App) topView() string {
	if n := len(a.viewStack); n > 0 {
		return a.viewStack[n-1]
	}
	return a.currentView
}

// syncKeyboard sends the report the current state calls for, if it differs
// from the last one sent. Precedence: ShowKeyboard override, then a focused
// Input, then the active view's declaration, else hide.
func (a *App) syncKeyboard() {
	a.kb.mu.Lock()
	defer a.kb.mu.Unlock()
	if !a.kb.enabled {
		return
	}
	body := softKeyboardHide
	switch {
	case a.kb.override != nil:
		body = a.kb.override.report()
	case a.kb.inputFocused && a.kb.inputHint != nil:
		body = a.kb.inputHint.report()
	case a.kb.inputFocused:
		body = Keyboard{}.report()
	default:
		if k, ok := a.kb.views[a.topView()]; ok {
			body = k.report()
		}
	}
	if body == a.kb.last {
		return
	}
	a.kb.last = body
	a.sendKeyboard(body)
}

// startSoftKeyboard sends the detection query and takes control of the
// keyboard when the mouse is on. The reply, if one comes, reaches
// handleKeyboardReply through the input loop.
func (a *App) startSoftKeyboard() {
	if !a.mouse {
		return
	}
	a.kb.mu.Lock()
	a.sendKeyboard("?")
	a.kb.enabled = true
	a.kb.mu.Unlock()
	a.syncKeyboard()
}

// emitSoftKeyboard writes one report and remembers whether it took control, so
// ExitRawMode can hand control back.
func (s *Screen) emitSoftKeyboard(body string) {
	s.emit(wrapTmuxPassthrough([]byte(softKeyboardOSC + body + softKeyboardST)))
	s.kbHeld = strings.HasPrefix(body, "control=app")
}

// releaseSoftKeyboard hands tap handling back to the terminal if the app took
// it. Safe to call more than once.
func (s *Screen) releaseSoftKeyboard() {
	if !s.kbHeld {
		return
	}
	s.emitSoftKeyboard(softKeyboardRelease)
}

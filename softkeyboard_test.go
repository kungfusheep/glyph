package glyph

import (
	"bytes"
	"regexp"
	"testing"
)

// newKeyboardApp builds an app with the mouse on and keyboard reports enabled,
// skipping the terminal query, and returns the buffer the reports land in.
func newKeyboardApp() (*App, *bytes.Buffer) {
	var out bytes.Buffer
	app := NewApp().Mouse(true)
	app.screen.writer = &out
	app.pool = NewBufferPool(40, 10)
	app.kb.enabled = true
	return app, &out
}

var reportRE = regexp.MustCompile(`\x1b\]523;([^\x1b]*)\x1b\\`)

// reports returns the bodies of every OSC 523 sequence written so far.
func reports(out *bytes.Buffer) []string {
	var got []string
	for _, m := range reportRE.FindAllSubmatch(out.Bytes(), -1) {
		got = append(got, string(m[1]))
	}
	return got
}

func assertReports(t *testing.T, out *bytes.Buffer, want ...string) {
	t.Helper()
	got := reports(out)
	if len(got) != len(want) {
		t.Fatalf("reports = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("report %d = %q, want %q (all: %q)", i, got[i], want[i], got)
		}
	}
}

func TestKeyboardReportCarriesOnlyTheHintsSet(t *testing.T) {
	if got := (Keyboard{}).report(); got != "control=app:keyboard=show" {
		t.Fatalf("plain = %q", got)
	}
	k := Keyboard{Type: KeyboardSearch, Enter: EnterSearch, Secure: true, NoCorrect: true}
	want := "control=app:keyboard=show:type=search:enter=search:secure=on:correct=off"
	if got := k.report(); got != want {
		t.Fatalf("full = %q, want %q", got, want)
	}
}

func TestKeyboardFollowsTheActiveView(t *testing.T) {
	app, out := newKeyboardApp()
	app.View("home", Text("home"))
	app.View("search", Text("search")).Keyboard(Keyboard{Type: KeyboardSearch, Enter: EnterSearch})

	app.Go("home")
	app.Go("search")
	app.Go("home")
	app.Go("home") // repeat costs nothing
	assertReports(t, out,
		"control=app:keyboard=hide",
		"control=app:keyboard=show:type=search:enter=search",
		"control=app:keyboard=hide",
	)
}

func TestPushedViewRaisesAndPopRestores(t *testing.T) {
	app, out := newKeyboardApp()
	app.View("home", Text("home"))
	app.View("search", Text("search")).Keyboard(Keyboard{Type: KeyboardSearch})
	app.Go("home")
	app.PushView("search")
	app.PopView()
	assertReports(t, out,
		"control=app:keyboard=hide",
		"control=app:keyboard=show:type=search",
		"control=app:keyboard=hide",
	)
}

func TestShowKeyboardOverridesAndHideReturnsToTheView(t *testing.T) {
	app, out := newKeyboardApp()
	app.View("compose", Text("compose")).Keyboard(Keyboard{Enter: EnterSend})
	app.Go("compose")
	app.ShowKeyboard(Keyboard{}) // a command palette opened over compose
	app.HideKeyboard()           // palette closed: compose still wants its keyboard
	assertReports(t, out,
		"control=app:keyboard=show:enter=send",
		"control=app:keyboard=show",
		"control=app:keyboard=show:enter=send",
	)
}

func TestFocusedInputRaisesTheKeyboard(t *testing.T) {
	app, out := newKeyboardApp()
	fm := NewFocusManager()
	var name, pin string
	app.View("form", VBox(
		Input(&name).ManagedBy(fm),
		Input(&pin).ManagedBy(fm).Keyboard(Keyboard{Type: KeyboardNumeric, Secure: true}),
	))
	app.View("home", Text("home"))

	app.Go("form") // first registered input takes focus on activation
	fm.Next()      // the pin field declares its own keyboard
	fm.BlurCurrent()
	app.Go("home")
	assertReports(t, out,
		"control=app:keyboard=show",
		"control=app:keyboard=show:type=numeric:secure=on",
		"control=app:keyboard=hide",
	)
}

func TestNoReportsWithoutTheMouse(t *testing.T) {
	var out bytes.Buffer
	app := NewApp()
	app.screen.writer = &out
	app.pool = NewBufferPool(40, 10)
	app.View("search", Text("search")).Keyboard(Keyboard{Type: KeyboardSearch})
	app.Go("search")
	app.ShowKeyboard(Keyboard{})
	assertReports(t, &out)
}

func TestExitReleasesControlOnlyWhenTaken(t *testing.T) {
	var out bytes.Buffer
	s := NewScreen(&out)
	s.releaseSoftKeyboard()
	assertReports(t, &out)

	s.emitSoftKeyboard("control=app:keyboard=hide")
	s.releaseSoftKeyboard()
	s.releaseSoftKeyboard() // second release is a no-op
	assertReports(t, &out, "control=app:keyboard=hide", "control=terminal")
}

func TestStartupSendsTheQueryThenTheFirstReport(t *testing.T) {
	var out bytes.Buffer
	app := NewApp().Mouse(true)
	app.screen.writer = &out
	app.pool = NewBufferPool(40, 10)
	app.View("home", Text("home"))
	app.Go("home") // before startup: nothing goes out
	assertReports(t, &out)
	app.startSoftKeyboard()
	assertReports(t, &out, "?", "control=app:keyboard=hide")
	if app.KeyboardDevice() != "" {
		t.Fatalf("device before any reply = %q", app.KeyboardDevice())
	}
}

func TestDeviceNoneReplyStopsReports(t *testing.T) {
	app, out := newKeyboardApp()
	app.View("home", Text("home"))
	app.View("search", Text("search")).Keyboard(Keyboard{Type: KeyboardSearch})
	app.Go("home")
	app.handleReply("\x1b]523;?:types=text:device=none\x07")
	app.Go("search")
	assertReports(t, out, "control=app:keyboard=hide")
	if app.KeyboardDevice() != "none" {
		t.Fatalf("device = %q, want none", app.KeyboardDevice())
	}
	// the screen still remembers control was taken, so exit hands it back
	app.screen.releaseSoftKeyboard()
	assertReports(t, out, "control=app:keyboard=hide", "control=terminal")
}

func TestDeviceSoftReplyKeepsReporting(t *testing.T) {
	app, out := newKeyboardApp()
	app.View("home", Text("home"))
	app.View("search", Text("search")).Keyboard(Keyboard{Type: KeyboardSearch})
	app.Go("home")
	app.handleReply("\x1b]523;?:device=soft:types=text,search:enters=return,search\x1b\\")
	app.Go("search")
	assertReports(t, out, "control=app:keyboard=hide", "control=app:keyboard=show:type=search")
	if app.KeyboardDevice() != "soft" {
		t.Fatalf("device = %q, want soft", app.KeyboardDevice())
	}
}

func TestOnKeyboardReportSeesEveryBodySent(t *testing.T) {
	var out bytes.Buffer
	app := NewApp().Mouse(true)
	app.screen.writer = &out
	app.pool = NewBufferPool(40, 10)
	var got []string
	app.OnKeyboardReport(func(body string) { got = append(got, body+"|"+app.KeyboardReport()) })
	app.View("home", Text("home"))
	app.View("search", Text("search")).Keyboard(Keyboard{Type: KeyboardSearch})
	app.Go("home")
	app.startSoftKeyboard()
	app.Go("search")
	want := []string{"?|", "control=app:keyboard=hide|control=app:keyboard=hide",
		"control=app:keyboard=show:type=search|control=app:keyboard=show:type=search"}
	if len(got) != len(want) {
		t.Fatalf("hook got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("hook call %d = %q, want %q", i, got[i], want[i])
		}
	}
	assertReports(t, &out, "?", "control=app:keyboard=hide", "control=app:keyboard=show:type=search")
}

// a managed input is a jump target: a tap on it focuses it through the
// FocusManager, so the keyboard report follows, and a tap on a blurred field
// brings it back.
func TestTapOnAnInputFocusesItAndSendsItsKeyboard(t *testing.T) {
	app, out := newKeyboardApp()
	fm := NewFocusManager()
	var name, pin string
	app.View("form", VBox(
		Input(&name).Width(10).ManagedBy(fm),
		Input(&pin).Width(10).ManagedBy(fm).Keyboard(Keyboard{Type: KeyboardNumeric}),
	))
	app.Go("form")
	renderWithMouse(app, app.viewTemplates["form"], 20, 4)

	if !app.handleMouse(press(3, 1)) {
		t.Fatal("tap on the second field was not handled")
	}
	if fm.Current() != 1 {
		t.Fatalf("focus after tap = %d, want 1", fm.Current())
	}
	fm.BlurCurrent()
	if !app.handleMouse(press(3, 1)) || fm.Current() != 1 {
		t.Fatal("tap on the blurred field did not refocus it")
	}
	if !app.handleMouse(press(0, 0)) || fm.Current() != 0 {
		t.Fatal("tap on the first field did not focus it")
	}
	if app.handleMouse(press(0, 3)) {
		t.Fatal("tap on empty space was handled")
	}
	assertReports(t, out,
		"control=app:keyboard=show",
		"control=app:keyboard=show:type=numeric",
		"control=app:keyboard=hide",
		"control=app:keyboard=show:type=numeric",
		"control=app:keyboard=show",
	)
}

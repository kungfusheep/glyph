// softkeyboard is a small touch-first app for trying the Soft Keyboard
// Protocol (OSC 523) in a terminal that implements it.
//
//	go run ./examples/softkeyboard
//	go run ./examples/softkeyboard -log /tmp/kb.log
//
// The home view is a tappable list: taps select rows and no keyboard should
// appear. The search view declares a search keyboard. The sign-in form has
// three inputs with different keyboards.
//
// A wire log sits at the top of every view: each OSC 523 sequence the app
// sends, and each reply the terminal sends back, colour replies included,
// stamped with the time since startup. -log appends the same lines to a file
// for reading after a run on a phone.
//
// To see the raw bytes on any terminal:
//
//	script -q /tmp/kb.typescript go run ./examples/softkeyboard
//	grep -ao $'\e]523;[^\e]*' /tmp/kb.typescript
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	. "github.com/kungfusheep/glyph"
	"github.com/kungfusheep/riffkey"
)

const logLines = 6

// wire keeps the last few sequences that crossed the pty, newest last.
type wire struct {
	mu    sync.Mutex
	start time.Time
	lines []string
	file  *os.File
}

// add records one sequence. dir is "→" for sent and "←" for received.
func (w *wire) add(dir, raw string) {
	line := fmt.Sprintf("%+6dms %s %s", time.Since(w.start).Milliseconds(), dir, printable(raw))
	w.mu.Lock()
	w.lines = append(w.lines, line)
	if len(w.lines) > logLines {
		w.lines = w.lines[len(w.lines)-logLines:]
	}
	w.mu.Unlock()
	if w.file != nil {
		fmt.Fprintln(w.file, line)
	}
}

// line returns entry i of the visible window, blank when nothing is there
// yet, so the panel keeps its height.
func (w *wire) line(i int) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if i < len(w.lines) {
		return w.lines[i]
	}
	return ""
}

// printable spells out the control bytes of a sequence.
func printable(raw string) string {
	r := strings.NewReplacer("\x1b]", "OSC ", "\x1b[", "CSI ", "\x1b\\", " ST", "\x07", " BEL", "\x1b", "ESC ")
	return r.Replace(raw)
}

func main() {
	logPath := flag.String("log", "", "append the wire log to this file")
	flag.Parse()

	w := &wire{start: time.Now()}
	if *logPath != "" {
		f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		w.file = f
	}

	app := NewApp().Mouse(true)
	app.OnKeyboardReport(func(body string) { w.add("→", "\x1b]523;"+body+"\x1b\\") })
	app.OnTerminalReply(func(raw string) bool { w.add("←", raw); return true })

	// the wire panel sits at the top of every view so it never moves with
	// the content under it
	panel := VBox(
		HBox.Gap(2)(
			Text("wire log").FG(Cyan).Bold(),
			Text(func() string {
				if d := app.KeyboardDevice(); d != "" {
					return "terminal replied device=" + d
				}
				return "no 523 reply yet"
			}).FG(BrightBlack),
		),
		Text(func() string { return w.line(0) }).FG(BrightBlack),
		Text(func() string { return w.line(1) }).FG(BrightBlack),
		Text(func() string { return w.line(2) }).FG(BrightBlack),
		Text(func() string { return w.line(3) }).FG(BrightBlack),
		Text(func() string { return w.line(4) }).FG(BrightBlack),
		Text(func() string { return w.line(5) }).FG(BrightBlack),
		HRule().FG(BrightBlack),
	)

	rows := []string{
		"Northgate dashboard: v2 handoff",
		"Deploy succeeded: northgate-dashboard",
		"Weekly usage summary",
		"Invoice 1042 is ready",
		"Re: print proofs for the spring issue",
		"Offsite agenda, draft two",
	}
	selected := 0
	tapped := "tap a row, or the buttons below"
	query := ""

	app.View("home",
		VBox.PaddingTRBL(1, 2, 0, 2)(
			panel,
			Text("Soft keyboard demo").FG(Cyan).Bold(),
			Text(&tapped).FG(BrightBlack),
			SpaceH(1),
			List(&rows).
				Selection(&selected).
				Marker("› ").
				SelectedStyle(Style{Attr: AttrBold}).
				OnTap(func(s *string) { tapped = "tapped: " + *s }),
			SpaceH(1),
			HBox.Gap(2)(
				Tap(Text("[ search ]").FG(Green), func() { app.Go("search") }),
				Tap(Text("[ sign in ]").FG(Green), func() { app.Go("login") }),
				Tap(Text("[ quit ]").FG(Red), app.Stop),
			),
		),
	).
		Handle("j", func() { selected = min(selected+1, len(rows)-1) }).
		Handle("k", func() { selected = max(selected-1, 0) }).
		Handle("/", func() { app.Go("search") }).
		Handle("l", func() { app.Go("login") }).
		Handle("q", app.Stop)

	app.View("search",
		VBox.PaddingTRBL(1, 2, 0, 2)(
			panel,
			Text("Search").FG(Cyan).Bold(),
			Text("a search keyboard with a Search return key; Enter or Esc goes back").FG(BrightBlack),
			SpaceH(1),
			HBox(Text("/ ").FG(Green), Text(&query)),
			SpaceH(1),
			Tap(Text("[ back ]").FG(Green), func() { app.Go("home") }),
		),
	).
		NoCounts().
		Keyboard(Keyboard{Type: KeyboardSearch, Enter: EnterSearch}).
		Handle("<CR>", func() { app.Go("home") }).
		Handle("<Esc>", func() { app.Go("home") }).
		Handle("<BS>", func() {
			if n := len(query); n > 0 {
				query = query[:n-1]
			}
		})
	if r, ok := app.ViewRouter("search"); ok {
		r.HandleUnmatched(func(k riffkey.Key) bool {
			if k.Rune == 0 || k.Mod != 0 {
				return false
			}
			query += string(k.Rune)
			app.RequestRender()
			return true
		})
	}

	var email, password, pin string
	app.View("login",
		VBox.PaddingTRBL(1, 2, 0, 2)(
			panel,
			Text("Sign in").FG(Cyan).Bold(),
			Text("Tab moves between fields; each declares its keyboard; Enter goes back").FG(BrightBlack),
			SpaceH(1),
			Form.Gap(1).LabelFG(BrightWhite).OnSubmit(func(*FormC) { app.Go("home") })(
				Field("Email", Input(&email).Placeholder("you@example.com").Width(28).
					Keyboard(Keyboard{Type: KeyboardEmail, Enter: EnterNext})),
				Field("Password", Input(&password).Placeholder("password").Width(28).Mask('*').
					Keyboard(Keyboard{Secure: true, Enter: EnterNext})),
				Field("PIN", Input(&pin).Placeholder("4 digits").Width(28).
					Keyboard(Keyboard{Type: KeyboardNumeric, Enter: EnterGo})),
			),
			SpaceH(1),
			Tap(Text("[ back ]").FG(Green), func() { app.Go("home") }),
		),
	).NoCounts()

	if err := app.RunFrom("home"); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("last report sent: %s\n", app.KeyboardReport())
}

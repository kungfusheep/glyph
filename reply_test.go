package glyph

import "testing"

func newReplyApp() *App {
	app := NewApp()
	app.pool = NewBufferPool(40, 10)
	return app
}

func TestDefaultColorReplyUpdatesTheDefaultStyle(t *testing.T) {
	app := newReplyApp()
	if !app.handleReply("\x1b]11;rgb:1e1e/1e1e/2e2e\x1b\\") {
		t.Fatal("OSC 11 reply not handled")
	}
	want := Color{Mode: ColorRGB, R: 0x1e, G: 0x1e, B: 0x2e}
	if app.defaultBG != want {
		t.Fatalf("defaultBG = %+v, want %+v", app.defaultBG, want)
	}
	if app.defaultStyle.BG != want {
		t.Fatalf("defaultStyle.BG = %+v, want %+v", app.defaultStyle.BG, want)
	}
	for _, buf := range app.pool.buffers {
		if buf.defaultStyle.BG != want {
			t.Fatalf("pool buffer defaultStyle.BG = %+v, want %+v", buf.defaultStyle.BG, want)
		}
	}
	if !app.handleReply("\x1b]10;rgb:ffff/ffff/ffff\x07") {
		t.Fatal("OSC 10 reply not handled")
	}
	if app.defaultFG != (Color{Mode: ColorRGB, R: 255, G: 255, B: 255}) {
		t.Fatalf("defaultFG = %+v", app.defaultFG)
	}
}

func TestSetDefaultStyleWinsOverTheColorReply(t *testing.T) {
	app := newReplyApp()
	mine := Color{Mode: ColorRGB, R: 1, G: 2, B: 3}
	app.defaultStyle.BG = mine
	app.handleReply("\x1b]11;rgb:1e1e/1e1e/2e2e\x1b\\")
	if app.defaultStyle.BG != mine {
		t.Fatalf("defaultStyle.BG = %+v, want the caller's %+v", app.defaultStyle.BG, mine)
	}
	if app.defaultBG.Mode != ColorRGB {
		t.Fatal("defaultBG should still record the terminal's colour")
	}
}

func TestPaletteReplyUpdatesBasic16(t *testing.T) {
	saved := basic16RGB
	defer func() { basic16RGB = saved; refreshBasic16Vars() }()
	app := newReplyApp()
	if !app.handleReply("\x1b]4;12;rgb:1234/5678/9abc\x07") {
		t.Fatal("OSC 4 reply not handled")
	}
	if basic16RGB[12] != [3]uint8{0x12, 0x56, 0x9a} {
		t.Fatalf("basic16RGB[12] = %v", basic16RGB[12])
	}
	if app.handleReply("\x1b]4;12;?\x07") {
		t.Fatal("a reply without a colour reported handled")
	}
}

func TestOnTerminalReplySeesEveryReply(t *testing.T) {
	app := newReplyApp()
	var got []string
	app.OnTerminalReply(func(raw string) bool { got = append(got, raw); return false })
	if !app.handleReply("\x1b]11;rgb:1e1e/1e1e/2e2e\x1b\\") {
		t.Fatal("the colour reply lost its handled result")
	}
	app.handleReply("\x1b]523;?:device=soft\x1b\\")
	if app.handleReply("\x1b[?2033;2$y") {
		t.Fatal("a reply the hook declined reported handled")
	}
	want := []string{"\x1b]11;rgb:1e1e/1e1e/2e2e\x1b\\", "\x1b]523;?:device=soft\x1b\\", "\x1b[?2033;2$y"}
	if len(got) != len(want) {
		t.Fatalf("hook got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("hook reply %d = %q, want %q", i, got[i], want[i])
		}
	}
	if app.KeyboardDevice() != "soft" {
		t.Fatal("the app stopped using the 523 reply itself")
	}
	app.OnTerminalReply(func(string) bool { return true })
	if !app.handleReply("\x1b[?1;2c") {
		t.Fatal("the hook asking for a redraw was ignored")
	}
	app.OnTerminalReply(nil)
	if app.handleReply("\x1b[?1;2c") {
		t.Fatal("an unclaimed reply reported handled")
	}
}

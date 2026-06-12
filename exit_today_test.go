package glyph

import (
	"strings"
	"testing"
	"time"
)

// Pete's question on proposal #7: In().Out() already handles exit states —
// does it work per-item inside ForEach? Pattern: each row's content sits in
// an If(&item.Alive) branch carrying Opacity(In(1).Out(Animate(0))); the TTL
// event flips Alive, the retained-branch exit machinery fades the row out
// per item, and OnComplete drives the slice removal.
func TestToastExitWithInOutVocabulary(t *testing.T) {
	type Toast struct {
		Msg   string
		Alive bool
	}
	toasts := []Toast{{Msg: "first", Alive: true}, {Msg: "second", Alive: true}}
	completed := 0

	tmpl := Build(VBox(
		ForEach(&toasts, func(to *Toast) Component {
			return If(&to.Alive).Then(
				HBox.Opacity(In(1.0).Out(
					Animate.Duration(200 * time.Millisecond).Ease(EaseLinear).
						OnComplete(func() { completed++ })(0.0),
				))(Text(&to.Msg)),
			)
		}),
	))

	base := time.Unix(5000, 0)
	clock := base
	tmpl.nowFn = func() time.Time { return clock }

	buf := NewBuffer(20, 4)
	tmpl.Execute(buf, 20, 4)
	if got := buf.GetLine(0); got != "first" {
		t.Fatalf("line 0 = %q, want first", got)
	}

	// TTL event
	toasts[0].Alive = false

	clock = base.Add(50 * time.Millisecond)
	buf2 := NewBuffer(20, 4)
	tmpl.Execute(buf2, 20, 4)
	if !tmpl.Animating() {
		t.Fatal("exiting toast must mark the template animating")
	}
	if got := buf2.GetLine(0); !strings.Contains(got, "first") {
		t.Fatalf("mid-exit line 0 = %q, want retained ghost still rendering", got)
	}
	if completed != 0 {
		t.Fatal("OnComplete fired before the exit finished")
	}

	clock = base.Add(400 * time.Millisecond)
	buf3 := NewBuffer(20, 4)
	tmpl.Execute(buf3, 20, 4)
	if completed != 1 {
		t.Fatalf("OnComplete fired %d times, want exactly 1", completed)
	}

	// app removes the item on completion; survivor reflows
	toasts = toasts[:copy(toasts, toasts[1:])]
	clock = base.Add(450 * time.Millisecond)
	buf4 := NewBuffer(20, 4)
	tmpl.Execute(buf4, 20, 4)
	if got := buf4.GetLine(0); got != "second" {
		t.Fatalf("after removal line 0 = %q, want survivor reflowed up", got)
	}
	if tmpl.Animating() {
		t.Fatal("still animating after exit completed and item removed")
	}
}

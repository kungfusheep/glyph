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

// the stale-NodeRef contract (proposal #27): a ref zeroes when its node stops
// rendering — but a node mid Out-animation is RETAINED and still rendered, so its
// ref must stay live through the whole fade and zero only once the branch is truly
// dropped. (If it zeroed when the condition first flipped, a closing overlay's dodge
// ref would release mid-fade and the effect would darken it — the #379 stacking.)
func TestNodeRefStaysLiveThroughExitAnimationThenZeroes(t *testing.T) {
	alive := true
	var ref NodeRef
	tmpl := Build(VBox(
		If(&alive).Then(
			VBox.NodeRef(&ref).Opacity(In(1.0).Out(
				Animate.Duration(200 * time.Millisecond).Ease(EaseLinear)(0.0),
			))(Text("overlay")),
		),
	))
	base := time.Unix(9000, 0)
	clock := base
	tmpl.nowFn = func() time.Time { return clock }

	buf := NewBuffer(20, 4)
	tmpl.Execute(buf, 20, 4)
	if ref.W == 0 || ref.H == 0 {
		t.Fatalf("ref should be live while shown: W%d H%d", ref.W, ref.H)
	}

	// close it: condition false, but the branch retains + animates out
	alive = false
	clock = base.Add(50 * time.Millisecond)
	buf.Clear()
	tmpl.Execute(buf, 20, 4)
	if !tmpl.Animating() {
		t.Fatal("exiting branch should mark the template animating")
	}
	if ref.W == 0 || ref.H == 0 {
		t.Fatalf("ref must STAY LIVE mid Out-animation (retained branch still renders): W%d H%d", ref.W, ref.H)
	}

	// fade completes; branch is dropped and no longer rendered
	clock = base.Add(400 * time.Millisecond)
	buf.Clear()
	tmpl.Execute(buf, 20, 4)
	clock = base.Add(450 * time.Millisecond)
	buf.Clear()
	tmpl.Execute(buf, 20, 4)
	if ref.W != 0 || ref.H != 0 {
		t.Errorf("ref must ZERO once the exiting branch is dropped: W%d H%d", ref.W, ref.H)
	}
}

// regression for the frozen per-item watch target: a tween watching an ITEM
// FIELD inside ForEach must follow each element's value, not the compile-time
// dummy's. This is the field-watch form of the toast fade.
func TestPerItemTweenWatchesItemField(t *testing.T) {
	type Toast struct {
		Msg   string
		Alpha float64
	}
	toasts := []Toast{{Msg: "first", Alpha: 1}, {Msg: "second", Alpha: 1}}
	completed := 0

	fade := Animate.Duration(200 * time.Millisecond).Ease(EaseLinear).
		OnComplete(func() { completed++ })

	tmpl := Build(VBox(
		ForEach(&toasts, func(to *Toast) Component {
			return HBox.Opacity(fade(&to.Alpha))(Text(&to.Msg))
		}),
	))

	base := time.Unix(6000, 0)
	clock := base
	tmpl.nowFn = func() time.Time { return clock }

	buf := NewBuffer(20, 4)
	tmpl.Execute(buf, 20, 4)

	toasts[0].Alpha = 0 // TTL event on the first item only

	clock = base.Add(50 * time.Millisecond)
	buf2 := NewBuffer(20, 4)
	tmpl.Execute(buf2, 20, 4)
	if !tmpl.Animating() {
		t.Fatal("per-item field tween did not animate; watch target frozen on the dummy")
	}

	clock = base.Add(400 * time.Millisecond)
	buf3 := NewBuffer(20, 4)
	tmpl.Execute(buf3, 20, 4)
	if completed != 1 {
		t.Fatalf("OnComplete fired %d times, want exactly 1 (first item only)", completed)
	}
	if tmpl.Animating() {
		t.Fatal("still animating after the single fade settled")
	}
}

// same regression, Color flavour: Fill tween watching an item colour field.
func TestPerItemColorTweenWatchesItemField(t *testing.T) {
	type Row struct {
		Label string
		Tone  Color
	}
	rows := []Row{{Label: "a", Tone: RGB(0, 0, 0)}, {Label: "b", Tone: RGB(0, 0, 0)}}

	tmpl := Build(VBox(
		ForEach(&rows, func(r *Row) Component {
			return HBox.Height(1).Fill(Animate.Duration(200*time.Millisecond).Ease(EaseLinear)(&r.Tone))(
				Text(&r.Label),
			)
		}),
	))

	base := time.Unix(7000, 0)
	clock := base
	tmpl.nowFn = func() time.Time { return clock }

	buf := NewBuffer(10, 4)
	tmpl.Execute(buf, 10, 4)

	rows[0].Tone = RGB(200, 0, 0) // event on the first row only

	clock = base.Add(50 * time.Millisecond)
	buf2 := NewBuffer(10, 4)
	tmpl.Execute(buf2, 10, 4)
	if !tmpl.Animating() {
		t.Fatal("per-item colour tween did not animate; watch target frozen on the dummy")
	}

	clock = base.Add(400 * time.Millisecond)
	buf3 := NewBuffer(10, 4)
	tmpl.Execute(buf3, 10, 4)
	bg0 := buf3.Get(2, 0).Style.BG
	if bg0.R != 200 || bg0.G != 0 {
		t.Fatalf("row 0 settled fill = %+v, want (200,0,0)", bg0)
	}
	bg1 := buf3.Get(2, 1).Style.BG
	if bg1.R != 0 {
		t.Fatalf("row 1 fill = %+v, want untouched black", bg1)
	}
}

// pins the OnComplete safety contract: completions fire AFTER Execute's
// reads finish, so a callback may mutate bound state — including removing
// the completed item from the very slice its ForEach iterates.
func TestOnCompleteMayMutateIteratedSlice(t *testing.T) {
	type Toast struct {
		Msg   string
		Alive bool
	}
	var toasts []Toast
	toasts = []Toast{{Msg: "first", Alive: true}, {Msg: "second", Alive: true}}

	var tmpl *Template
	tmpl = Build(VBox(
		ForEach(&toasts, func(to *Toast) Component {
			return If(&to.Alive).Then(
				HBox.Opacity(In(1.0).Out(
					Animate.Duration(100 * time.Millisecond).Ease(EaseLinear).
						OnComplete(func() {
							// remove dead items in place — the hazard case
							live := toasts[:0]
							for _, x := range toasts {
								if x.Alive {
									live = append(live, x)
								}
							}
							toasts = live
						})(0.0),
				))(Text(&to.Msg)),
			)
		}),
	))

	base := time.Unix(8000, 0)
	clock := base
	tmpl.nowFn = func() time.Time { return clock }

	buf := NewBuffer(20, 4)
	tmpl.Execute(buf, 20, 4)

	toasts[0].Alive = false

	clock = base.Add(50 * time.Millisecond)
	buf2 := NewBuffer(20, 4)
	tmpl.Execute(buf2, 20, 4) // mid-fade

	clock = base.Add(200 * time.Millisecond)
	buf3 := NewBuffer(20, 4)
	tmpl.Execute(buf3, 20, 4) // completion frame: callback mutates the slice AFTER reads
	if len(toasts) != 1 {
		t.Fatalf("len(toasts) = %d after completion, want 1 (callback ran)", len(toasts))
	}

	clock = base.Add(250 * time.Millisecond)
	buf4 := NewBuffer(20, 4)
	tmpl.Execute(buf4, 20, 4)
	if got := buf4.GetLine(0); got != "second" {
		t.Fatalf("after in-callback removal line 0 = %q, want survivor", got)
	}
}

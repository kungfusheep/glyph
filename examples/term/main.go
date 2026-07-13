// Command term is a basic tmux clone built on the glyph terminal component: it
// tiles panes, each running a shell on its own pty, with a tmux-style prefix
// key (Ctrl-B) for splits and focus. It proves the term component embeds like
// any other glyph component.
//
// Keys: Ctrl-B then
//
//	%   split the focused pane side by side
//	"   split it stacked
//	o   cycle focus
//	x   close the focused pane
//
// Every other key goes to the focused shell. The app exits when the last shell
// does.
package main

import (
	"log"
	"strconv"
	"sync"

	. "github.com/kungfusheep/glyph"
	termpkg "github.com/kungfusheep/glyph/term"
	"github.com/kungfusheep/riffkey"
)

// maxPanes bounds the pane pool. The view is compiled once, so every pane a
// session can ever hold must exist as a child of the Arrange at compile time.
//
// A pool is needed because a Layer-backed component cannot be driven from a
// ForEach: the compiled op holds the layer pointer captured at compile time, so
// every item would render the same pane's buffer. Panes therefore get a slot
// each, and the layout hands unused slots a zero rect.
//
// An unused slot costs nothing: a zero-size box makes the component skip its
// frame, so it never opens a pty. Shells start only when a slot is given a real
// box.
const maxPanes = 16

// ui owns the pane tree and serialises every structural change. Mutations run
// under mu from the input and pty-reader goroutines; the layout function reads
// the tree under the same lock on the render goroutine.
type ui struct {
	// apply marshals a mutation onto the render goroutine, to run at frame top
	// before Execute reads anything. Every write to bound state goes through it:
	// the template iterates `chips` with no host lock, and pane exits arrive on a
	// pty reader goroutine, so a plain mutex on the write side cannot make that
	// read safe. App.Apply is the seam glyph provides for exactly this.
	apply  func(func())
	render func() // repaint request
	stop   func() // last shell exited

	mu   sync.Mutex
	tree *tree

	slots    [maxPanes]*termpkg.TermC
	free     []int
	nextName int

	// the screen box, tracked from resize events. The custom-layout callback is
	// handed availH == 0 by the framework, so the height it lays out with has to
	// come from here rather than from the layout call.
	w, h int

	// the layout's scratch rects, reused every frame (see layout)
	rects [maxPanes]Rect

	// chips is the status line's bound model, rewritten whenever the pane set or
	// the focus changes. The template reads it every frame.
	chips []chip
}

// chip is one pane's entry in the status line. Label is derived at the point the
// pane set changes, so the template only ever reads it.
type chip struct {
	Label   string
	Focused bool
}

// newUI builds the pane pool and the initial single-pane tree. tune, when set,
// configures every terminal in the pool; tests use it to pin a deterministic
// shell.
func newUI(apply func(func()), render, stop func(), tune func(*termpkg.TermC)) *ui {
	u := &ui{apply: apply, render: render, stop: stop}
	for i := maxPanes - 1; i >= 0; i-- {
		slot := i
		t := termpkg.New().
			Grow(0). // the Arrange sizes each pane; flex-grow would fight it
			OnUpdate(render).
			OnExit(func(error) { u.onSlotExit(slot) })
		if tune != nil {
			tune(t)
		}
		u.slots[i] = t
		u.free = append(u.free, i)
	}
	u.tree = newTree(u.newPane())
	u.sync()
	return u
}

// resize records the screen box the layout works from.
func (u *ui) resize(w, h int) {
	u.apply(func() {
		u.mu.Lock()
		u.w, u.h = w, h
		u.mu.Unlock()
	})
}

// view is the whole screen, compiled once. Splits, focus changes and pane deaths
// mutate the tree and the chip slice; the layout function and the template
// re-read them every frame. Nothing here is ever rebuilt.
//
// The pane area is an Arrange. It cannot flex-grow (a custom-layout container
// gets no grow factor), so it takes its height from the rects the layout returns
// rather than from the box it is given.
func (u *ui) view() Component {
	return VBox(
		Arrange(u.layout)(u.paneComponents()...),
		HBox(
			Text(" glyph-term ").Bold(),
			ForEach(&u.chips, func(c *chip) Component {
				return If(&c.Focused).
					Then(Text(&c.Label).FG(Black).BG(Green)).
					Else(Text(&c.Label).Dim())
			}),
			Text(`   C-b:  % vsplit   " hsplit   o next   x close`),
		),
	)
}

func main() {
	app := NewApp()
	u := newUI(app.Apply, app.RequestRender, app.Stop, nil)

	app.SetView(u.view())

	sz := app.Size()
	u.resize(int(sz.Width), int(sz.Height))
	app.OnResize(u.resize)

	app.Handle("<C-b>%", func() { u.split(true) })
	app.Handle("<C-b>\"", func() { u.split(false) })
	app.Handle("<C-b>o", func() { u.focusNext() })
	app.Handle("<C-b>x", func() { u.closeFocused() })

	// Every unbound key goes to the focused shell. Ctrl-B is held by riffkey as
	// a sequence prefix, so it never leaks here.
	app.Router().HandleUnmatched(func(k riffkey.Key) bool {
		u.mu.Lock()
		f := u.tree.focused
		u.mu.Unlock()
		if f != nil && !f.dead {
			return u.slots[f.slot].HandleKey(k)
		}
		return false
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// paneComponents returns the pool in slot order. The Arrange addresses its
// children by index, so this order is the contract the layout function relies on.
func (u *ui) paneComponents() []Component {
	out := make([]Component, maxPanes)
	for i, t := range u.slots {
		out[i] = t
	}
	return out
}

// layout places each live pane's slot and hands every unused slot a zero rect,
// which parks it. It runs on the render goroutine every frame.
//
// availH arrives as 0 from the framework, so the pane height comes from the
// tracked screen box, less the status row. The rects it returns are what give the
// container its height, so returning them IS how the pane area gets sized.
func (u *ui) layout(_ []ChildSize, availW, _ int) []Rect {
	u.mu.Lock()
	defer u.mu.Unlock()

	// reused across frames: this runs on the render path, and the caller copies
	// the rects into child geometry before returning, so it never retains them.
	rects := u.rects[:]
	clear(rects)

	w, h := u.w, u.h
	if availW > 0 {
		w = availW
	}
	if w <= 0 || h <= 1 {
		return rects // no geometry yet
	}

	placeNode(u.tree.root, Rect{X: 0, Y: 0, W: w, H: h - 1}, rects)
	return rects
}

// placeNode divides a box down the split tree, writing each leaf's rect into its
// pane's slot. Children share their parent's box equally; the last one absorbs
// the rounding remainder so no column is ever dropped.
func placeNode(n *node, box Rect, rects []Rect) {
	if n == nil {
		return
	}
	if n.leaf != nil {
		rects[n.leaf.slot] = box
		return
	}
	kids := n.split.children
	if len(kids) == 0 {
		return
	}
	if n.split.horizontal {
		w := box.W / len(kids)
		for i, c := range kids {
			cw := w
			if i == len(kids)-1 {
				cw = box.W - w*(len(kids)-1)
			}
			placeNode(c, Rect{X: box.X + w*i, Y: box.Y, W: cw, H: box.H}, rects)
		}
		return
	}
	h := box.H / len(kids)
	for i, c := range kids {
		ch := h
		if i == len(kids)-1 {
			ch = box.H - h*(len(kids)-1)
		}
		placeNode(c, Rect{X: box.X, Y: box.Y + h*i, W: box.W, H: ch}, rects)
	}
}

// newPane claims a slot for a new shell. Caller holds u.mu (or is in setup).
// It returns nil when the pool is exhausted.
func (u *ui) newPane() *pane {
	if len(u.free) == 0 {
		return nil
	}
	slot := u.free[len(u.free)-1]
	u.free = u.free[:len(u.free)-1]

	name := strconv.Itoa(u.nextName)
	u.nextName++
	return &pane{slot: slot, name: name}
}

// sync pushes tree state into the parts of the world that are not the tree: the
// focus flag on each terminal, and the status line's chip model. It runs at every
// mutation, never during render. Caller holds u.mu (or is in setup).
func (u *ui) sync() {
	leaves := u.tree.leaves()
	u.chips = u.chips[:0]
	for _, p := range leaves {
		u.slots[p.slot].Focus(p == u.tree.focused)
		u.chips = append(u.chips, chip{
			Label:   " " + p.name + " ",
			Focused: p == u.tree.focused,
		})
	}
}

func (u *ui) split(horizontal bool) {
	u.apply(func() {
		u.mu.Lock()
		defer u.mu.Unlock()
		if p := u.newPane(); p != nil {
			u.tree.splitFocused(horizontal, p)
		}
		u.sync()
	})
}

func (u *ui) focusNext() {
	u.apply(func() {
		u.mu.Lock()
		defer u.mu.Unlock()
		u.tree.focusNext()
		u.sync()
	})
}

func (u *ui) closeFocused() {
	u.mu.Lock()
	f := u.tree.focused
	u.mu.Unlock()
	if f != nil {
		// the reader hits EOF and onSlotExit prunes the pane
		u.slots[f.slot].Close()
	}
}

// onSlotExit prunes the pane whose shell exited and releases its slot back to
// the pool.
//
// It is called on that pane's pty-reader goroutine, so it does NOT mutate here:
// it marshals the whole change onto the render goroutine. Rewriting `chips`
// from this goroutine races the template iterating it.
func (u *ui) onSlotExit(slot int) {
	u.apply(func() {
		u.mu.Lock()
		for _, p := range u.tree.leaves() {
			if p.slot == slot {
				p.dead = true
			}
		}
		u.tree.prune()
		u.free = append(u.free, slot)
		empty := len(u.tree.leaves()) == 0
		u.sync()
		u.mu.Unlock()

		if empty {
			u.stop()
		}
	})
}

// Command term is a basic tmux clone built on the glyph terminal component: it
// tiles panes, each running a shell on its own pty, with a tmux-style prefix
// key (Ctrl-B) for splits and focus. It proves the term component embeds like
// any other glyph Renderer.
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
//
// Not full-screen apps yet: vim/htop need the alt-screen buffer, which is a
// later slice of the component. A plain shell (ls, cat, pipelines) renders.
package main

import (
	"log"
	"strconv"
	"sync"

	. "github.com/kungfusheep/glyph"
	termpkg "github.com/kungfusheep/glyph/term"
	"github.com/kungfusheep/riffkey"
)

// ui owns the pane tree and serialises every structural change. All mutations
// and the SetView rebuild happen under mu, so a shell exiting on its reader
// goroutine never races a split fired from the input goroutine.
type ui struct {
	app      *App
	mu       sync.Mutex
	tree     *tree
	nextName int
}

func main() {
	app := NewApp()
	u := &ui{app: app}
	u.tree = newTree(u.newPane())

	app.SetView(buildView(u.tree))

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
		if f != nil && !f.dead && f.term != nil {
			return f.term.HandleKey(k)
		}
		return false
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// newPane creates a shell pane wired to repaint on output and to prune itself
// when its shell exits.
func (u *ui) newPane() *pane {
	name := strconv.Itoa(u.nextName)
	u.nextName++
	p := &pane{name: name}
	p.term = termpkg.New().
		Grow(1).
		OnUpdate(u.app.RequestRender).
		OnExit(func(error) { u.onPaneExit(p) })
	return p
}

func (u *ui) split(horizontal bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.tree.splitFocused(horizontal, u.newPane())
	u.rebuild()
}

func (u *ui) focusNext() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.tree.focusNext()
	u.rebuild()
}

func (u *ui) closeFocused() {
	u.mu.Lock()
	f := u.tree.focused
	u.mu.Unlock()
	if f != nil {
		f.dead = true
		f.term.Close() // reader hits EOF → onPaneExit prunes and rebuilds
	}
}

func (u *ui) onPaneExit(p *pane) {
	u.mu.Lock()
	defer u.mu.Unlock()
	p.dead = true
	u.tree.prune()
	if len(u.tree.leaves()) == 0 {
		u.app.Stop()
		return
	}
	u.rebuild()
}

// rebuild reflects the current tree into the view. Caller holds u.mu.
func (u *ui) rebuild() {
	u.app.SetView(buildView(u.tree))
	u.app.RequestRender()
}

// buildView renders the pane tree above a status line.
func buildView(t *tree) Component {
	return VBox(
		buildNode(t.root, t.focused),
		statusLine(t),
	)
}

// buildNode recurses the split tree into nested boxes. Each pane and each split
// grows to share its parent's space equally.
func buildNode(n *node, focused *pane) Component {
	if n.leaf != nil {
		n.leaf.term.Focus(n.leaf == focused)
		return n.leaf.term
	}
	kids := make([]Component, len(n.split.children))
	for i, c := range n.split.children {
		kids[i] = buildNode(c, focused)
	}
	if n.split.horizontal {
		return HBox.Grow(1)(kids...)
	}
	return VBox.Grow(1)(kids...)
}

// statusLine names the panes, highlighting the focused one, and shows the key
// hints.
func statusLine(t *tree) Component {
	kids := []Component{Text(" glyph-term ").Bold()}
	for _, p := range t.leaves() {
		chip := Text(" " + p.name + " ")
		if p == t.focused {
			chip = chip.FG(Black).BG(Green)
		}
		kids = append(kids, chip)
	}
	kids = append(kids, Text(`   C-b:  % vsplit   " hsplit   o next   x close`))
	return HBox(kids...)
}

package main

// pane is one terminal in the layout. slot indexes the compiled pane pool, which
// is how a pane reaches its terminal component and its rect. dead is set when the
// shell exits, so the next prune removes it.
type pane struct {
	slot int
	name string
	dead bool
}

// node is a binary layout tree. Exactly one of leaf/split is set: a leaf holds
// a pane, a split arranges child nodes side by side or stacked.
type node struct {
	leaf  *pane
	split *split
}

type split struct {
	horizontal bool // true = children side by side (tmux %); false = stacked (tmux ")
	children   []*node
}

// tree is the pane layout with a focus pointer.
type tree struct {
	root    *node
	focused *pane
}

func newTree(first *pane) *tree {
	return &tree{root: &node{leaf: first}, focused: first}
}

// leaves returns panes in left-to-right, top-to-bottom tree order.
func (t *tree) leaves() []*pane {
	var out []*pane
	var walk func(n *node)
	walk = func(n *node) {
		if n == nil {
			return
		}
		if n.leaf != nil {
			out = append(out, n.leaf)
			return
		}
		for _, c := range n.split.children {
			walk(c)
		}
	}
	walk(t.root)
	return out
}

// focusNext cycles focus to the next pane in tree order, wrapping around.
func (t *tree) focusNext() {
	ls := t.leaves()
	if len(ls) == 0 {
		return
	}
	idx := 0
	for i, p := range ls {
		if p == t.focused {
			idx = i
			break
		}
	}
	t.focused = ls[(idx+1)%len(ls)]
}

// splitFocused replaces the focused leaf with a split of [oldPane, newPane] and
// moves focus to the new pane. horizontal true places them side by side.
func (t *tree) splitFocused(horizontal bool, newPane *pane) {
	n := t.findNode(t.focused)
	if n == nil {
		return
	}
	n.split = &split{
		horizontal: horizontal,
		children:   []*node{{leaf: n.leaf}, {leaf: newPane}},
	}
	n.leaf = nil
	t.focused = newPane
}

func (t *tree) findNode(p *pane) *node {
	var res *node
	var walk func(n *node)
	walk = func(n *node) {
		if res != nil || n == nil {
			return
		}
		if n.leaf == p {
			res = n
			return
		}
		if n.split != nil {
			for _, c := range n.split.children {
				walk(c)
			}
		}
	}
	walk(t.root)
	return res
}

// prune removes dead panes and collapses splits left with a single child. It
// keeps focus valid, moving it to the first surviving pane if the focused one
// died.
func (t *tree) prune() {
	t.root = pruneNode(t.root)
	ls := t.leaves()
	for _, p := range ls {
		if p == t.focused {
			return // focus still valid
		}
	}
	if len(ls) > 0 {
		t.focused = ls[0]
	} else {
		t.focused = nil
	}
}

func pruneNode(n *node) *node {
	if n == nil {
		return nil
	}
	if n.leaf != nil {
		if n.leaf.dead {
			return nil
		}
		return n
	}
	var kept []*node
	for _, c := range n.split.children {
		if pc := pruneNode(c); pc != nil {
			kept = append(kept, pc)
		}
	}
	switch len(kept) {
	case 0:
		return nil
	case 1:
		return kept[0] // collapse a split down to its lone survivor
	default:
		n.split.children = kept
		return n
	}
}

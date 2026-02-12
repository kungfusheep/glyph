package forme

import "github.com/kungfusheep/riffkey"

// focusable is implemented by components that can receive keyboard focus.
type focusable interface {
	// focusBinding returns the text input binding for this component
	focusBinding() *textInputBinding
	// setFocused updates the component's visual focus state
	setFocused(focused bool)
}

// FocusManager coordinates keyboard focus across multiple components.
// It automatically wires Tab/Shift-Tab for focus cycling and routes
// keystrokes to the currently focused component.
//
// usage:
//
//	fm := NewFocusManager()
//	name := Input().Placeholder("Name").ManagedBy(fm)
//	email := Input().Placeholder("Email").ManagedBy(fm)
//	app.SetView(VBox(name, email))
type FocusManager struct {
	items    []*focusItem
	current  int
	handlers []*riffkey.TextHandler

	nextKey  string
	prevKey  string
	onChange func(index int) // called when focus changes
}

type focusItem struct {
	focusable focusable
	tib       *textInputBinding
}

// NewFocusManager creates a new focus manager with default Tab/Shift-Tab bindings.
func NewFocusManager() *FocusManager {
	return &FocusManager{
		nextKey: "<Tab>",
		prevKey: "<S-Tab>",
	}
}

// Register adds a focusable component to the manager.
// The first registered component receives initial focus.
func (fm *FocusManager) Register(f focusable) *FocusManager {
	tib := f.focusBinding()
	fm.items = append(fm.items, &focusItem{
		focusable: f,
		tib:       tib,
	})

	// create handler for this item
	if tib != nil {
		h := riffkey.NewTextHandler(tib.value, tib.cursor)
		h.OnChange = tib.onChange
		fm.handlers = append(fm.handlers, h)
	} else {
		fm.handlers = append(fm.handlers, nil)
	}

	// first component gets focus
	if len(fm.items) == 1 {
		f.setFocused(true)
	}

	return fm
}

// NextKey sets the key binding for moving to the next focusable (default: Tab).
func (fm *FocusManager) NextKey(key string) *FocusManager {
	fm.nextKey = key
	return fm
}

// PrevKey sets the key binding for moving to the previous focusable (default: Shift-Tab).
func (fm *FocusManager) PrevKey(key string) *FocusManager {
	fm.prevKey = key
	return fm
}

// OnChange sets a callback that fires when focus changes.
func (fm *FocusManager) OnChange(fn func(index int)) *FocusManager {
	fm.onChange = fn
	return fm
}

// Next moves focus to the next component.
func (fm *FocusManager) Next() {
	fm.moveFocus(1)
}

// Prev moves focus to the previous component.
func (fm *FocusManager) Prev() {
	fm.moveFocus(-1)
}

func (fm *FocusManager) moveFocus(delta int) {
	if len(fm.items) <= 1 {
		return
	}
	fm.items[fm.current].focusable.setFocused(false)
	fm.current = (fm.current + len(fm.items) + delta) % len(fm.items)
	fm.items[fm.current].focusable.setFocused(true)
	if fm.onChange != nil {
		fm.onChange(fm.current)
	}
}

// Focus sets focus to a specific index.
func (fm *FocusManager) Focus(index int) {
	if index < 0 || index >= len(fm.items) {
		return
	}
	if fm.current == index {
		return
	}
	fm.items[fm.current].focusable.setFocused(false)
	fm.current = index
	fm.items[fm.current].focusable.setFocused(true)
	if fm.onChange != nil {
		fm.onChange(fm.current)
	}
}

// Current returns the currently focused index.
func (fm *FocusManager) Current() int {
	return fm.current
}

// HandleKey routes a key to the currently focused component.
func (fm *FocusManager) HandleKey(k riffkey.Key) bool {
	if len(fm.handlers) == 0 {
		return false
	}
	h := fm.handlers[fm.current]
	if h == nil {
		return false
	}
	return h.HandleKey(k)
}

// bindings returns the focus cycling key bindings.
func (fm *FocusManager) bindings() []binding {
	var binds []binding
	if fm.nextKey != "" {
		binds = append(binds, binding{pattern: fm.nextKey, handler: func(_ riffkey.Match) { fm.Next() }})
	}
	if fm.prevKey != "" {
		binds = append(binds, binding{pattern: fm.prevKey, handler: func(_ riffkey.Match) { fm.Prev() }})
	}
	return binds
}

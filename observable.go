package tui

// Observable is a generic data container that notifies on changes.
// It separates data management from UI representation.
type Observable[T any] struct {
	items     []T
	listeners []func(Change[T])
}

// Change describes a modification to the observable.
type Change[T any] struct {
	Type  ChangeType
	Index int
	Item  T // For Add/Update, the new value
	Old   T // For Update/Remove, the old value
}

type ChangeType int

const (
	ChangeAdd ChangeType = iota
	ChangeUpdate
	ChangeRemove
	ChangeClear
	ChangeSet // Full replacement
)

// NewObservable creates a new observable list.
func NewObservable[T any]() *Observable[T] {
	return &Observable[T]{}
}

// Items returns all items.
func (o *Observable[T]) Items() []T {
	return o.items
}

// Len returns the number of items.
func (o *Observable[T]) Len() int {
	return len(o.items)
}

// At returns the item at index i, or zero value if out of bounds.
func (o *Observable[T]) At(i int) T {
	if i < 0 || i >= len(o.items) {
		var zero T
		return zero
	}
	return o.items[i]
}

// Set replaces all items.
func (o *Observable[T]) Set(items []T) *Observable[T] {
	o.items = items
	o.notify(Change[T]{Type: ChangeSet})
	return o
}

// Add appends an item.
func (o *Observable[T]) Add(item T) *Observable[T] {
	idx := len(o.items)
	o.items = append(o.items, item)
	o.notify(Change[T]{Type: ChangeAdd, Index: idx, Item: item})
	return o
}

// Insert inserts an item at index i.
func (o *Observable[T]) Insert(i int, item T) *Observable[T] {
	if i < 0 {
		i = 0
	}
	if i > len(o.items) {
		i = len(o.items)
	}
	o.items = append(o.items[:i], append([]T{item}, o.items[i:]...)...)
	o.notify(Change[T]{Type: ChangeAdd, Index: i, Item: item})
	return o
}

// RemoveAt removes the item at index i.
func (o *Observable[T]) RemoveAt(i int) *Observable[T] {
	if i < 0 || i >= len(o.items) {
		return o
	}
	old := o.items[i]
	o.items = append(o.items[:i], o.items[i+1:]...)
	o.notify(Change[T]{Type: ChangeRemove, Index: i, Old: old})
	return o
}

// Update modifies the item at index i.
func (o *Observable[T]) Update(i int, fn func(*T)) *Observable[T] {
	if i < 0 || i >= len(o.items) {
		return o
	}
	old := o.items[i]
	fn(&o.items[i])
	o.notify(Change[T]{Type: ChangeUpdate, Index: i, Item: o.items[i], Old: old})
	return o
}

// Clear removes all items.
func (o *Observable[T]) Clear() *Observable[T] {
	o.items = o.items[:0]
	o.notify(Change[T]{Type: ChangeClear})
	return o
}

// Subscribe adds a change listener and returns an unsubscribe function.
func (o *Observable[T]) Subscribe(fn func(Change[T])) func() {
	o.listeners = append(o.listeners, fn)
	idx := len(o.listeners) - 1
	return func() {
		// Zero out to allow GC, don't reorder
		o.listeners[idx] = nil
	}
}

func (o *Observable[T]) notify(c Change[T]) {
	for _, fn := range o.listeners {
		if fn != nil {
			fn(c)
		}
	}
}

// Dispatcher defines how data items map to components.
// Create builds a new component for a data item.
// Update modifies an existing component when data changes (optional).
type Dispatcher[T any, C Component] struct {
	Create func(item T, index int) C
	Update func(comp C, item T, index int) // nil = recreate on change
}

// BoundList connects an Observable to a List via a Dispatcher.
// It handles granular add/update/remove without full rebuilds.
type BoundList[T any, C Component] struct {
	BaseContainer
	data       *Observable[T]
	dispatcher Dispatcher[T, C]
	list       *List[C]
	unsub      func()
}

// Bind creates a bound list with a simple create function.
// Components are recreated on update (stateless items).
func Bind[T any, C Component](data *Observable[T], create func(item T, index int) C) *BoundList[T, C] {
	return BindWith(data, Dispatcher[T, C]{Create: create})
}

// BindWith creates a bound list with a full dispatcher.
// Allows custom update logic for efficient in-place modifications.
func BindWith[T any, C Component](data *Observable[T], dispatcher Dispatcher[T, C]) *BoundList[T, C] {
	b := &BoundList[T, C]{
		data:       data,
		dispatcher: dispatcher,
		list:       NewList[C](),
	}
	b.style = DefaultStyle()

	// Subscribe to changes
	b.unsub = data.Subscribe(func(c Change[T]) {
		b.handleChange(c)
	})

	// Initial build
	b.rebuild()
	return b
}

// handleChange processes a single change event efficiently.
func (b *BoundList[T, C]) handleChange(c Change[T]) {
	switch c.Type {
	case ChangeAdd:
		comp := b.dispatcher.Create(c.Item, c.Index)
		if c.Index >= b.list.Len() {
			b.list.Add(comp)
		} else {
			b.list.Insert(c.Index, comp)
		}

	case ChangeUpdate:
		if b.dispatcher.Update != nil {
			// In-place update
			comp := b.list.At(c.Index)
			b.dispatcher.Update(comp, c.Item, c.Index)
		} else {
			// Recreate
			b.list.RemoveAt(c.Index)
			comp := b.dispatcher.Create(c.Item, c.Index)
			b.list.Insert(c.Index, comp)
		}

	case ChangeRemove:
		b.list.RemoveAt(c.Index)

	case ChangeClear, ChangeSet:
		b.rebuild()
	}

	// Sync children reference
	b.children = b.list.children
}

// rebuild recreates the entire component list from data.
func (b *BoundList[T, C]) rebuild() {
	b.list.Clear()
	for i, item := range b.data.Items() {
		comp := b.dispatcher.Create(item, i)
		b.list.Add(comp)
	}
	b.children = b.list.children
}

// Data returns the underlying observable.
func (b *BoundList[T, C]) Data() *Observable[T] {
	return b.data
}

// List returns the underlying component list.
func (b *BoundList[T, C]) List() *List[C] {
	return b.list
}

// Horizontal switches to horizontal layout.
func (b *BoundList[T, C]) Horizontal() *BoundList[T, C] {
	// Recreate as horizontal
	b.list = NewHList[C]()
	b.rebuild()
	return b
}

// SetConstraints implements Component.
func (b *BoundList[T, C]) SetConstraints(width, height int) {
	b.Base.SetConstraints(width, height)
	b.list.SetConstraints(width, height)
	b.width, b.height = b.list.Size()
}

// MinSize implements Component.
func (b *BoundList[T, C]) MinSize() (int, int) {
	return b.list.MinSize()
}

// Size implements Component.
func (b *BoundList[T, C]) Size() (int, int) {
	return b.list.Size()
}

// Render implements Component.
func (b *BoundList[T, C]) Render(buf *Buffer, x, y int) {
	b.list.Render(buf, x, y)
}

// Dispose cleans up the subscription.
func (b *BoundList[T, C]) Dispose() {
	if b.unsub != nil {
		b.unsub()
	}
}

// --- Fluent API (delegates to inner list) ---

func (b *BoundList[T, C]) Gap(g int) *BoundList[T, C] {
	b.list.Gap(g)
	return b
}

func (b *BoundList[T, C]) Padding(p int) *BoundList[T, C] {
	b.list.Padding(p)
	return b
}

func (b *BoundList[T, C]) Border(s BorderStyle) *BoundList[T, C] {
	b.list.Border(s)
	return b
}

func (b *BoundList[T, C]) Background(c Color) *BoundList[T, C] {
	b.list.Background(c)
	return b
}

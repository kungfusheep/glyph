package tui

import "sync"

// Buffer pool - keyed by capacity to avoid reallocating cells
var bufferPool = sync.Pool{
	New: func() any { return &Buffer{} },
}

// GetBuffer gets a buffer from the pool, resizing if needed.
func GetBuffer(width, height int) *Buffer {
	b := bufferPool.Get().(*Buffer)
	needed := width * height
	if cap(b.cells) < needed {
		b.cells = make([]Cell, needed)
	} else {
		b.cells = b.cells[:needed]
	}
	b.width = width
	b.height = height
	b.Clear()
	return b
}

// PutBuffer returns a buffer to the pool.
func PutBuffer(b *Buffer) {
	if b == nil {
		return
	}
	bufferPool.Put(b)
}

// ReleaseTree recursively releases all components in a tree back to their pools.
func ReleaseTree(c Component) {
	if c == nil {
		return
	}

	// Release children first (depth-first)
	if cont, ok := c.(Container); ok {
		for _, child := range cont.Children() {
			ReleaseTree(child)
		}
	}

	// Release the component itself
	switch v := c.(type) {
	case *TextComponent:
		textPool.Put(v)
	case *StackComponent:
		stackPool.Put(v)
	case *SpacerComponent:
		spacerPool.Put(v)
	case *ProgressComponent:
		progressPool.Put(v)
	case *GridComponent:
		gridPool.Put(v)
	}
}

// Poolable is implemented by components that can be pooled.
type Poolable interface {
	Reset()
}

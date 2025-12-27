package tui

import "unsafe"

// sliceHeader is the runtime representation of a slice.
// Used for zero-allocation slice iteration.
type sliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

// isWithinRange checks if a pointer falls within a memory range.
// Used to determine if a pointer is inside a struct for offset calculation.
func isWithinRange(ptr, base unsafe.Pointer, size uintptr) bool {
	p := uintptr(ptr)
	b := uintptr(base)
	return p >= b && p < b+size
}

// DStyle represents styling options for declarative components
type DStyle struct {
	FG        *Color
	BG        *Color
	Bold      bool
	Dim       bool
	Italic    bool
	Underline bool
	Inverse   bool
}

// ToStyle converts DStyle to the rendering Style
func (ds DStyle) ToStyle() Style {
	s := Style{}
	if ds.FG != nil {
		s.FG = *ds.FG
	}
	if ds.BG != nil {
		s.BG = *ds.BG
	}
	if ds.Bold {
		s.Attr |= AttrBold
	}
	if ds.Dim {
		s.Attr |= AttrDim
	}
	if ds.Italic {
		s.Attr |= AttrItalic
	}
	if ds.Underline {
		s.Attr |= AttrUnderline
	}
	if ds.Inverse {
		s.Attr |= AttrInverse
	}
	return s
}

// ColorPtr returns a pointer to a color for use in DStyle
func ColorPtr(c Color) *Color { return &c }

// Text displays text content
type Text struct {
	Content any    // string or *string
	Bold    bool   // shorthand for Style.Bold
	Style   DStyle // full styling options
}

// Progress displays a progress bar
type Progress struct {
	Value any   // int or *int (0-100)
	Width int16 // width in characters
}

// Row arranges children horizontally
type Row struct {
	Children []any
	Gap      int8
}

// Col arranges children vertically
type Col struct {
	Children []any
	Gap      int8
}

// IfNode conditionally renders content
type IfNode struct {
	Cond any // *bool
	Then any
}

// ElseNode renders when preceding If was false
type ElseNode struct {
	Then any
}

// If creates a conditional node
func If(cond any, then any) IfNode {
	return IfNode{Cond: cond, Then: then}
}

// Else creates an else branch
func Else(then any) ElseNode {
	return ElseNode{Then: then}
}

// ForEachNode iterates over a slice
type ForEachNode struct {
	Items  any // *[]T
	Render any // func(*T) any
}

// ForEach creates an iteration over a slice
func ForEach(items any, render any) ForEachNode {
	return ForEachNode{Items: items, Render: render}
}

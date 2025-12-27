// Package tui provides a high-performance terminal UI framework.
package tui

import "unsafe"

// Attribute represents text styling attributes that can be combined.
type Attribute uint8

const (
	AttrNone          Attribute = 0
	AttrBold          Attribute = 1 << iota
	AttrDim
	AttrItalic
	AttrUnderline
	AttrBlink
	AttrInverse
	AttrStrikethrough
)

// Has returns true if the attribute set contains the given attribute.
func (a Attribute) Has(attr Attribute) bool {
	return a&attr != 0
}

// With returns a new attribute set with the given attribute added.
func (a Attribute) With(attr Attribute) Attribute {
	return a | attr
}

// Without returns a new attribute set with the given attribute removed.
func (a Attribute) Without(attr Attribute) Attribute {
	return a &^ attr
}

// ColorMode represents the color mode for a color value.
type ColorMode uint8

const (
	ColorDefault ColorMode = iota // Terminal default
	Color16                       // Basic 16 colors (0-15)
	Color256                      // 256 color palette (0-255)
	ColorRGB                      // 24-bit true color
)

// Color represents a terminal color.
type Color struct {
	Mode    ColorMode
	R, G, B uint8 // For RGB mode
	Index   uint8 // For 16/256 mode
}

// DefaultColor returns the terminal's default color.
func DefaultColor() Color {
	return Color{Mode: ColorDefault}
}

// BasicColor returns one of the 16 basic terminal colors.
func BasicColor(index uint8) Color {
	return Color{Mode: Color16, Index: index}
}

// PaletteColor returns one of the 256 palette colors.
func PaletteColor(index uint8) Color {
	return Color{Mode: Color256, Index: index}
}

// RGB returns a 24-bit true color.
func RGB(r, g, b uint8) Color {
	return Color{Mode: ColorRGB, R: r, G: g, B: b}
}

// Hex returns a 24-bit true color from a hex value (e.g., 0xFF5500).
func Hex(hex uint32) Color {
	return Color{
		Mode: ColorRGB,
		R:    uint8((hex >> 16) & 0xFF),
		G:    uint8((hex >> 8) & 0xFF),
		B:    uint8(hex & 0xFF),
	}
}

// Standard basic colors for convenience.
var (
	Black   = BasicColor(0)
	Red     = BasicColor(1)
	Green   = BasicColor(2)
	Yellow  = BasicColor(3)
	Blue    = BasicColor(4)
	Magenta = BasicColor(5)
	Cyan    = BasicColor(6)
	White   = BasicColor(7)

	// Bright variants
	BrightBlack   = BasicColor(8)
	BrightRed     = BasicColor(9)
	BrightGreen   = BasicColor(10)
	BrightYellow  = BasicColor(11)
	BrightBlue    = BasicColor(12)
	BrightMagenta = BasicColor(13)
	BrightCyan    = BasicColor(14)
	BrightWhite   = BasicColor(15)
)

// Equal returns true if two colors are equal.
func (c Color) Equal(other Color) bool {
	if c.Mode != other.Mode {
		return false
	}
	switch c.Mode {
	case ColorDefault:
		return true
	case Color16, Color256:
		return c.Index == other.Index
	case ColorRGB:
		return c.R == other.R && c.G == other.G && c.B == other.B
	}
	return false
}

// Style combines foreground, background colors and attributes.
type Style struct {
	FG   Color
	BG   Color
	Attr Attribute
}

// DefaultStyle returns a style with default colors and no attributes.
func DefaultStyle() Style {
	return Style{
		FG: DefaultColor(),
		BG: DefaultColor(),
	}
}

// Foreground returns a new style with the given foreground color.
func (s Style) Foreground(c Color) Style {
	s.FG = c
	return s
}

// Background returns a new style with the given background color.
func (s Style) Background(c Color) Style {
	s.BG = c
	return s
}

// Bold returns a new style with bold enabled.
func (s Style) Bold() Style {
	s.Attr = s.Attr.With(AttrBold)
	return s
}

// Dim returns a new style with dim enabled.
func (s Style) Dim() Style {
	s.Attr = s.Attr.With(AttrDim)
	return s
}

// Italic returns a new style with italic enabled.
func (s Style) Italic() Style {
	s.Attr = s.Attr.With(AttrItalic)
	return s
}

// Underline returns a new style with underline enabled.
func (s Style) Underline() Style {
	s.Attr = s.Attr.With(AttrUnderline)
	return s
}

// Inverse returns a new style with inverse enabled.
func (s Style) Inverse() Style {
	s.Attr = s.Attr.With(AttrInverse)
	return s
}

// Strikethrough returns a new style with strikethrough enabled.
func (s Style) Strikethrough() Style {
	s.Attr = s.Attr.With(AttrStrikethrough)
	return s
}

// Equal returns true if two styles are equal.
func (s Style) Equal(other Style) bool {
	return s.FG.Equal(other.FG) && s.BG.Equal(other.BG) && s.Attr == other.Attr
}

// Cell represents a single character cell on the terminal.
type Cell struct {
	Rune  rune
	Style Style
}

// EmptyCell returns a cell with a space and default style.
func EmptyCell() Cell {
	return Cell{Rune: ' ', Style: DefaultStyle()}
}

// NewCell creates a cell with the given rune and style.
func NewCell(r rune, style Style) Cell {
	return Cell{Rune: r, Style: style}
}

// Equal returns true if two cells are equal.
func (c Cell) Equal(other Cell) bool {
	return c.Rune == other.Rune && c.Style.Equal(other.Style)
}

// DStyle represents styling options for declarative components.
// Uses pointers to allow dynamic binding.
type DStyle struct {
	FG        *Color
	BG        *Color
	Bold      bool
	Dim       bool
	Italic    bool
	Underline bool
	Inverse   bool
}

// ToStyle converts DStyle to the rendering Style.
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

// ColorPtr returns a pointer to a color for use in DStyle.
func ColorPtr(c Color) *Color { return &c }

// Text displays text content.
type Text struct {
	Content any    // string or *string
	Bold    bool   // shorthand for Style.Bold
	Style   DStyle // full styling options
}

// Progress displays a progress bar.
type Progress struct {
	Value any   // int or *int (0-100)
	Width int16 // width in characters
}

// Row arranges children horizontally.
type Row struct {
	Children []any
	Gap      int8
}

// Col arranges children vertically.
type Col struct {
	Children []any
	Gap      int8
}

// IfNode conditionally renders content.
type IfNode struct {
	Cond any // *bool
	Then any
}

// ElseNode renders when preceding If was false.
type ElseNode struct {
	Then any
}

// If creates a conditional node.
func If(cond any, then any) IfNode {
	return IfNode{Cond: cond, Then: then}
}

// Else creates an else branch.
func Else(then any) ElseNode {
	return ElseNode{Then: then}
}

// ForEachNode iterates over a slice.
type ForEachNode struct {
	Items  any // *[]T
	Render any // func(*T) any
}

// ForEach creates an iteration over a slice.
func ForEach(items any, render any) ForEachNode {
	return ForEachNode{Items: items, Render: render}
}

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

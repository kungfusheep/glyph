package glyph

// Deprecation shims for renamed identifiers. Each shim carries a `//go:fix inline`
// directive (Go 1.26+ tool support) so that `go fix` and gopls IDE refactorings
// can auto-migrate call sites from the old name to the new name.

// Deprecated: use Filter instead.
//
//go:fix inline
func NewFilter[T any](source *[]T, extract func(*T) string) *FilterC[T] {
	return Filter(source, extract)
}

// Deprecated: use Scrollbar instead.
//
//go:fix inline
func Scroll(contentSize, viewSize int, position *int) ScrollbarC {
	return Scrollbar(contentSize, viewSize, position)
}

// Deprecated: use Custom instead.
//
//go:fix inline
func Widget(
	measure func(availW int16) (w, h int16),
	render func(buf *Buffer, x, y, w, h int16),
) Component {
	return Custom(measure, render)
}

// Deprecated: use Ansi16 instead.
//
//go:fix inline
func BasicColor(index uint8) Color { return Ansi16(index) }

// Deprecated: use Ansi256 instead.
//
//go:fix inline
func PaletteColor(index uint8) Color { return Ansi256(index) }

// Deprecated: use Blend instead.
//
//go:fix inline
func BlendColor(base, top Color, mode BlendMode) Color { return Blend(base, top, mode) }

// Deprecated: use Lerp instead.
//
//go:fix inline
func LerpColor(a, b Color, t float64) Color { return Lerp(a, b, t) }

// Deprecated: use BlendDodge instead.
const BlendColorDodge = BlendDodge

// Deprecated: use BlendBurn instead.
const BlendColorBurn = BlendBurn

// Deprecated: use SEDim instead.
//
//go:fix inline
func SEDimAll() Effect { return SEDim() }

// Deprecated: use FormatBool instead.
//
//go:fix inline
func Bool(yes, no string) ColumnOption { return FormatBool(yes, no) }

// Deprecated: use FormatBytes instead.
//
//go:fix inline
func Bytes() ColumnOption { return FormatBytes() }

// Deprecated: use FormatCurrency instead.
//
//go:fix inline
func Currency(symbol string, decimals int) ColumnOption { return FormatCurrency(symbol, decimals) }

// Deprecated: use FormatNumber instead.
//
//go:fix inline
func Number(decimals int) ColumnOption { return FormatNumber(decimals) }

// Deprecated: use FormatPercent instead.
//
//go:fix inline
func Percent(decimals int) ColumnOption { return FormatPercent(decimals) }

// Deprecated: use FormatPercentChange instead.
//
//go:fix inline
func PercentChange(decimals int) ColumnOption { return FormatPercentChange(decimals) }

// Deprecated: use Width instead.
//
//go:fix inline
func (l LayerViewC) ViewWidth(w int16) LayerViewC { return l.Width(w) }

// Deprecated: use Height instead.
//
//go:fix inline
func (l LayerViewC) ViewHeight(h int16) LayerViewC { return l.Height(h) }

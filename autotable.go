package forme

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ColumnOption configures a single AutoTable column.
type ColumnOption func(*ColumnConfig)

// ColumnConfig holds rendering configuration for one column.
type ColumnConfig struct {
	align    Align
	hasAlign bool // true if explicitly set (vs type default)
	format   func(any) string
	style    func(any) Style
}

// Align sets the column alignment.
func (c *ColumnConfig) Align(a Align) { c.align = a; c.hasAlign = true }

// Format sets a function that converts the field value to display text.
func (c *ColumnConfig) Format(fn func(any) string) { c.format = fn }

// Style sets a function that returns a per-cell style based on the field value.
func (c *ColumnConfig) Style(fn func(any) Style) { c.style = fn }

// ----------------------------------------------------------------------------
// canned format presets
// ----------------------------------------------------------------------------

// Number formats numeric values with comma separators.
// decimals controls decimal places for floats (ignored for integers).
func Number(decimals int) ColumnOption {
	return func(c *ColumnConfig) {
		c.Align(AlignRight)
		c.Format(func(v any) string {
			return formatNumber(v, decimals)
		})
	}
}

// Currency formats numeric values with a symbol prefix and comma
// separators - it is by no means a full internationalization solution,
// but it's a quick default.
func Currency(symbol string, decimals int) ColumnOption {
	return func(c *ColumnConfig) {
		c.Align(AlignRight)
		c.Format(func(v any) string {
			return symbol + formatNumber(v, decimals)
		})
	}
}

// Percent formats numeric values as percentages.
func Percent(decimals int) ColumnOption {
	return func(c *ColumnConfig) {
		c.Align(AlignRight)
		c.Format(func(v any) string {
			f := toFloat64(v)
			return strconv.FormatFloat(f, 'f', decimals, 64) + "%"
		})
	}
}

// PercentChange formats numeric values as signed percentages with green/red coloring.
func PercentChange(decimals int) ColumnOption {
	return func(c *ColumnConfig) {
		c.Align(AlignRight)
		c.Format(func(v any) string {
			f := toFloat64(v)
			sign := "+"
			if f < 0 {
				sign = ""
			}
			return sign + strconv.FormatFloat(f, 'f', decimals, 64) + "%"
		})
		c.Style(func(v any) Style {
			if toFloat64(v) >= 0 {
				return Style{FG: Green}
			}
			return Style{FG: Red}
		})
	}
}

// Bytes formats numeric values as human-readable byte sizes.
func Bytes() ColumnOption {
	return func(c *ColumnConfig) {
		c.Align(AlignRight)
		c.Format(func(v any) string {
			return formatBytes(toFloat64(v))
		})
	}
}

// Bool formats boolean values with custom labels.
func Bool(yes, no string) ColumnOption {
	return func(c *ColumnConfig) {
		c.Align(AlignCenter)
		c.Format(func(v any) string {
			if b, ok := v.(bool); ok && b {
				return yes
			}
			return no
		})
	}
}

// ----------------------------------------------------------------------------
// canned style presets
// ----------------------------------------------------------------------------

// StyleSign colors cells based on the numeric sign of the value.
func StyleSign(positive, negative Style) ColumnOption {
	return func(c *ColumnConfig) {
		c.Style(func(v any) Style {
			if toFloat64(v) >= 0 {
				return positive
			}
			return negative
		})
	}
}

// StyleBool colors cells based on a boolean value.
func StyleBool(trueStyle, falseStyle Style) ColumnOption {
	return func(c *ColumnConfig) {
		c.Style(func(v any) Style {
			if b, ok := v.(bool); ok && b {
				return trueStyle
			}
			return falseStyle
		})
	}
}

// StyleThreshold colors cells based on numeric value thresholds.
// Values < low get belowStyle, low..high get betweenStyle, > high get aboveStyle.
func StyleThreshold(low, high float64, belowStyle, betweenStyle, aboveStyle Style) ColumnOption {
	return func(c *ColumnConfig) {
		c.Style(func(v any) Style {
			f := toFloat64(v)
			if f < low {
				return belowStyle
			}
			if f > high {
				return aboveStyle
			}
			return betweenStyle
		})
	}
}

// ----------------------------------------------------------------------------
// internal helpers
// ----------------------------------------------------------------------------

// toFloat64 converts common numeric types to float64.
func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int8:
		return float64(n)
	case int16:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case uint:
		return float64(n)
	case uint8:
		return float64(n)
	case uint16:
		return float64(n)
	case uint32:
		return float64(n)
	case uint64:
		return float64(n)
	default:
		return 0
	}
}

// formatNumber formats a numeric value with comma separators.
func formatNumber(v any, decimals int) string {
	f := toFloat64(v)
	// format the number without commas first
	s := strconv.FormatFloat(f, 'f', decimals, 64)
	return insertCommas(s)
}

// insertCommas adds thousand separators to a numeric string.
func insertCommas(s string) string {
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}

	// split on decimal point
	integer, decimal, hasDecimal := strings.Cut(s, ".")

	// insert commas into integer part from right to left
	n := len(integer)
	if n <= 3 {
		// no commas needed
	} else {
		var b strings.Builder
		b.Grow(n + n/3)
		start := n % 3
		if start == 0 {
			start = 3
		}
		b.WriteString(integer[:start])
		for i := start; i < n; i += 3 {
			b.WriteByte(',')
			b.WriteString(integer[i : i+3])
		}
		integer = b.String()
	}

	var result string
	if hasDecimal {
		result = integer + "." + decimal
	} else {
		result = integer
	}

	if neg {
		return "-" + result
	}
	return result
}

// formatBytes converts a byte count to a human-readable string.
func formatBytes(b float64) string {
	if b < 0 {
		return "-" + formatBytes(-b)
	}

	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	if b < 1 {
		return "0 B"
	}

	exp := int(math.Log(b) / math.Log(1024))
	if exp >= len(units) {
		exp = len(units) - 1
	}

	val := b / math.Pow(1024, float64(exp))

	if exp == 0 {
		return fmt.Sprintf("%.0f %s", val, units[exp])
	}
	return fmt.Sprintf("%.1f %s", val, units[exp])
}

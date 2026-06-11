package glyph

import (
	"strings"
	"testing"
)

// terminal sizes used across benchmarks
const (
	benchSmallW, benchSmallH = 80, 24
	benchLargeW, benchLargeH = 200, 50
)

var benchStyle = DefaultStyle().Foreground(Green)
var benchCell = NewCell('X', benchStyle)

// --- Cell operations ---

func BenchmarkSetSmall(b *testing.B) {
	buf := NewBuffer(benchSmallW, benchSmallH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Set(i%benchSmallW, (i/benchSmallW)%benchSmallH, benchCell)
	}
}

func BenchmarkSetLarge(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Set(i%benchLargeW, (i/benchLargeW)%benchLargeH, benchCell)
	}
}

func BenchmarkSetFastSmall(b *testing.B) {
	buf := NewBuffer(benchSmallW, benchSmallH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.SetFast(i%benchSmallW, (i/benchSmallW)%benchSmallH, benchCell)
	}
}

func BenchmarkSetFastLarge(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.SetFast(i%benchLargeW, (i/benchLargeW)%benchLargeH, benchCell)
	}
}

func BenchmarkSetBorderMerge(b *testing.B) {
	buf := NewBuffer(benchSmallW, benchSmallH)
	horiz := NewCell('─', benchStyle)
	vert := NewCell('│', benchStyle)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x, y := i%benchSmallW, (i/benchSmallW)%benchSmallH
		buf.Set(x, y, horiz)
		buf.Set(x, y, vert) // triggers merge → ┼
	}
}

func BenchmarkSetRune(b *testing.B) {
	buf := NewBuffer(benchSmallW, benchSmallH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.SetRune(i%benchSmallW, (i/benchSmallW)%benchSmallH, 'X')
	}
}

func BenchmarkSetStyle(b *testing.B) {
	buf := NewBuffer(benchSmallW, benchSmallH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.SetStyle(i%benchSmallW, (i/benchSmallW)%benchSmallH, benchStyle)
	}
}

// --- String / text writes ---

func BenchmarkWriteStringFastShort(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	s := "hello"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.WriteStringFast(0, i%benchLargeH, s, benchStyle, benchLargeW)
	}
}

func BenchmarkWriteStringFastLong(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	s := "the quick brown fox jumps over the lazy dog and keeps on running across the screen"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.WriteStringFast(0, i%benchLargeH, s, benchStyle, benchLargeW)
	}
}

func BenchmarkWriteStringFastClipped(b *testing.B) {
	buf := NewBuffer(benchSmallW, benchSmallH)
	s := "this string is longer than the max width and will be clipped at the boundary"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.WriteStringFast(0, i%benchSmallH, s, benchStyle, 40)
	}
}

func BenchmarkWriteStringLegacy(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	s := "hello world"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.WriteString(0, i%benchLargeH, s, benchStyle)
	}
}

func BenchmarkWriteStringClipped(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	s := "this is a string that needs to be clipped to a maximum width"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.WriteStringClipped(0, i%benchLargeH, s, benchStyle, 30)
	}
}

func BenchmarkWriteStringPadded(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	s := "padded"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.WriteStringPadded(0, i%benchLargeH, s, benchStyle, 40)
	}
}

func BenchmarkWriteSpans(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	spans := []Span{
		{Text: "red text ", Style: DefaultStyle().Foreground(Red)},
		{Text: "green text ", Style: DefaultStyle().Foreground(Green)},
		{Text: "blue text", Style: DefaultStyle().Foreground(Blue)},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.WriteSpans(0, i%benchLargeH, spans, benchLargeW)
	}
}

// --- Drawing primitives ---

func BenchmarkWriteProgressBar(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.WriteProgressBar(0, i%benchLargeH, 80, 0.73, benchStyle)
	}
}

func BenchmarkWriteLeader(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.WriteLeader(0, i%benchLargeH, "CPU", "95%", 40, '.', benchStyle)
	}
}

func BenchmarkWriteSparkline(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	data := make([]float64, 60)
	for i := range data {
		data[i] = float64(i%8) * 12.5
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.WriteSparkline(0, i%benchLargeH, data, 60, 0, 100, benchStyle)
	}
}

func BenchmarkWriteSparklineMulti(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	data := make([]float64, 60)
	for i := range data {
		data[i] = float64(i%8) * 12.5
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.WriteSparklineMulti(0, 0, data, 60, 5, 0, 100, benchStyle)
	}
}

func BenchmarkHLine(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.HLine(0, i%benchLargeH, benchLargeW, '─', benchStyle)
	}
}

func BenchmarkVLine(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.VLine(i%benchLargeW, 0, benchLargeH, '│', benchStyle)
	}
}

func BenchmarkDrawBorderSmall(b *testing.B) {
	buf := NewBuffer(benchSmallW, benchSmallH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.DrawBorder(0, 0, 40, 12, BorderRounded, benchStyle)
	}
}

func BenchmarkDrawBorderLarge(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.DrawBorder(0, 0, benchLargeW, benchLargeH, BorderRounded, benchStyle)
	}
}

func BenchmarkDrawBorderNested(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.DrawBorder(0, 0, 100, 25, BorderRounded, benchStyle)
		buf.DrawBorder(1, 1, 48, 23, BorderSingle, benchStyle)
		buf.DrawBorder(50, 1, 49, 23, BorderSingle, benchStyle)
	}
}

// --- Bulk operations ---

func BenchmarkFillSmall(b *testing.B) {
	buf := NewBuffer(benchSmallW, benchSmallH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Fill(benchCell)
	}
}

func BenchmarkFillLarge(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Fill(benchCell)
	}
}

func BenchmarkFillRectSmall(b *testing.B) {
	buf := NewBuffer(benchSmallW, benchSmallH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.FillRect(10, 5, 40, 10, benchCell)
	}
}

func BenchmarkFillRectLarge(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.FillRect(0, 0, 100, 25, benchCell)
	}
}

func BenchmarkClearSmall(b *testing.B) {
	buf := NewBuffer(benchSmallW, benchSmallH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear()
	}
}

func BenchmarkClearLarge(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear()
	}
}

func BenchmarkClearDirtyPartial(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// write to first 10 rows then clear only dirty region
		buf.WriteStringFast(0, 5, "some content", benchStyle, 20)
		buf.ClearDirty()
	}
}

func BenchmarkClearDirtyFull(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.WriteStringFast(0, benchLargeH-1, "bottom", benchStyle, 20)
		buf.ClearDirty()
	}
}

func BenchmarkClearLine(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.ClearLine(i % benchLargeH)
	}
}

func BenchmarkBlitSmall(b *testing.B) {
	src := NewBuffer(40, 12)
	dst := NewBuffer(benchSmallW, benchSmallH)
	src.Fill(benchCell)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst.Blit(src, 0, 0, 10, 5, 40, 12)
	}
}

func BenchmarkBlitLarge(b *testing.B) {
	src := NewBuffer(100, 25)
	dst := NewBuffer(benchLargeW, benchLargeH)
	src.Fill(benchCell)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst.Blit(src, 0, 0, 0, 0, 100, 25)
	}
}

func BenchmarkCopyFrom(b *testing.B) {
	src := NewBuffer(benchLargeW, benchLargeH)
	dst := NewBuffer(benchLargeW, benchLargeH)
	src.Fill(benchCell)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst.CopyFrom(src)
	}
}

// --- Dirty tracking ---

func BenchmarkRowDirtyCheck(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	buf.Set(0, 25, benchCell)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buf.RowDirty(i % benchLargeH)
	}
}

func BenchmarkMarkAllDirty(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.MarkAllDirty()
	}
}

func BenchmarkClearDirtyFlags(b *testing.B) {
	buf := NewBuffer(benchLargeW, benchLargeH)
	buf.MarkAllDirty()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.ClearDirtyFlags()
	}
}

// --- Composite: simulates a typical frame ---

func BenchmarkTypicalFrame(b *testing.B) {
	buf := NewBuffer(benchSmallW, benchSmallH)
	spans := []Span{
		{Text: "status: ", Style: DefaultStyle().Foreground(BrightBlack)},
		{Text: "running", Style: DefaultStyle().Foreground(Green)},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear()
		buf.DrawBorder(0, 0, benchSmallW, benchSmallH, BorderRounded, DefaultStyle())
		buf.WriteStringFast(2, 0, " dashboard ", DefaultStyle().Bold(), 20)
		for y := 1; y < 5; y++ {
			buf.WriteLeader(2, y, "metric", "42", benchSmallW-4, '.', benchStyle)
		}
		buf.WriteProgressBar(2, 6, benchSmallW-4, 0.73, benchStyle)
		buf.WriteSpans(2, 8, spans, benchSmallW-4)
		buf.HLine(1, 10, benchSmallW-2, '─', DefaultStyle())
		for y := 11; y < benchSmallH-1; y++ {
			buf.WriteStringFast(2, y, "log line content here", DefaultStyle(), benchSmallW-4)
		}
	}
}

// --- Text wrapping ---

var benchWrapText = strings.Repeat("The quick brown fox jumps over the lazy dog. ", 20)

func BenchmarkWrapTextChar(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = wrapText(benchWrapText, 80, true)
	}
}

func BenchmarkWrapTextWord(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = wrapText(benchWrapText, 80, false)
	}
}

func BenchmarkWrapTextDrawChar(b *testing.B) {
	buf := NewBuffer(80, 20)
	style := DefaultStyle()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		wrapTextDraw(benchWrapText, buf, 0, 0, 80, 20, style, true)
	}
}

func BenchmarkWrapTextDrawWord(b *testing.B) {
	buf := NewBuffer(80, 20)
	style := DefaultStyle()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		wrapTextDraw(benchWrapText, buf, 0, 0, 80, 20, style, false)
	}
}

// end-to-end: wrap + write to buffer (fair comparison with DrawWord/DrawChar)
func BenchmarkWrapWriteWord(b *testing.B) {
	buf := NewBuffer(80, 20)
	style := DefaultStyle()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lines := wrapTextWord(benchWrapText, 80)
		for j, line := range lines {
			if j >= 20 {
				break
			}
			buf.WriteStringFast(0, j, line, style, 80)
		}
	}
}

func BenchmarkWrapWriteChar(b *testing.B) {
	buf := NewBuffer(80, 20)
	style := DefaultStyle()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lines := wrapText(benchWrapText, 80, true)
		for j, line := range lines {
			if j >= 20 {
				break
			}
			buf.WriteStringFast(0, j, line, style, 80)
		}
	}
}

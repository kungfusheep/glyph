package glyph

import (
	"fmt"
	"strings"
	"testing"
)

// BenchmarkBuildSimple measures compile time for a simple template.
func BenchmarkBuildSimple(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Build(VBox(
			Text("Header"),
			Text("Body"),
			Text("Footer"),
		))
	}
}

// BenchmarkBuildNested measures compile time for nested containers.
func BenchmarkBuildNested(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Build(VBox(
			Text("Header"),
			HBox(
				VBox(
					Text("Left 1"),
					Text("Left 2"),
				),
				VBox(
					Text("Right 1"),
					Text("Right 2"),
				),
			),
			Text("Footer"),
		))
	}
}

// BenchmarkBuildForEach measures compile time with ForEach.
func BenchmarkBuildForEach(b *testing.B) {
	type Item struct {
		Name string
	}
	items := make([]Item, 100)
	for i := range items {
		items[i].Name = "Item"
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Build(VBox(
			Text("Header"),
			ForEach(&items, func(item *Item) Component {
				return Text(&item.Name)
			}),
		))
	}
}

// BenchmarkV2ExecuteSimple measures execute time for a simple template.
func BenchmarkV2ExecuteSimple(b *testing.B) {
	tmpl := Build(VBox(
		Text("Header"),
		Text("Body"),
		Text("Footer"),
	))
	buf := NewBuffer(80, 24)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear()
		tmpl.Execute(buf, 80, 24)
	}
}

// BenchmarkV2ExecuteNested measures execute time for nested containers.
func BenchmarkV2ExecuteNested(b *testing.B) {
	tmpl := Build(VBox(
		Text("Header"),
		HBox(
			VBox(
				Text("Left 1"),
				Text("Left 2"),
			),
			VBox(
				Text("Right 1"),
				Text("Right 2"),
			),
		),
		Text("Footer"),
	))
	buf := NewBuffer(80, 24)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear()
		tmpl.Execute(buf, 80, 24)
	}
}

// BenchmarkV2ExecuteDynamic measures execute time with dynamic text.
func BenchmarkV2ExecuteDynamic(b *testing.B) {
	title := "Dynamic Title"
	status := "Running..."
	count := 42

	tmpl := Build(VBox(
		Text(&title),
		Text(&status),
		Progress(&count).Width(20),
	))
	buf := NewBuffer(80, 24)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear()
		tmpl.Execute(buf, 80, 24)
	}
}

// BenchmarkV2ExecuteForEach10 measures execute time with 10 items.
func BenchmarkV2ExecuteForEach10(b *testing.B) {
	benchmarkV2ForEach(b, 10)
}

// BenchmarkV2ExecuteForEach100 measures execute time with 100 items.
func BenchmarkV2ExecuteForEach100(b *testing.B) {
	benchmarkV2ForEach(b, 100)
}

// BenchmarkV2ExecuteForEach1000 measures execute time with 1000 items.
func BenchmarkV2ExecuteForEach1000(b *testing.B) {
	benchmarkV2ForEach(b, 1000)
}

func benchmarkV2ForEach(b *testing.B, n int) {
	type Item struct {
		Name string
	}
	items := make([]Item, n)
	for i := range items {
		items[i].Name = "Item"
	}

	tmpl := Build(VBox(
		ForEach(&items, func(item *Item) Component {
			return Text(&item.Name)
		}),
	))
	buf := NewBuffer(80, n+10)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear()
		tmpl.Execute(buf, 80, int16(n)+10)
	}
}

// BenchmarkV2ExecuteIf measures execute time with conditional.
func BenchmarkV2ExecuteIf(b *testing.B) {
	show := true
	tmpl := Build(VBox(
		Text("Header"),
		If(&show).Eq(true).Then(VBox(
			Text("Detail 1"),
			Text("Detail 2"),
			Text("Detail 3"),
		)),
		Text("Footer"),
	))
	buf := NewBuffer(80, 24)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		show = i%2 == 0 // Toggle condition
		buf.Clear()
		tmpl.Execute(buf, 80, 24)
	}
}

// BenchmarkV2ExecuteComplex measures a realistic complex layout.
func BenchmarkV2ExecuteComplex(b *testing.B) {
	type Task struct {
		Name   string
		Status string
	}
	tasks := []Task{
		{Name: "Build project", Status: "Done"},
		{Name: "Run tests", Status: "Running"},
		{Name: "Deploy", Status: "Pending"},
		{Name: "Monitor", Status: "Pending"},
		{Name: "Cleanup", Status: "Pending"},
	}
	title := "Task Manager"
	showCompleted := true
	progress := 40

	tmpl := Build(VBox(
		Text(&title),
		HBox.Gap(2)(
			Text("Status:"),
			Progress(&progress).Width(20),
		),
		If(&showCompleted).Eq(true).Then(Text("Showing all tasks")),
		ForEach(&tasks, func(t *Task) Component {
			return HBox.Gap(1)(
				Text(&t.Name),
				Text(&t.Status),
			)
		}),
	))
	buf := NewBuffer(80, 24)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear()
		tmpl.Execute(buf, 80, 24)
	}
}

// BenchmarkV2WidthDistribution measures just the width phase.
func BenchmarkV2WidthDistribution(b *testing.B) {
	tmpl := Build(VBox(
		Text("Header"),
		HBox(
			VBox(
				Text("Left 1"),
				Text("Left 2"),
			),
			VBox(
				Text("Right 1"),
				Text("Right 2"),
			),
		),
		Text("Footer"),
	))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpl.distributeWidths(80, nil)
	}
}

// BenchmarkV2Layout measures just the layout phase.
func BenchmarkV2Layout(b *testing.B) {
	tmpl := Build(VBox(
		Text("Header"),
		HBox(
			VBox(
				Text("Left 1"),
				Text("Left 2"),
			),
			VBox(
				Text("Right 1"),
				Text("Right 2"),
			),
		),
		Text("Footer"),
	))
	tmpl.distributeWidths(80, nil) // Need widths first

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpl.layout(24)
	}
}

// BenchmarkV2Render measures just the render phase.
func BenchmarkV2Render(b *testing.B) {
	tmpl := Build(VBox(
		Text("Header"),
		HBox(
			VBox(
				Text("Left 1"),
				Text("Left 2"),
			),
			VBox(
				Text("Right 1"),
				Text("Right 2"),
			),
		),
		Text("Footer"),
	))
	tmpl.distributeWidths(80, nil)
	tmpl.layout(24)
	buf := NewBuffer(80, 24)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear()
		tmpl.render(buf, 0, 0, 80)
	}
}

// BenchmarkV2ExecuteForEachPerItemFlex measures the per-item flex-pointer
// rebind path (Space().Grow + Text().Width reading element fields): the rebind
// runs an itemEval per item per frame. Proves the cost is bounded and the
// steady-state render stays allocation-free (storage + closures are built once
// at compile, not per Execute).
func BenchmarkV2ExecuteForEachPerItemFlex(b *testing.B) {
	type Row struct {
		lg    float32
		w     int16
		label string
	}
	n := 100
	rows := make([]Row, n)
	for i := range rows {
		rows[i] = Row{lg: float32(i % 2), w: int16(4 + i%8), label: "x"}
	}
	tmpl := Build(VBox(
		ForEach(&rows, func(r *Row) Component {
			return HBox.Width(40)(Space().Grow(&r.lg), Text(&r.label).Width(&r.w))
		}),
	))
	buf := NewBuffer(40, n+10)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Clear()
		tmpl.Execute(buf, 40, int16(n)+10)
	}
}

// BenchmarkBigViewFrame is the whole-frame guardrail: a realistic heavy 3-pane view
// (60 styled list rows + 40 long grapheme-heavy paragraphs, 220x60). Frame cost here
// times 60fps is the engine's idle animation burn — keep it in low single-digit
// milliseconds at worst. See also BenchmarkUnboundedListFrame for the pathological case.
func BenchmarkBigViewFrame(b *testing.B) {
	type row struct{ Title, Meta, Body string }
	rows := make([]row, 60)
	for i := range rows {
		rows[i] = row{
			Title: fmt.Sprintf("item %03d — some longish title text here", i),
			Meta:  "2026-07-06 12:34 · pending · repo/name",
			Body:  strings.Repeat("the quick brown fox jumps over the lazy dog. ", 4),
		}
	}
	long := strings.Repeat("body text with wide 世界 runes and emoji 🚀 mixed in for grapheme cost. ", 3)
	para := make([]string, 40)
	for i := range para {
		para[i] = long
	}
	status := "ready"

	tmpl := Build(VBox(
		HBox.Gap(1)(
			VBox.Border(BorderRounded).Grow(1)(
				ForEach(&rows, func(r *row) Component {
					return VBox(
						Text(&r.Title).Bold(),
						HBox.Gap(1)(Text(&r.Meta).FG(RGB(150, 150, 150)), Text(&r.Body)),
					)
				}),
			),
			VBox.Border(BorderRounded).Grow(2)(
				ForEach(&para, func(s *string) Component { return Text(s) }),
			),
		),
		Text(&status),
	))
	buf := NewBuffer(220, 60)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpl.Execute(buf, 220, 60)
	}
}

// BenchmarkUnboundedListFrame is the pathological case: a ForEach bound to a large
// backlog (2000 grapheme-heavy rows) with only ~60 visible. Layout measures every row
// in the tree per frame, so this is where measure-all-rows cost shows — the target of
// text-measurement memoisation.
func BenchmarkUnboundedListFrame(b *testing.B) {
	long := strings.Repeat("wide 世界 and 🚀 emoji text in every row to stress measurement. ", 2)
	items := make([]string, 2000)
	for i := range items {
		items[i] = fmt.Sprintf("%04d %s", i, long)
	}
	tmpl := Build(VBox(
		ForEach(&items, func(s *string) Component { return Text(s) }),
	))
	buf := NewBuffer(220, 60)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpl.Execute(buf, 220, 60)
	}
}

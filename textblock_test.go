package glyph

import "testing"

func TestTextBlock_WrapsText(t *testing.T) {
	buf := NewBuffer(20, 10)
	tmpl := Build(VBox(
		TextBlock("hello world this is a long line that wraps"),
	))
	tmpl.Execute(buf, 20, 10)

	// word-wrap: "hello world this is" fits in 20, "a" would make 21
	if got := buf.GetLine(0); got != "hello world this is" {
		t.Errorf("line 0: got %q, want %q", got, "hello world this is")
	}
	// "a long line that" on line 1
	if got := buf.GetLine(1); got != "a long line that" {
		t.Errorf("line 1: got %q, want %q", got, "a long line that")
	}
}

func TestTextBlock_WrapsWideRunes(t *testing.T) {
	// 8 double-width runes = 16 cells. In width 10 they must wrap after 5 (10 cells),
	// not pack 8 runes into 8 columns and bleed past the right edge — the rune-count
	// wrap bug behind the ragged comment pane. Each wide rune claims its glyph cell
	// plus a placeholder (Rune 0) so the renderer skips the continuation.
	buf := NewBuffer(10, 6)
	tmpl := Build(VBox(TextBlock("你好世界你好世界")))
	tmpl.Execute(buf, 10, 6)

	if r := buf.Get(0, 0).Rune; r != '你' {
		t.Fatalf("(0,0) = %q, want 你", r)
	}
	if r := buf.Get(1, 0).Rune; r != 0 {
		t.Fatalf("(1,0) = %q, want placeholder 0 (continuation of a wide rune)", r)
	}
	if r := buf.Get(2, 0).Rune; r != '好' {
		t.Fatalf("(2,0) = %q, want 好", r)
	}
	// 5th rune sits at col 8 (0,2,4,6,8) — the line fills exactly 10 cells, no overflow
	if r := buf.Get(8, 0).Rune; r != '你' {
		t.Fatalf("(8,0) = %q, want 你 (5th wide rune)", r)
	}
	// the 6th rune wraps to line 1
	if r := buf.Get(0, 1).Rune; r != '好' {
		t.Fatalf("(0,1) = %q, want 好 (6th rune wrapped to line 1)", r)
	}
}

func TestTextBlock_MultipleLines(t *testing.T) {
	buf := NewBuffer(40, 10)
	tmpl := Build(VBox(
		TextBlock("line one\nline two\nline three"),
	))
	tmpl.Execute(buf, 40, 10)

	if got := buf.GetLine(0); got != "line one" {
		t.Errorf("line 0: got %q, want %q", got, "line one")
	}
	if got := buf.GetLine(1); got != "line two" {
		t.Errorf("line 1: got %q, want %q", got, "line two")
	}
	if got := buf.GetLine(2); got != "line three" {
		t.Errorf("line 2: got %q, want %q", got, "line three")
	}
}

func TestTextBlock_HeightPushesContent(t *testing.T) {
	buf := NewBuffer(40, 10)
	tmpl := Build(VBox(
		TextBlock("first\nsecond\nthird"),
		Text("after"),
	))
	tmpl.Execute(buf, 40, 10)

	// TextBlock takes 3 lines
	if got := buf.GetLine(0); got != "first" {
		t.Errorf("line 0: got %q", got)
	}
	if got := buf.GetLine(2); got != "third" {
		t.Errorf("line 2: got %q", got)
	}
	// Text("after") should be at line 3
	if got := buf.GetLine(3); got != "after" {
		t.Errorf("line 3: got %q, want %q", got, "after")
	}
}

func TestTextBlock_StyleApplied(t *testing.T) {
	buf := NewBuffer(40, 10)
	tmpl := Build(VBox(
		TextBlock("styled text").Bold(),
	))
	tmpl.Execute(buf, 40, 10)

	cell := buf.Get(0, 0)
	if cell.Rune != 's' || cell.Style.Attr&AttrBold == 0 {
		t.Errorf("(0,0): rune=%c bold=%v, want 's' bold=true", cell.Rune, cell.Style.Attr&AttrBold != 0)
	}
}

func TestTextBlock_PointerContent(t *testing.T) {
	content := "dynamic"
	buf := NewBuffer(40, 10)
	tmpl := Build(VBox(
		TextBlock(&content),
	))
	tmpl.Execute(buf, 40, 10)

	if got := buf.GetLine(0); got != "dynamic" {
		t.Errorf("line 0: got %q, want %q", got, "dynamic")
	}

	// update pointer and re-render
	content = "changed\nsecond line"
	buf.Clear()
	tmpl.Execute(buf, 40, 10)

	if got := buf.GetLine(0); got != "changed" {
		t.Errorf("after update line 0: got %q, want %q", got, "changed")
	}
	if got := buf.GetLine(1); got != "second line" {
		t.Errorf("after update line 1: got %q, want %q", got, "second line")
	}
}

func TestTextBlock_InsideScrollView(t *testing.T) {
	sv := ScrollView.Grow(1)(
		Text("header").Bold(),
		SpaceH(1),
		TextBlock("body line one\nbody line two\nbody line three"),
	)

	buf := NewBuffer(40, 10)
	tmpl := Build(VBox(sv))
	tmpl.Execute(buf, 40, 10)

	if got := buf.GetLine(0); got != "header" {
		t.Errorf("line 0: got %q, want %q", got, "header")
	}
	// SpaceH(1) at line 1
	if got := buf.GetLine(2); got != "body line one" {
		t.Errorf("line 2: got %q, want %q", got, "body line one")
	}
	if got := buf.GetLine(4); got != "body line three" {
		t.Errorf("line 4: got %q, want %q", got, "body line three")
	}
}

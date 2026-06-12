package glyph

import (
	"strings"
	"testing"
)

// a PRESENT dynamic width binding evaluating to 0 means "explicitly sized,
// currently zero" — it must not fall into implicit row flex and grab a share.
func TestDynWidthZeroIsNotFlexEligible(t *testing.T) {
	spacer := int16(0)
	label := "content"

	tmpl := Build(HBox(
		VBox.Width(&spacer)(Text("X")),
		VBox(Text(&label)),
	))

	buf := NewBuffer(40, 3)
	tmpl.Execute(buf, 40, 3)

	if got := buf.GetLine(0); !strings.HasPrefix(got, "content") {
		t.Fatalf("line 0 = %q, want content at column 0 (zero-width dyn spacer took flex share)", got)
	}

	spacer = 4
	buf2 := NewBuffer(40, 3)
	tmpl.Execute(buf2, 40, 3)
	if got := buf2.GetLine(0); !strings.HasPrefix(got, "X   content") {
		t.Fatalf("line 0 = %q, want X then content at column 4 after widening", got)
	}
}

// an explicit Text width — static or a dynamic binding — clips the content,
// matching container behaviour (declared size means what it says).
func TestTextExplicitWidthClipsContent(t *testing.T) {
	w := int16(7)
	tmpl := Build(HBox(
		Text("████████████").Width(&w),
		Text("|"),
	))
	buf := NewBuffer(20, 1)
	tmpl.Execute(buf, 20, 1)
	if got := buf.GetLine(0); got != "███████|" {
		t.Fatalf("line = %q, want 7 blocks then pipe", got)
	}

	tmpl2 := Build(HBox(Text("abcdefgh").Width(4), Text("|")))
	buf2 := NewBuffer(20, 1)
	tmpl2.Execute(buf2, 20, 1)
	if got := buf2.GetLine(0); got != "abcd|" {
		t.Fatalf("static width line = %q, want abcd|", got)
	}
}

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

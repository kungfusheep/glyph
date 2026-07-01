package glyph

import "testing"

// HalfPageDown/Up move the selection by half a full page (true-vim ^d/^u, distinct from ^f/^b),
// clamped at the ends. With no MaxVisible the full page defaults to 10, so a half page is 5.
func TestListHalfPageMovement(t *testing.T) {
	items := make([]int, 30)
	var list *ListC[int]
	tmpl := Build(VBox(
		List(&items).Render(func(n *int) Component { return Text(&emptyStr) }).Ref(func(l *ListC[int]) { list = l }),
	))
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10) // initialises len

	list.First(nil)
	list.PageDown(nil)
	full := list.Index()
	if full <= 0 {
		t.Fatalf("PageDown should advance a full page, got %d", full)
	}

	list.First(nil)
	list.HalfPageDown(nil)
	half := list.Index()
	if half != full/2 || half <= 0 {
		t.Fatalf("HalfPageDown (%d) should be half of PageDown (%d)", half, full)
	}

	list.HalfPageUp(nil)
	if list.Index() != 0 {
		t.Fatalf("HalfPageUp from a half page should return to 0, got %d", list.Index())
	}

	// clamps at the ends
	list.Last(nil)
	list.HalfPageDown(nil)
	if list.Index() != len(items)-1 {
		t.Fatalf("HalfPageDown at the end should clamp to %d, got %d", len(items)-1, list.Index())
	}
	list.First(nil)
	list.HalfPageUp(nil)
	if list.Index() != 0 {
		t.Fatalf("HalfPageUp at the top should clamp to 0, got %d", list.Index())
	}
}

var emptyStr = ""

// BindHalfPageNav collects exactly the two half-page bindings, in order.
func TestListBindHalfPageNavCollected(t *testing.T) {
	items := []string{"a", "b", "c"}
	tmpl := Build(VBox(
		List(&items).Render(func(s *string) Component { return Text(s) }).BindHalfPageNav("<C-d>", "<C-u>"),
	))
	if len(tmpl.pendingBindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(tmpl.pendingBindings))
	}
	if tmpl.pendingBindings[0].pattern != "<C-d>" || tmpl.pendingBindings[1].pattern != "<C-u>" {
		t.Errorf("expected <C-d>,<C-u>, got %q,%q", tmpl.pendingBindings[0].pattern, tmpl.pendingBindings[1].pattern)
	}
}

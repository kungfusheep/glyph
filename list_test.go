package glyph

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
)

type TestItem struct {
	Name string
	Done bool
}

func TestListCNavigation(t *testing.T) {
	items := []TestItem{
		{Name: "First", Done: false},
		{Name: "Second", Done: true},
		{Name: "Third", Done: false},
	}

	var list *ListC[TestItem]

	listComp := List(&items).Render(func(item *TestItem) Component {
		return Text(&item.Name)
	}).Ref(func(l *ListC[TestItem]) { list = l })

	// Build and execute template to initialize len
	tmpl := Build(VBox(listComp))
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	// Verify initial selection is 0
	if list.Index() != 0 {
		t.Errorf("Expected initial index 0, got %d", list.Index())
	}

	// Verify Selected returns correct item
	if list.Selected() == nil {
		t.Fatal("Selected() returned nil")
	}
	if list.Selected().Name != "First" {
		t.Errorf("Expected 'First', got '%s'", list.Selected().Name)
	}

	// Test navigation
	list.Down(nil)
	if list.Index() != 1 {
		t.Errorf("After Down, expected index 1, got %d", list.Index())
	}
	if list.Selected().Name != "Second" {
		t.Errorf("After Down, expected 'Second', got '%s'", list.Selected().Name)
	}

	list.Down(nil)
	if list.Index() != 2 {
		t.Errorf("After second Down, expected index 2, got %d", list.Index())
	}

	// Can't go past end
	list.Down(nil)
	if list.Index() != 2 {
		t.Errorf("Should stay at 2 (end), got %d", list.Index())
	}

	list.Up(nil)
	if list.Index() != 1 {
		t.Errorf("After Up, expected index 1, got %d", list.Index())
	}

	// Test First/Last
	list.Last(nil)
	if list.Index() != 2 {
		t.Errorf("After Last, expected index 2, got %d", list.Index())
	}

	list.First(nil)
	if list.Index() != 0 {
		t.Errorf("After First, expected index 0, got %d", list.Index())
	}
}

func TestListCRendersText(t *testing.T) {
	items := []TestItem{
		{Name: "Apple", Done: false},
		{Name: "Banana", Done: true},
	}

	listComp := List(&items).Render(func(item *TestItem) Component {
		return Text(&item.Name)
	})

	tmpl := Build(VBox(listComp))
	buf := NewBuffer(40, 5)
	tmpl.Execute(buf, 40, 5)

	// Check that text renders correctly
	line0 := buf.GetLine(0)
	line1 := buf.GetLine(1)

	// Should see marker and text
	if line0 == "" {
		t.Error("Line 0 is empty")
	}
	if !strings.Contains(line0, "Apple") {
		t.Errorf("Line 0 should contain 'Apple', got: %q", line0)
	}
	if !strings.Contains(line1, "Banana") {
		t.Errorf("Line 1 should contain 'Banana', got: %q", line1)
	}
}

func TestListCOnSelect(t *testing.T) {
	items := []TestItem{
		{Name: "First"},
		{Name: "Second"},
		{Name: "Third"},
	}

	var list *ListC[TestItem]
	var selected string
	callCount := 0

	listComp := List(&items).Render(func(item *TestItem) Component {
		return Text(&item.Name)
	}).OnSelect(func(item *TestItem) {
		selected = item.Name
		callCount++
	}).Ref(func(l *ListC[TestItem]) { list = l })

	tmpl := Build(VBox(listComp))
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	// move down, should fire OnSelect
	list.Down(nil)
	if selected != "Second" {
		t.Errorf("OnSelect should receive 'Second', got %q", selected)
	}
	if callCount != 1 {
		t.Errorf("OnSelect should fire once, fired %d times", callCount)
	}

	// move down again
	list.Down(nil)
	if selected != "Third" {
		t.Errorf("OnSelect should receive 'Third', got %q", selected)
	}

	// move down at end, should NOT fire (no change)
	callCount = 0
	list.Down(nil)
	if callCount != 0 {
		t.Errorf("OnSelect should not fire when selection doesn't change, fired %d", callCount)
	}

	// move up
	callCount = 0
	list.Up(nil)
	if selected != "Second" {
		t.Errorf("OnSelect should receive 'Second', got %q", selected)
	}
	if callCount != 1 {
		t.Errorf("OnSelect should fire once on Up, fired %d", callCount)
	}

	// First/Last
	callCount = 0
	list.Last(nil)
	if selected != "Third" {
		t.Errorf("OnSelect should receive 'Third' after Last, got %q", selected)
	}
	list.First(nil)
	if selected != "First" {
		t.Errorf("OnSelect should receive 'First' after First, got %q", selected)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls (Last+First), got %d", callCount)
	}
}

func TestListCDelete(t *testing.T) {
	items := []TestItem{
		{Name: "First", Done: false},
		{Name: "Second", Done: true},
		{Name: "Third", Done: false},
	}

	var list *ListC[TestItem]

	listComp := List(&items).Render(func(item *TestItem) Component {
		return Text(&item.Name)
	}).Ref(func(l *ListC[TestItem]) { list = l })

	// Need to compile/execute to set len
	tmpl := Build(VBox(listComp))
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	// Delete first item
	list.Delete()
	if len(items) != 2 {
		t.Errorf("After delete, expected 2 items, got %d", len(items))
	}
	if items[0].Name != "Second" {
		t.Errorf("First item should now be 'Second', got '%s'", items[0].Name)
	}
	if list.Index() != 0 {
		t.Errorf("Selection should stay at 0, got %d", list.Index())
	}

	// Move to end and delete
	list.Down(nil)
	if list.Index() != 1 {
		t.Errorf("Expected index 1, got %d", list.Index())
	}
	list.Delete()
	if len(items) != 1 {
		t.Errorf("After second delete, expected 1 item, got %d", len(items))
	}
	// Selection should adjust to stay in bounds
	if list.Index() != 0 {
		t.Errorf("After deleting last item, selection should be 0, got %d", list.Index())
	}
}

// TestListScrollConvergesInOneFrame guards against the one-frame-stale
// blank-bottom: a height-windowed variable-height List must produce its final
// window in a SINGLE render pass after a scroll changes the offset. Previously
// the render-phase scroll-down adjustment set endIdx=selected+1 and left the
// viewport's bottom rows unfilled until the next frame re-rendered from the
// persisted offset (a visible blank flash on every j past the bottom edge).
func TestListScrollConvergesInOneFrame(t *testing.T) {
	const W, H = 24, 14
	type Item struct{ Lines []string }
	items := make([]Item, 18)
	for i := range items {
		n := (i % 4) + 1 // variable heights 1..4 so the window has a partial edge
		ls := make([]string, n)
		for j := range ls {
			ls[j] = strings.Repeat(string(rune('A'+i%26)), 4)
		}
		items[i] = Item{Lines: ls}
	}
	sel := 0
	list := List(&items).MaxVisible(8).Selection(&sel).Render(func(it *Item) Component {
		return VBox(ForEach(&it.Lines, func(s *string) Component { return Text(s) }))
	})
	tmpl := Build(VBox.Height(H)(list.Build()))

	nonblank := func(b *Buffer) int {
		c := 0
		for y := 0; y < H; y++ {
			if strings.TrimRight(extractLine(b, y, W), " \x00") != "" {
				c++
			}
		}
		return c
	}

	// at every step a SINGLE render must already equal a second render of the
	// same state (converged) — in both scroll directions.
	assertConverged := func(label string, s int) {
		first := NewBuffer(W, H)
		tmpl.Execute(first, W, H)
		second := NewBuffer(W, H)
		tmpl.Execute(second, W, H)
		for y := 0; y < H; y++ {
			a := strings.TrimRight(extractLine(first, y, W), " \x00")
			b := strings.TrimRight(extractLine(second, y, W), " \x00")
			if a != b {
				t.Errorf("%s step %d row %d: first render %q != converged render %q (one-frame-stale)", label, s, y, a, b)
			}
		}
		if nb1, nb2 := nonblank(first), nonblank(second); nb1 != nb2 {
			t.Errorf("%s step %d: first render filled %d rows, converged fills %d", label, s, nb1, nb2)
		}
	}

	for s := 0; s < 16; s++ {
		list.cached.Down(nil)
		assertConverged("down", s)
	}
	for s := 0; s < 16; s++ {
		list.cached.Up(nil)
		assertConverged("up", s)
	}
}

func BenchmarkSelectionListScrollRender(b *testing.B) {
	const W, H = 24, 14
	type Item struct{ Lines []string }
	items := make([]Item, 60)
	for i := range items {
		n := (i % 4) + 1
		ls := make([]string, n)
		for j := range ls {
			ls[j] = strings.Repeat(string(rune('A'+i%26)), 4)
		}
		items[i] = Item{Lines: ls}
	}
	sel := 0
	list := List(&items).MaxVisible(8).Selection(&sel).Render(func(it *Item) Component {
		return VBox(ForEach(&it.Lines, func(s *string) Component { return Text(s) }))
	})
	tmpl := Build(VBox.Height(H)(list.Build()))
	buf := NewBuffer(W, H)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpl.Execute(buf, W, H)
		list.cached.Down(nil)
		if sel >= len(items)-1 {
			sel = 0
			list.cached.offset = 0
		}
	}
}

// CheckList.Style/SelectedStyle must accept a live *Style (parity with List): a theme
// var reassigned after build must recolour on the next render, since views are built
// once at registration. The non-selected row carries Style; assert it tracks the pointer.
func TestCheckListStyleAcceptsLivePointer(t *testing.T) {
	items := []string{"alpha", "beta"}
	st := Style{FG: RGB(200, 0, 0)}
	sel := 0 // row 0 selected, so row 1 ("beta") is the non-selected row carrying Style
	cl := CheckList(&items).
		Selection(&sel).
		Checked(func(s *string) *bool { b := false; return &b }).
		Render(func(s *string) Component { return Text(s) }).
		Style(&st)
	tmpl := Build(VBox(cl))
	buf := NewBuffer(20, 4)
	tmpl.Execute(buf, 20, 4)

	if got := buf.Get(4, 1).Style.FG; got != (RGB(200, 0, 0)) {
		t.Fatalf("non-selected row FG = %v, want red from the live *Style", got)
	}

	// theme switch: reassign the style var; the live pointer must recolour next frame
	st = Style{FG: RGB(0, 200, 0)}
	buf.Clear()
	tmpl.Execute(buf, 20, 4)
	if got := buf.Get(4, 1).Style.FG; got != (RGB(0, 200, 0)) {
		t.Fatalf("after theme reassignment FG = %v, want green — Style(&st) not read live (frozen-at-build parity gap)", got)
	}
}

// easeRow is a fixed-height row, so a row offset and an item index differ by a known
// factor and an assertion can name either without ambiguity.
type easeRow struct{ Name string }

// easeListFixture builds a clipped List over n single-row items, opted into ScrollEase
// on a test clock. It returns the template, the bound offset, the selection and a paint
// function so a test can drive frames without re-deriving the setup.
func easeListFixture(t *testing.T, n, viewH int, dur time.Duration) (paint func() []string, sel *int, off *int, sl *selectionList, clock *time.Time) {
	t.Helper()
	rows := make([]easeRow, n)
	for i := range rows {
		rows[i].Name = "row" + strconv.Itoa(i)
	}
	selected := 0
	offset := 0
	lc := List(&rows).Selection(&selected).ScrollEase(Animate.Duration(dur)(&offset)).
		Marker(" ").Render(func(r *easeRow) Component { return Text(&r.Name) })
	tpl := Build(VBox.Grow(1)(lc))
	now := time.Unix(0, 0)
	// arm on the first frame, then take the clock over so time only moves when a test says
	buf := NewBuffer(12, viewH)
	tpl.Execute(buf, 12, int16(viewH))
	list := lc.cached
	list.ease.nowFn = func() time.Time { return now }
	paint = func() []string {
		b := NewBuffer(12, viewH)
		tpl.Execute(b, 12, int16(viewH))
		out := make([]string, viewH)
		for y := 0; y < viewH; y++ {
			out[y] = strings.TrimRight(b.GetLine(y), " ")
		}
		return out
	}
	return paint, &selected, &offset, list, &now
}

// A list that opts in glides: mid-ease the top row is BEHIND the target window, and it
// arrives once the duration elapses. Without the presentation stage the first frame
// after the move would already show the destination.
func TestListScrollEaseGlidesToTheWindow(t *testing.T) {
	paint, sel, _, list, clock := easeListFixture(t, 40, 6, 100*time.Millisecond)

	if got := paint()[0]; got != " row0" {
		t.Fatalf("resting top row = %q, want row0", got)
	}

	*sel = 10           // window top 5, well inside the 12-row snap threshold at this height
	first := paint()[0] // starts the ease; still near the old position
	*clock = clock.Add(50 * time.Millisecond)
	mid := paint()[0]
	*clock = clock.Add(60 * time.Millisecond)
	settled := paint()

	if first == mid {
		t.Errorf("the offset did not move between frames: %q then %q", first, mid)
	}
	if !list.ease.shownSet {
		t.Fatal("the ease never armed")
	}
	// at rest the same rows are visible as a snapping list would have shown: the selected
	// row is the last fully-fitting one
	if last := settled[len(settled)-1]; last != " row10" {
		t.Errorf("settled bottom row = %q, want row10 — the eased list must rest where the snap would", last)
	}
	if list.ease.animating {
		t.Error("the ease is still animating after its duration elapsed")
	}
}

// A jump past the threshold snaps: easing hundreds of rows is a blur, and the widened
// build it would cost buys nothing. The ease stays armed for the next move.
func TestListScrollEaseSnapsPastTheThreshold(t *testing.T) {
	paint, sel, off, list, _ := easeListFixture(t, 400, 6, 100*time.Millisecond)
	paint()

	*sel = 300 // far beyond scrollEaseSnapScreens * 6 rows
	got := paint()

	if list.ease.animating {
		t.Error("a jump past the threshold must snap, not animate")
	}
	if last := got[len(got)-1]; last != " row300" {
		t.Errorf("bottom row = %q, want row300 — the snap must land on the target window", last)
	}
	if list.ease.target == nil {
		t.Error("the snap must leave the ease armed for the next move")
	}
	if *off != int(list.ease.shown) {
		t.Errorf("target %d and displayed %v disagree after a snap", *off, list.ease.shown)
	}
}

// The scrollbar writeback follows the EASED position, not the target: a bar resting at
// the destination while rows still glide would disagree with the visible window on
// every in-flight frame.
func TestListScrollEaseScrollbarFollowsTheEasedPosition(t *testing.T) {
	rows := make([]easeRow, 40)
	for i := range rows {
		rows[i].Name = "row" + strconv.Itoa(i)
	}
	selected, offset := 0, 0
	var barOff, barVis, barTotal int
	lc := List(&rows).Selection(&selected).
		ScrollEase(Animate.Duration(100*time.Millisecond).Ease(EaseLinear)(&offset)).
		ScrollState(&barOff, &barVis, &barTotal).
		Marker(" ").Render(func(r *easeRow) Component { return Text(&r.Name) })
	tpl := Build(VBox.Grow(1)(lc))
	buf := NewBuffer(12, 6)
	tpl.Execute(buf, 12, 6)
	now := time.Unix(0, 0)
	lc.cached.ease.nowFn = func() time.Time { return now }

	selected = 16 // window top 11, inside the 12-row snap threshold at this height
	tpl.Execute(buf, 12, 6)
	now = now.Add(50 * time.Millisecond)
	tpl.Execute(buf, 12, 6)

	mid := barOff
	if mid >= offset {
		t.Errorf("mid-ease the bar (%d) must lag the target (%d)", mid, offset)
	}
	// row-stepped by construction: the bound offset is an integer, so the bar tracks the
	// eased position ROUNDED rather than a sub-row value
	if want := int(math.Round(lc.cached.ease.shown)); mid != want {
		t.Errorf("bar offset %d does not track the eased position %v (rounded %d)", mid, lc.cached.ease.shown, want)
	}

	now = now.Add(60 * time.Millisecond)
	tpl.Execute(buf, 12, 6)
	if barOff != offset {
		t.Errorf("at rest the bar (%d) and the target (%d) must agree", barOff, offset)
	}
}

// Mid-tween the frame builds ONE window's worth of items, never the union of the whole
// traversal. A long jump that widened the build would defeat the culling contract.
//
// This one is a TRIPWIRE, not a proof: with the eased stage removed it still passes,
// because a snapping list never widens the build in the first place. It exists to fail
// on a future implementation that culls across the traversal instead of from the current
// eased position — read it as guarding that, not as evidence the stage works.
func TestListScrollEaseCullStaysBoundedMidTween(t *testing.T) {
	rows := make([]easeRow, 400)
	for i := range rows {
		rows[i].Name = "row" + strconv.Itoa(i)
	}
	selected, offset := 0, 0
	built := 0
	lc := List(&rows).Selection(&selected).
		ScrollEase(Animate.Duration(100 * time.Millisecond)(&offset)).
		Marker(" ").Render(func(r *easeRow) Component { built++; return Text(&r.Name) })
	tpl := Build(VBox.Grow(1)(lc))
	buf := NewBuffer(12, 6)
	tpl.Execute(buf, 12, 6)
	now := time.Unix(0, 0)
	lc.cached.ease.nowFn = func() time.Time { return now }

	selected = 8 // an eased move, inside the snap threshold
	tpl.Execute(buf, 12, 6)
	now = now.Add(50 * time.Millisecond)
	built = 0
	tpl.Execute(buf, 12, 6)

	if built > 12 {
		t.Errorf("an in-flight frame built %d rows; a bounded cull is one window's worth (~6, allow the edge row and marker pass)", built)
	}
}

// Zero allocations on TWEEN frames specifically. A resting frame staying allocation-free
// proves nothing about the frames where this feature does its work.
func TestListScrollEaseTweenFrameAllocatesNothing(t *testing.T) {
	rows := make([]easeRow, 60)
	for i := range rows {
		rows[i].Name = "row" + strconv.Itoa(i)
	}
	selected, offset := 0, 0
	lc := List(&rows).Selection(&selected).
		ScrollEase(Animate.Duration(10 * time.Second)(&offset)).
		Marker(" ").Render(func(r *easeRow) Component { return Text(&r.Name) })
	tpl := Build(VBox.Grow(1)(lc))
	buf := NewBuffer(12, 6)
	tpl.Execute(buf, 12, 6)
	now := time.Unix(0, 0)
	lc.cached.ease.nowFn = func() time.Time { return now }

	selected = 8
	tpl.Execute(buf, 12, 6)
	now = now.Add(time.Second) // well inside a 10s ease, so every run below is mid-tween

	if !lc.cached.ease.animating {
		t.Fatal("the fixture must be mid-tween for this to measure anything")
	}
	got := testing.AllocsPerRun(30, func() { tpl.Execute(buf, 12, 6) })
	if !lc.cached.ease.animating {
		t.Fatal("the ease settled during the measurement; it no longer measures tween frames")
	}
	if got != 0 {
		t.Errorf("a tween frame allocates %v times, want 0", got)
	}
}

// One list holds one ease, so an offset bound to a ForEach element field cannot work:
// the state is shared while the pointer differs. Measured before the refusal existed —
// the target resolved to the compile prototype, writes landed there, and BOTH lists
// painted from a frozen zero offset, ignoring their own selections. This is the
// binding day-one guard for ScrollEase: it refuses at build instead.
func TestListScrollEaseRefusesAPerItemOffset(t *testing.T) {
	type outer struct {
		Rows []easeRow
		Sel  int
		Off  int
	}
	outers := []outer{{Rows: make([]easeRow, 4)}, {Rows: make([]easeRow, 4)}}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("a per-item ScrollEase offset must be refused at build, not silently frozen")
		}
		if msg, _ := r.(string); !strings.Contains(msg, "ScrollEase") {
			t.Errorf("panic %q does not name the surface that refused", r)
		}
	}()

	Build(VBox.Grow(1)(ForEach(&outers, func(o *outer) Component {
		return VBox.Height(4)(List(&o.Rows).Selection(&o.Sel).
			ScrollEase(Animate(&o.Off)).
			Marker(" ").Render(func(r *easeRow) Component { return Text(&r.Name) }))
	})))
}

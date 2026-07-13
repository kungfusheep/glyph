package glyph

import (
	"strings"
	"testing"

	"github.com/kungfusheep/riffkey"
)

func TestListBindNavCollected(t *testing.T) {
	items := []string{"a", "b", "c"}
	tmpl := Build(VBox(
		List(&items).Render(func(s *string) Component { return Text(s) }).BindNav("j", "k"),
	))
	if len(tmpl.pendingBindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(tmpl.pendingBindings))
	}
	if tmpl.pendingBindings[0].pattern != "j" {
		t.Errorf("expected pattern 'j', got %q", tmpl.pendingBindings[0].pattern)
	}
	if tmpl.pendingBindings[1].pattern != "k" {
		t.Errorf("expected pattern 'k', got %q", tmpl.pendingBindings[1].pattern)
	}
}

func TestListBindDeleteCollected(t *testing.T) {
	items := []string{"a", "b"}
	tmpl := Build(VBox(
		List(&items).Render(func(s *string) Component { return Text(s) }).BindNav("j", "k").BindDelete("d"),
	))
	if len(tmpl.pendingBindings) != 3 {
		t.Fatalf("expected 3 bindings, got %d", len(tmpl.pendingBindings))
	}
	if tmpl.pendingBindings[2].pattern != "d" {
		t.Errorf("expected pattern 'd', got %q", tmpl.pendingBindings[2].pattern)
	}
}

func TestCheckboxBindToggleCollected(t *testing.T) {
	checked := false
	tmpl := Build(VBox(
		Checkbox(&checked, "agree").BindToggle("x"),
	))
	if len(tmpl.pendingBindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(tmpl.pendingBindings))
	}
	if tmpl.pendingBindings[0].pattern != "x" {
		t.Errorf("expected pattern 'x', got %q", tmpl.pendingBindings[0].pattern)
	}
}

func TestRadioBindNavCollected(t *testing.T) {
	sel := 0
	tmpl := Build(VBox(
		Radio(&sel, "a", "b", "c").BindNav("n", "p"),
	))
	if len(tmpl.pendingBindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(tmpl.pendingBindings))
	}
	if tmpl.pendingBindings[0].pattern != "n" {
		t.Errorf("expected pattern 'n', got %q", tmpl.pendingBindings[0].pattern)
	}
	if tmpl.pendingBindings[1].pattern != "p" {
		t.Errorf("expected pattern 'p', got %q", tmpl.pendingBindings[1].pattern)
	}
}

func TestCheckListBindingsCollected(t *testing.T) {
	type Item struct {
		Name string
		Done bool
	}
	items := []Item{{Name: "a"}, {Name: "b"}}
	tmpl := Build(VBox(
		CheckList(&items).
			Checked(func(i *Item) *bool { return &i.Done }).
			Render(func(i *Item) Component { return Text(&i.Name) }).
			BindNav("j", "k").
			BindToggle("x").
			BindDelete("d"),
	))
	// BindNav(2) + BindToggle(1) + BindDelete(1) = 4
	if len(tmpl.pendingBindings) != 4 {
		t.Fatalf("expected 4 bindings, got %d", len(tmpl.pendingBindings))
	}
	expected := []string{"j", "k", "x", "d"}
	for i, exp := range expected {
		if tmpl.pendingBindings[i].pattern != exp {
			t.Errorf("binding %d: expected %q, got %q", i, exp, tmpl.pendingBindings[i].pattern)
		}
	}
}

func TestInputBindCollected(t *testing.T) {
	tmpl := Build(VBox(
		Input().Placeholder("name").Bind(),
	))
	if tmpl.pendingTIB == nil {
		t.Fatal("expected textInputBinding to be set")
	}
	if tmpl.pendingTIB.value == nil {
		t.Error("expected value pointer to be set")
	}
	if tmpl.pendingTIB.cursor == nil {
		t.Error("expected cursor pointer to be set")
	}
}

func TestMultipleComponentBindingsCollected(t *testing.T) {
	checked := false
	sel := 0
	items := []string{"a", "b"}
	tmpl := Build(VBox(
		Checkbox(&checked, "agree").BindToggle("a"),
		Radio(&sel, "x", "y").BindNav("n", "p"),
		List(&items).Render(func(s *string) Component { return Text(s) }).BindNav("j", "k"),
	))
	// 1 (checkbox) + 2 (radio) + 2 (list) = 5
	if len(tmpl.pendingBindings) != 5 {
		t.Fatalf("expected 5 bindings, got %d", len(tmpl.pendingBindings))
	}
}

func TestListBindPageNavCollected(t *testing.T) {
	items := []string{"a", "b", "c"}
	tmpl := Build(VBox(
		List(&items).Render(func(s *string) Component { return Text(s) }).BindPageNav("<C-d>", "<C-u>"),
	))
	if len(tmpl.pendingBindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(tmpl.pendingBindings))
	}
	if tmpl.pendingBindings[0].pattern != "<C-d>" {
		t.Errorf("expected pattern '<C-d>', got %q", tmpl.pendingBindings[0].pattern)
	}
	if tmpl.pendingBindings[1].pattern != "<C-u>" {
		t.Errorf("expected pattern '<C-u>', got %q", tmpl.pendingBindings[1].pattern)
	}
}

func TestListBindFirstLastCollected(t *testing.T) {
	items := []string{"a", "b", "c"}
	tmpl := Build(VBox(
		List(&items).Render(func(s *string) Component { return Text(s) }).BindFirstLast("g", "G"),
	))
	if len(tmpl.pendingBindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(tmpl.pendingBindings))
	}
	if tmpl.pendingBindings[0].pattern != "g" {
		t.Errorf("expected pattern 'g', got %q", tmpl.pendingBindings[0].pattern)
	}
	if tmpl.pendingBindings[1].pattern != "G" {
		t.Errorf("expected pattern 'G', got %q", tmpl.pendingBindings[1].pattern)
	}
}

func TestListBindVimNavCollected(t *testing.T) {
	items := []string{"a", "b", "c"}
	tmpl := Build(VBox(
		List(&items).Render(func(s *string) Component { return Text(s) }).BindVimNav(),
	))
	// j, k, <C-d>, <C-u>, g, G = 6
	if len(tmpl.pendingBindings) != 6 {
		t.Fatalf("expected 6 bindings, got %d", len(tmpl.pendingBindings))
	}
	expected := []string{"j", "k", "<C-d>", "<C-u>", "g", "G"}
	for i, exp := range expected {
		if tmpl.pendingBindings[i].pattern != exp {
			t.Errorf("binding %d: expected %q, got %q", i, exp, tmpl.pendingBindings[i].pattern)
		}
	}
}

func TestCheckListBindPageNavCollected(t *testing.T) {
	type Item struct {
		Name string
		Done bool
	}
	items := []Item{{Name: "a"}, {Name: "b"}}
	tmpl := Build(VBox(
		CheckList(&items).
			Checked(func(i *Item) *bool { return &i.Done }).
			Render(func(i *Item) Component { return Text(&i.Name) }).
			BindPageNav("<C-d>", "<C-u>").
			BindFirstLast("g", "G"),
	))
	// 2 (page) + 2 (first/last) = 4
	if len(tmpl.pendingBindings) != 4 {
		t.Fatalf("expected 4 bindings, got %d", len(tmpl.pendingBindings))
	}
	expected := []string{"<C-d>", "<C-u>", "g", "G"}
	for i, exp := range expected {
		if tmpl.pendingBindings[i].pattern != exp {
			t.Errorf("binding %d: expected %q, got %q", i, exp, tmpl.pendingBindings[i].pattern)
		}
	}
}

func TestCheckListBindVimNavCollected(t *testing.T) {
	type Item struct {
		Name string
		Done bool
	}
	items := []Item{{Name: "a"}, {Name: "b"}}
	tmpl := Build(VBox(
		CheckList(&items).
			Checked(func(i *Item) *bool { return &i.Done }).
			Render(func(i *Item) Component { return Text(&i.Name) }).
			BindVimNav(),
	))
	// j, k, <C-d>, <C-u>, g, G = 6
	if len(tmpl.pendingBindings) != 6 {
		t.Fatalf("expected 6 bindings, got %d", len(tmpl.pendingBindings))
	}
	expected := []string{"j", "k", "<C-d>", "<C-u>", "g", "G"}
	for i, exp := range expected {
		if tmpl.pendingBindings[i].pattern != exp {
			t.Errorf("binding %d: expected %q, got %q", i, exp, tmpl.pendingBindings[i].pattern)
		}
	}
}

func TestListHandleCollected(t *testing.T) {
	items := []string{"a", "b", "c"}
	tmpl := Build(VBox(
		List(&items).Render(func(s *string) Component { return Text(s) }).
			Handle("<Enter>", func(s *string) {}).
			Handle("w", func(s *string) {}),
	))
	if len(tmpl.pendingBindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(tmpl.pendingBindings))
	}
	if tmpl.pendingBindings[0].pattern != "<Enter>" {
		t.Errorf("expected pattern '<Enter>', got %q", tmpl.pendingBindings[0].pattern)
	}
	if tmpl.pendingBindings[1].pattern != "w" {
		t.Errorf("expected pattern 'w', got %q", tmpl.pendingBindings[1].pattern)
	}
}

func TestListHandleCallsWithSelected(t *testing.T) {
	items := []string{"a", "b", "c"}
	var got string
	var list *ListC[string]
	tmpl := Build(VBox(
		List(&items).Render(func(s *string) Component { return Text(s) }).
			Ref(func(l *ListC[string]) { list = l }).
			Handle("<Enter>", func(s *string) { got = *s }),
	))
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	list.Down(nil) // select "b"
	// extract and call the handler
	for _, b := range list.bindings() {
		if b.pattern == "<Enter>" {
			b.handler.(func())()
		}
	}
	if got != "b" {
		t.Errorf("expected 'b', got %q", got)
	}
}

func TestListHandleSkipsNilSelection(t *testing.T) {
	items := []string{}
	called := false
	var list *ListC[string]
	tmpl := Build(VBox(
		List(&items).Render(func(s *string) Component { return Text(s) }).
			Ref(func(l *ListC[string]) { list = l }).
			Handle("<Enter>", func(s *string) { called = true }),
	))
	buf := NewBuffer(40, 10)
	tmpl.Execute(buf, 40, 10)

	for _, b := range list.bindings() {
		if b.pattern == "<Enter>" {
			b.handler.(func())()
		}
	}
	if called {
		t.Error("expected handler not to be called with empty list")
	}
}

func TestCheckListHandleCollected(t *testing.T) {
	type Item struct {
		Name string
		Done bool
	}
	items := []Item{{Name: "a"}, {Name: "b"}}
	tmpl := Build(VBox(
		CheckList(&items).
			Checked(func(i *Item) *bool { return &i.Done }).
			Render(func(i *Item) Component { return Text(&i.Name) }).
			Handle("<Enter>", func(i *Item) {}),
	))
	if len(tmpl.pendingBindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(tmpl.pendingBindings))
	}
	if tmpl.pendingBindings[0].pattern != "<Enter>" {
		t.Errorf("expected pattern '<Enter>', got %q", tmpl.pendingBindings[0].pattern)
	}
}

func TestBindingsBubbleUpFromIfElse(t *testing.T) {
	items := []string{"a", "b"}
	show := true
	tmpl := Build(VBox(
		If(&show).Then(
			List(&items).Render(func(s *string) Component { return Text(s) }).BindNav("j", "k"),
		).Else(
			Text("empty"),
		),
	))
	if len(tmpl.pendingBindings) != 2 {
		t.Fatalf("expected 2 bindings from Then branch, got %d", len(tmpl.pendingBindings))
	}
}

func TestBindingsBubbleUpFromElseBranch(t *testing.T) {
	items := []string{"a", "b"}
	show := false
	tmpl := Build(VBox(
		If(&show).Then(
			Text("loading"),
		).Else(
			List(&items).Render(func(s *string) Component { return Text(s) }).BindVimNav(),
		),
	))
	// j, k, <C-d>, <C-u>, g, G = 6
	if len(tmpl.pendingBindings) != 6 {
		t.Fatalf("expected 6 bindings from Else branch, got %d", len(tmpl.pendingBindings))
	}
}

func TestNoBindingsWhenNotUsed(t *testing.T) {
	items := []string{"a", "b"}
	tmpl := Build(VBox(
		List(&items).Render(func(s *string) Component { return Text(s) }),
	))
	if len(tmpl.pendingBindings) != 0 {
		t.Errorf("expected 0 bindings, got %d", len(tmpl.pendingBindings))
	}
}

// --- #60: named bindings + live key-help ---

// A Named() binding is wired through riffkey HandleNamed, so it surfaces in
// ActiveBindings with its pattern — the live source for key-help.
func TestNamedBindingInActiveBindings(t *testing.T) {
	app := NewApp()
	app.SetView(VBox(
		On(Key("j", func() {}).Named("scroll-down")),
		Text("hi"),
	))
	found := false
	for _, b := range app.ActiveBindings() {
		if b.Name == "scroll-down" {
			found = true
			if b.Pattern != "j" {
				t.Errorf("pattern = %q, want \"j\"", b.Pattern)
			}
		}
	}
	if !found {
		t.Fatal("named binding \"scroll-down\" not present in ActiveBindings")
	}
}

// An unnamed binding still wires (works) but stays off the introspection surface —
// only Named() bindings appear in ActiveBindings.
func TestUnnamedBindingNotIntrospectable(t *testing.T) {
	app := NewApp()
	app.SetView(VBox(
		On(Key("q", func() {})),                       // unnamed
		On(Key("j", func() {}).Named("scroll-down")),  // named
	))
	names := map[string]bool{}
	for _, b := range app.ActiveBindings() {
		names[b.Name] = true
		if b.Pattern == "q" {
			t.Errorf("unnamed binding leaked into ActiveBindings: %+v", b)
		}
	}
	if !names["scroll-down"] {
		t.Fatal("named binding missing from ActiveBindings")
	}
}

func TestKeyHelpRendersActiveBindings(t *testing.T) {
	src := func() []riffkey.Binding {
		return []riffkey.Binding{
			{Name: "scroll-down", Pattern: "j"},
			{Name: "quit", Pattern: "q"},
		}
	}
	help := KeyHelp(src)
	_, h := help.MinSize()
	if h != 2 {
		t.Fatalf("MinSize height = %d, want 2", h)
	}
	buf := NewBuffer(30, 2)
	help.Render(buf, 0, 0, 30, 2)
	row0 := bindingRowString(buf, 0, 30)
	if !strings.Contains(row0, "j") || !strings.Contains(row0, "Scroll down") {
		t.Fatalf("row 0 = %q, want key \"j\" + humanized \"Scroll down\"", row0)
	}
}

func TestKeyHelpPreservesUnderlyingBG(t *testing.T) {
	src := func() []riffkey.Binding {
		return []riffkey.Binding{{Name: "quit", Pattern: "q"}}
	}
	help := KeyHelp(src)
	buf := NewBuffer(20, 1)
	floatBG := RGB(40, 40, 60)
	fill := Style{BG: floatBG}
	for x := 0; x < 20; x++ {
		buf.Set(x, 0, NewCell(' ', fill))
	}
	help.Render(buf, 0, 0, 20, 1)
	for x := 0; x < 6; x++ { // "q  Quit" span
		c := buf.Get(x, 0)
		if c.Rune == 0 || c.Rune == ' ' {
			continue
		}
		if c.Style.BG != floatBG {
			t.Fatalf("cell %d rune %q BG = %+v, want underlying fill BG %+v", x, c.Rune, c.Style.BG, floatBG)
		}
		if c.Style.Attr.Has(AttrPreserveBG) {
			t.Fatalf("cell %d leaked AttrPreserveBG into the buffer", x)
		}
	}
}

// bindingRowString reads row y of buf as a string (test helper).
func bindingRowString(buf *Buffer, y, w int) string {
	var b strings.Builder
	for x := 0; x < w; x++ {
		c := buf.Get(x, y)
		if c.Rune == 0 {
			b.WriteByte(' ')
			continue
		}
		b.WriteRune(c.Rune)
	}
	return b.String()
}

func TestHumanizeBindingName(t *testing.T) {
	cases := map[string]string{
		"scroll-down": "Scroll down",
		"scroll_up":   "Scroll up",
		"quit":        "Quit",
		"":            "",
	}
	for in, want := range cases {
		if got := humanizeBindingName(in); got != want {
			t.Errorf("humanizeBindingName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestOnBindingIsScopedToItsBranch pins the guarantee the docs now teach: a
// binding declared with On inside a branch belongs to that branch, not the app.
// glyph attaches the branch's router when the branch becomes visible and
// disables it when it goes away, so the key simply does not exist while the
// branch is closed. A global handler would have to ask whether the branch is
// showing before acting, and that hand-managed lifetime is the bug On removes.
func TestOnBindingIsScopedToItsBranch(t *testing.T) {
	showModal := false
	tmpl := Build(VBox(
		On(Key("j", func() {}).Named("next")),
		If(&showModal).Eq(true).Then(
			VBox(
				On(Key("<Escape>", func() { showModal = false }).Named("dismiss")),
				Text("modal"),
			),
		),
	))

	for _, b := range tmpl.pendingRouteBindings {
		if b.pattern == "<Escape>" {
			t.Fatal("Escape leaked onto the root router — it belongs to the modal branch")
		}
	}
	if len(tmpl.pendingRouteBindings) != 1 || tmpl.pendingRouteBindings[0].pattern != "j" {
		t.Fatalf("root should carry only the always-on binding, got %+v", tmpl.pendingRouteBindings)
	}

	found := false
	for _, child := range routeChildTemplates(tmpl) {
		for _, b := range child.pendingRouteBindings {
			if b.pattern == "<Escape>" && b.name == "dismiss" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("the branch does not carry its own Escape binding — On is not branch-scoped")
	}
}

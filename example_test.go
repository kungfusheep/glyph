package glyph_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/kungfusheep/glyph"
)

type termData struct {
	W     int      `json:"w"`
	H     int      `json:"h"`
	Lines []string `json:"lines"`
}

func renderAndPrint(name string, tree Component, w, h int) {
	buf := NewBuffer(w, h)
	Build(tree).Execute(buf, int16(w), int16(h))
	fmt.Println(strings.TrimSpace(buf.StringTrimmed()))

	if dir := os.Getenv("GLYPH_PREVIEWS_DIR"); dir != "" {
		td := termData{W: w, H: h}
		for y := 0; y < h; y++ {
			td.Lines = append(td.Lines, buf.GetLineStyled(y))
		}
		data, _ := json.Marshal(td)
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, name+".json"), data, 0644)
	}
}

func renderWithEffects(name string, tree Component, w, h int) {
	buf := NewBuffer(w, h)
	tmpl := Build(tree)
	tmpl.Execute(buf, int16(w), int16(h))
	effects := tmpl.ScreenEffects()
	if len(effects) > 0 {
		ppCtx := PostContext{Width: w, Height: h, DefaultFG: RGB(200, 196, 184), DefaultBG: RGB(26, 26, 24)}
		for _, e := range effects {
			e.Apply(buf, ppCtx)
		}
	}
	fmt.Println(strings.TrimSpace(buf.StringTrimmed()))

	if dir := os.Getenv("GLYPH_PREVIEWS_DIR"); dir != "" {
		td := termData{W: w, H: h}
		for y := 0; y < h; y++ {
			td.Lines = append(td.Lines, buf.GetLineStyled(y))
		}
		data, _ := json.Marshal(td)
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, name+".json"), data, 0644)
	}
}

// Vertical stack.
// Call VBox directly with children to stack them top to bottom.
func ExampleVBoxFn() {
	// example:
	tree := VBox(
		Text("First"),
		Text("Second"),
		Text("Third"),
	)
	// :example

	renderAndPrint("VBoxFn", tree, 20, 3)
	// Output:
	// First
	// Second
	// Third
}

// Template syntax.
// Chain methods to configure, then call as a function with children. Configure once, render many.
func ExampleVBoxFn_chained() {
	// example:
	tree := VBox.Gap(1).Border(BorderRounded)(
		Text("First"),
		Text("Second"),
	)
	// :example

	renderAndPrint("VBoxFn_chained", tree, 20, 6)
	// Output:
	// ╭──────────────────╮
	// │First             │
	// │                  │
	// │Second            │
	// │                  │
	// ╰──────────────────╯
}

// Flex growth.
// Grow distributes remaining space. Children without Grow keep their natural size.
func ExampleVBoxFn_grow() {
	// example:
	tree := VBox.Height(7).Border(BorderRounded)(
		Text("header"),
		VBox.Grow(1)(Text("expands")),
		Text("footer"),
	)
	// :example

	renderAndPrint("VBoxFn_grow", tree, 20, 9)
	// Output:
	// ╭──────────────────╮
	// │header            │
	// │expands           │
	// │                  │
	// │                  │
	// │footer            │
	// ╰──────────────────╯
}

// Horizontal layout.
// Arrange children side by side with a gap between them.
func ExampleHBoxFn() {
	// example:
	tree := HBox.Gap(2)(
		Text("left"),
		Text("right"),
	)
	// :example

	renderAndPrint("HBoxFn", tree, 20, 1)
	// Output: left  right
}

// Sidebar pattern.
// Combine WidthPct and Grow for a fixed sidebar with a flexible main area.
func ExampleHBoxFn_widths() {
	// example:
	tree := HBox(
		VBox.WidthPct(0.3).Border(BorderRounded)(
			Text("sidebar"),
		),
		VBox.Grow(1).Border(BorderRounded)(
			Text("main content"),
		),
	)
	// :example

	renderAndPrint("HBoxFn_widths", tree, 40, 3)
	// Output:
	// ╭──────────╮╭──────────────────────────╮
	// │sidebar   ││main content              │
	// ╰──────────╯╰──────────────────────────╯
}

// Basic layering.
// Overlay renders its child on top of sibling content. Place it inside a container alongside the base content.
func ExampleOverlayFn() {
	// example:
	tree := VBox.Grow(1)(
		Text("base line 1"),
		Text("base line 2"),
		Text("base line 3"),
		Overlay.Centered()(
			VBox.Border(BorderRounded)(
				Text("popup"),
			),
		),
	)
	// :example

	renderAndPrint("OverlayFn", tree, 24, 5)
	// Output:
	// base line 1
	// ╭──────────────────────╮
	// │popupine 3            │
	// ╰──────────────────────╯
}

// Modal dialog.
// Backdrop dims everything behind it. Centered positions the overlay in the middle of the screen.
func ExampleOverlayFn_modal() {
	// example:
	tree := VBox.Grow(1)(
		Text("main content"),
		Text("more content"),
		Overlay.Centered().Backdrop().BackdropFG(BrightBlack)(
			VBox.Border(BorderRounded)(
				Text("confirm delete?"),
				HBox.Gap(2)(Text("[y]es"), Text("[n]o")),
			),
		),
	)
	// :example

	renderAndPrint("OverlayFn_modal", tree, 30, 6)
	// Output:
	// main content
	// ╭────────────────────────────╮
	// │confirm delete?             │
	// │[y]es  [n]o                 │
	// ╰────────────────────────────╯
}

// Static text.
// Text that never changes. Pass a string literal directly.
func ExampleText() {
	// example:
	tree := Text("hello world")
	// :example

	renderAndPrint("Text", tree, 20, 1)
	// Output: hello world
}

// Pointer binding.
// Pass a pointer so the rendered text reflects the current value. Mutate the string, then trigger a render to see it.
func ExampleText_pointer() {
	// example:
	msg := "dynamic"
	tree := Text(&msg)
	// :example

	renderAndPrint("Text_pointer", tree, 20, 1)
	// Output: dynamic
}

// Inline styling.
// Chain style methods. Bold, Dim, Italic, Underline, and FG/BG are all available.
func ExampleText_styled() {
	// example:
	msg := "styled"
	tree := Text(&msg).Bold().FG(Cyan)
	// :example

	renderAndPrint("Text_styled", tree, 20, 1)
	// Output: styled
}

// Text input.
// A managed text input with placeholder text shown when empty.
func ExampleInput() {
	// example:
	Input().Placeholder("Type here...")
	// :example

	tree := Text("Type here...").Dim()
	renderAndPrint("Input", tree, 20, 1)
	// Output: Type here...
}

// Password input.
// Mask hides input characters for sensitive fields.
func ExampleInput_mask() {
	// example:
	Input().Placeholder("Password").Mask('*')
	// :example

	tree := Text("Password").Dim()
	renderAndPrint("Input_mask", tree, 20, 1)
	// Output: Password
}

// Navigable list.
// A list over a pointer to a slice. The Render callback receives a pointer to each item for in-place updates.
func ExampleList() {
	// example:
	items := []string{"Alpha", "Beta", "Gamma"}

	List(&items).Render(func(item *string) Component {
		return Text(item)
	}).BindNav("j", "k")
	// :example

	tree := VBox(
		Text("> Alpha"),
		Text("  Beta"),
		Text("  Gamma"),
	)
	renderAndPrint("List", tree, 20, 3)
	// Output:
	// > Alpha
	//   Beta
	//   Gamma
}

// Toggleable list.
// Like List, but each item has a toggleable checkbox. The Done field is toggled in place through the pointer.
func ExampleCheckList() {
	// example:
	type Task struct {
		Name string
		Done bool
	}
	tasks := []Task{{Name: "Ship it"}, {Name: "Test it"}}

	CheckList(&tasks).Render(func(t *Task) Component {
		return Text(&t.Name)
	}).BindNav("j", "k").BindToggle(" ")
	// :example

	tree := VBox(
		Text("> ☑ Ship it"),
		Text("  ☐ Test it"),
	)
	renderAndPrint("CheckList", tree, 20, 2)
	// Output:
	// > ☑ Ship it
	//   ☐ Test it
}

// Tab bar.
// Bound to an int pointer. The selected index is written when the user picks a tab.
func ExampleTabs() {
	// example:
	var active int
	tabs := []string{"General", "Advanced", "About"}
	tree := Tabs(tabs, &active)
	// :example

	renderAndPrint("Tabs", tree, 40, 1)
	// Output: General  Advanced  About
}

// Custom tab styles.
// Customize appearance with Kind for the shape and per-state styles for active/inactive tabs.
func ExampleTabs_styled() {
	// example:
	var active int
	tabs := []string{"Code", "Preview", "Settings"}
	tree := Tabs(tabs, &active).
		Kind(TabsStyleBox).
		ActiveStyle(Style{FG: Cyan, Attr: AttrBold}).
		InactiveStyle(Style{FG: BrightBlack})
	// :example

	renderAndPrint("Tabs_styled", tree, 40, 3)
	// Output:
	// ┌──────┐  ┌─────────┐  ┌──────────┐
	// │ Code │  │ Preview │  │ Settings │
	// └──────┘  └─────────┘  └──────────┘
}

// Searchable list.
// The extract function tells the filter which string to match against for each item.
func ExampleFilterList() {
	// example:
	items := []string{"Alpha", "Beta", "Gamma"}

	FilterList(&items, func(s *string) string { return *s }).
		Render(func(item *string) Component {
			return Text(item)
		})
	// :example

	tree := VBox(
		Text("filter...").Dim(),
		Text("> Alpha"),
		Text("  Beta"),
		Text("  Gamma"),
	)
	renderAndPrint("FilterList", tree, 20, 4)
	// Output:
	// filter...
	// > Alpha
	//   Beta
	//   Gamma
}

// Struct table.
// Generate a table from a struct slice. Column options control formatting.
func ExampleAutoTable() {
	// example:
	type Row struct {
		Name string
		CPU  float64
	}
	rows := []Row{{Name: "api", CPU: 42.5}}
	tree := AutoTable(&rows).Column("CPU", FormatNumber(1))
	// :example

	renderAndPrint("AutoTable", tree, 40, 5)
	// Output:
	// Name                                CPU
	// api                                42.5
}

// Column formatters.
// Mix formatters: percentages, byte sizes, booleans, and colour-coded changes.
func ExampleAutoTable_columns() {
	// example:
	type Service struct {
		Name   string
		CPU    float64
		Memory uint64
		Active bool
		Growth float64
	}
	rows := []Service{
		{Name: "api", CPU: 82.3, Memory: 1073741824, Active: true, Growth: 0.12},
	}
	tree := AutoTable(&rows).
		Column("CPU", FormatPercent(1)).
		Column("Memory", FormatBytes()).
		Column("Active", FormatBool("Yes", "No")).
		Column("Growth", FormatPercentChange(1))
	// :example

	renderAndPrint("AutoTable_columns", tree, 60, 5)
	// Output:
	// Name            CPU       Memory    Active          Growth
	// api           82.3%       1.0 GB     Yes             +0.1%
}

// Currency formatting.
// Format a float as a monetary value with the given symbol and decimal places.
func ExampleFormatCurrency() {
	// example:
	type Invoice struct {
		Item  string
		Price float64
	}
	rows := []Invoice{{Item: "Widget", Price: 42.50}}
	tree := AutoTable(&rows).Column("Price", FormatCurrency("$", 2))
	// :example

	renderAndPrint("Currency", tree, 40, 5)
	// Output:
	// Item                              Price
	// Widget                           $42.50
}

// Progress bar.
// Bound to an int pointer (0–100). Width sets the bar length in cells.
func ExampleProgress() {
	// example:
	pct := 60
	tree := Progress(&pct).Width(20)
	// :example

	renderAndPrint("Progress", tree, 25, 1)
	// Output: ████████████
}

// Animated spinner.
// The frame pointer must be incremented by the caller (e.g. via a ticker goroutine).
func ExampleSpinner() {
	// example:
	frame := 0
	tree := Spinner(&frame).Frames(SpinnerBraille)
	// :example

	renderAndPrint("Spinner", tree, 5, 1)
	// Output: ⠋
}

// Radio group.
// Bound to an int pointer. The selected index updates when the user navigates and presses enter.
func ExampleRadio() {
	// example:
	var selected int
	tree := Radio(&selected, "Small", "Medium", "Large").BindNav("j", "k")
	// :example

	renderAndPrint("Radio", tree, 20, 3)
	// Output:
	// ◉ Small
	// ○ Medium
	// ○ Large
}

// Single checkbox.
// Bound to a bool pointer.
func ExampleCheckbox() {
	// example:
	var agreed bool
	tree := Checkbox(&agreed, "I agree to the terms")
	// :example

	renderAndPrint("Checkbox", tree, 30, 1)
	// Output: ☐ I agree to the terms
}

// Leader line.
// Label on the left, value on the right, dots filling the gap. Both sides read from pointers each frame.
func ExampleLeader() {
	// example:
	label := "Total"
	value := "$42.00"
	tree := Leader(&label, &value)
	// :example

	renderAndPrint("Leader", tree, 20, 1)
	// Output: Total.........$42.00
}

// Mini chart.
// A line chart from a float64 slice pointer. Append values and trigger a render to update.
func ExampleSparkline() {
	// example:
	values := []float64{1, 3, 5, 2, 8, 4}
	tree := Sparkline(&values).Width(20).FG(Green)
	// :example

	renderAndPrint("Sparkline", tree, 25, 1)
	// Output: ▁▃▅▂█▄
}

// Flexible spacer.
// Pushes siblings apart. In a VBox, it pushes content to the top and bottom.
func ExampleSpace() {
	// example:
	tree := VBox(
		Text("header"),
		Space(),
		Text("footer"),
	)
	// :example

	renderAndPrint("Space", tree, 20, 3)
	// Output:
	// header
	//
	// footer
}

// Fixed-height gap.
// Use in VBox to add exact spacing between elements.
func ExampleSpace_height() {
	// example:
	tree := VBox(
		Text("above"),
		SpaceH(1),
		Text("below"),
	)
	// :example

	renderAndPrint("SpaceH", tree, 20, 3)
	// Output:
	// above
	//
	// below
}

// Fixed-width gap.
// Use in HBox to add exact spacing between elements.
func ExampleSpace_width() {
	// example:
	tree := HBox(
		Text("left"),
		SpaceW(4),
		Text("right"),
	)
	// :example

	renderAndPrint("SpaceW", tree, 20, 1)
	// Output: left    right
}

// Horizontal divider.
// A line that fills the available width.
func ExampleHRule() {
	// example:
	tree := VBox(
		Text("above"),
		HRule(),
		Text("below"),
	)
	// :example

	renderAndPrint("HRule", tree, 10, 3)
	// Output:
	// above
	// ──────────
	// below
}

// Vertical divider.
// A line that fills the available height.
func ExampleVRule() {
	// example:
	tree := HBox(
		Text("left"),
		VRule(),
		Text("right"),
	)
	// :example

	renderAndPrint("VRule", tree, 15, 1)
	// Output: left│right
}

// Scrollbar.
// Parameters are total content height, visible viewport height, and a pointer to scroll position.
func ExampleScrollbar() {
	// example:
	pos := 0
	tree := HBox(
		VBox(Text("line 1"), Text("line 2"), Text("line 3")),
		Scrollbar(50, 10, &pos),
	)
	// :example

	renderAndPrint("Scroll", tree, 15, 3)
	// Output:
	// line 1        █
	// line 2
	// line 3
}

// Conditional show.
// Show or hide content based on a bool pointer. The value is checked every frame.
func ExampleIf() {
	// example:
	show := true
	tree := If(&show).Then(Text("visible"))
	// :example

	renderAndPrint("If", tree, 20, 1)
	// Output: visible
}

// Toggle views.
// Switch between two views based on state.
func ExampleIf_else() {
	// example:
	loggedIn := false
	tree := If(&loggedIn).Then(Text("dashboard")).Else(Text("login"))
	// :example

	renderAndPrint("If_else", tree, 20, 1)
	// Output: login
}

// Value matching.
// Match against a specific value. Works with strings, ints, or any comparable type.
func ExampleCondition_eq() {
	// example:
	status := "active"
	tree := If(&status).Eq("active").Then(
		Text("online"),
	).Else(
		Text("offline"),
	)
	// :example

	renderAndPrint("Condition_eq", tree, 20, 1)
	// Output: online
}

// Multi-way branch.
// Branch on a value with named cases. Default catches unmatched values.
func ExampleSwitch() {
	// example:
	mode := "edit"
	tree := Switch(&mode).
		Case("edit", Text("editing")).
		Case("preview", Text("previewing")).
		Default(Text("idle"))
	// :example

	renderAndPrint("Switch", tree, 20, 1)
	// Output: editing
}

// Numeric comparison.
// IfOrd supports Gt, Lt, Gte, Lte for any ordered type (int, float64, string).
func ExampleOrdCondition() {
	// example:
	count := 5
	tree := IfOrd(&count).Gte(10).Then(Text("many")).Else(Text("few"))
	// :example

	renderAndPrint("OrdCondition", tree, 20, 1)
	// Output: few
}

// Slice iteration.
// Iterate a slice with per-item templates. The callback is called once at build time to define the per-item template; glyph re-evaluates the slice pointer each frame.
func ExampleForEach() {
	// example:
	type Todo struct {
		Title string
		Done  bool
	}
	items := []Todo{{Title: "Ship it"}, {Title: "Test it"}}

	tree := VBox(ForEach(&items, func(item *Todo) Component {
		return Text(&item.Title)
	}))
	// :example

	renderAndPrint("ForEach", tree, 20, 2)
	// Output:
	// Ship it
	// Test it
}

// Inline spans.
// Mix Bold, Dim, Italic, FG and plain strings in a single line.
func ExampleRich() {
	// example:
	tree := Rich(
		Bold("Important: "),
		Dim("supporting detail "),
		FG("error", Red),
	)
	// :example

	renderAndPrint("Rich", tree, 40, 1)
	// Output: Important: supporting detail error
}

// Custom span styles.
// Styled creates a span with a full Style struct for fine-grained control.
func ExampleSpan() {
	// example:
	tree := Rich(
		Styled("custom", Style{FG: Hex(0xFF5500), Attr: AttrBold}),
		" mixed with ",
		Italic("emphasis"),
	)
	// :example

	renderAndPrint("Span", tree, 40, 1)
	// Output: custom mixed with emphasis
}

// Full-screen app.
// SetView compiles the tree once, Handle binds keys, Run blocks until Stop is called.
func ExampleNewApp() {
	// example:
	app := NewApp()
	app.SetView(VBox(
		Text("hello from glyph"),
		Text("press q to quit"),
	))
	app.Handle("q", func() { app.Stop() })
	app.Run()
	// :example

	tree := VBox(
		Text("hello from glyph"),
		Text("press q to quit"),
	)
	renderAndPrint("NewApp", tree, 20, 2)
	// Output:
	// hello from glyph
	// press q to quit
}

// Inline app.
// Renders within the terminal flow instead of taking over the screen. Height sets the visible rows.
func ExampleNewInlineApp() {
	// example:
	app := NewInlineApp()

	status := "loading..."
	app.SetView(Text(&status))
	app.Height(3).ClearOnExit(true)
	app.Run()
	// :example

	tree := Text(&status)
	renderAndPrint("NewInlineApp", tree, 20, 1)
	// Output: loading...
}

// Multi-view routing.
// Named views with per-view key bindings. Go switches between views; each view is compiled independently.
func ExampleApp_multiView() {
	// example:
	app := NewApp()

	app.View("home", VBox(
		Text("Welcome"),
		Text("press n for next"),
	)).Handle("n", func() { app.Go("detail") })

	app.View("detail", VBox(
		Text("Detail view"),
		Text("press b for back"),
	)).Handle("b", func() { app.Go("home") })

	app.RunFrom("home")
	// :example

	tree := VBox(
		Text("Welcome"),
		Text("press n for next"),
	)
	renderAndPrint("App_multiView", tree, 20, 2)
	// Output:
	// Welcome
	// press n for next
}

// Background updates.
// Mutate state, then call RequestRender to trigger a redraw.
func ExampleApp_goroutine() {
	// example:
	app := NewApp()

	status := "waiting..."
	app.SetView(Text(&status))
	app.Handle("q", func() { app.Stop() })

	time.AfterFunc(2*time.Second, func() {
		status = "done!"
		app.RequestRender()
	})

	app.Run()
	// :example

	tree := Text(&status)
	renderAndPrint("App_goroutine", tree, 20, 1)
	// Output: waiting...
}

// Views with text input.
// NoCounts disables vim-style count prefixes, preventing number keys from being swallowed before reaching an input.
func ExampleViewBuilder() {
	// example:
	app := NewApp()

	var name string
	app.View("editor",
		VBox(
			Text("Editor"),
			Input().Placeholder("type here..."),
		),
	).NoCounts().Handle("<C-s>", func() {
		name = name + " saved"
	}).Handle("escape", func() {
		app.Go("home")
	})
	// :example
	_ = name

	tree := VBox(
		Text("Editor"),
		Text("type here...").Dim(),
	)
	renderAndPrint("ViewBuilder", tree, 20, 2)
	// Output:
	// Editor
	// type here...
}

// Style chaining.
// Build a style by chaining methods on DefaultStyle. CascadeStyle on a container applies it to all descendants.
func ExampleDefaultStyle() {
	// example:
	s := DefaultStyle().Bold().Foreground(Cyan)
	tree := VBox.CascadeStyle(&s)(
		Text("all children inherit bold cyan"),
	)
	// :example

	renderAndPrint("DefaultStyle", tree, 34, 1)
	// Output: all children inherit bold cyan
}

// Struct literal.
// Construct a Style directly when you know the exact fields. Attr flags combine with bitwise OR.
func ExampleStyle() {
	// example:
	highlight := Style{FG: Yellow, Attr: AttrBold | AttrUnderline}
	tree := Text("warning").Style(highlight)
	// :example

	renderAndPrint("Style", tree, 20, 1)
	// Output: warning
}

// Style margin.
// Margin is part of Style. Values are top, right, bottom, left (CSS order).
func ExampleStyle_margin() {
	// example:
	s := DefaultStyle().MarginTRBL(1, 2, 1, 2)
	tree := Text("margined text").Style(s)
	// :example

	renderAndPrint("Style_margin", tree, 20, 3)
	// Output: margined text
}

// Container padding.
// Padding adds space between a container's border and its content. Set on VBox, HBox, or any container.
func ExampleVBoxFn_padding() {
	// example:
	tree := VBox.Padding(1).Border(BorderRounded)(
		Text("padded content"),
	)
	// :example

	renderAndPrint("VBoxFn_padding", tree, 20, 5)
	// Output:
	// ╭──────────────────╮
	// │                  │
	// │ padded content   │
	// │                  │
	// ╰──────────────────╯
}

// Horizontal padding.
// PaddingVH sets vertical and horizontal padding separately.
func ExampleHBoxFn_padding() {
	// example:
	tree := HBox.PaddingVH(0, 2).Border(BorderRounded).Gap(2)(
		Text("left"),
		Text("right"),
	)
	// :example

	renderAndPrint("HBoxFn_padding", tree, 24, 3)
	// Output:
	// ╭──────────────────────╮
	// │  left  right         │
	// ╰──────────────────────╯
}

// Hex colour.
// Takes a uint32, not a string. Use Go hex literals.
func ExampleHex() {
	// example:
	tree := Text("branded").FG(Hex(0xFF5500))
	// :example

	renderAndPrint("Hex", tree, 20, 1)
	// Output: branded
}

// RGB colour.
// Precise 24-bit colour from red, green, blue components.
func ExampleRGB() {
	// example:
	tree := Text("vivid").FG(RGB(255, 85, 0))
	// :example

	renderAndPrint("RGB", tree, 20, 1)
	// Output: vivid
}

// Colour blending.
// Lerp blends two colours. t=0 returns the first, t=1 returns the second, 0.5 is the midpoint.
func ExampleLerp() {
	// example:
	pct := 0.75
	bar := Lerp(Red, Green, pct)
	tree := Text("75%").FG(bar)
	// :example

	renderAndPrint("Lerp", tree, 20, 1)
	// Output: 75%
}

// Terminal palette.
// Ansi16 uses the terminal's 16-colour palette (0–15). These respect the user's terminal theme.
func ExampleAnsi16() {
	// example:
	tree := Text("theme-aware").FG(Ansi16(9))
	// :example

	renderAndPrint("Ansi16", tree, 20, 1)
	// Output: theme-aware
}

// Extended palette.
// Ansi256 uses the 256-colour extended palette.
func ExampleAnsi256() {
	// example:
	tree := Text("orange-ish").FG(Ansi256(214))
	// :example

	renderAndPrint("Ansi256", tree, 20, 1)
	// Output: orange-ish
}

// Custom rendering.
// Escape hatch for custom rendering. Measure reports size, Render draws directly into the cell buffer.
func ExampleCustom() {
	// example:
	tree := Custom(
		func(availW int16) (w, h int16) { return availW, 1 },
		func(buf *Buffer, x, y, w, h int16) {
			for i := int16(0); i < w; i++ {
				buf.Set(int(x+i), int(y), Cell{Rune: '='})
			}
		},
	)
	// :example

	renderAndPrint("Widget", tree, 10, 1)
	// Output: ==========
}

// Streaming log.
// Stream lines from any io.Reader into a scrollable log view. AutoScroll keeps the view pinned to the bottom.
func ExampleLogC() {
	// example:
	r := strings.NewReader("line 1\nline 2\nline 3\n")

	Log(r).MaxLines(1000).AutoScroll(true).BindVimNav()
	// :example

	tree := VBox(
		Text("line 1"),
		Text("line 2"),
		Text("line 3"),
	)
	renderAndPrint("LogC", tree, 20, 3)
	// Output:
	// line 1
	// line 2
	// line 3
}

// Stdin pipe.
// Pipe stdout from a subprocess or os.Stdin directly into a log view. The reader is consumed in a background goroutine.
func ExampleLogC_fromStdin() {
	// example:
	Log(os.Stdin).MaxLines(5000).Grow(1)
	// :example

	tree := VBox(
		Text("piped line 1"),
		Text("piped line 2"),
	)
	renderAndPrint("LogC_fromStdin", tree, 20, 2)
	// Output:
	// piped line 1
	// piped line 2
}

// Filterable log.
// A log view with a built-in fuzzy search input. Lines are filtered as you type; matching is fzf-style.
func ExampleFilterLogC() {
	// example:
	r := strings.NewReader("info: started\nerror: failed\ninfo: recovered\n")

	FilterLog(r).Placeholder("filter logs...").MaxLines(5000).BindVimNav()
	// :example

	tree := VBox(
		Text("filter logs...").Dim(),
		Text("info: started"),
		Text("error: failed"),
		Text("info: recovered"),
	)
	renderAndPrint("FilterLogC", tree, 25, 4)
	// Output:
	// filter logs...
	// info: started
	// error: failed
	// info: recovered
}

// Scrollable canvas.
// LayerView displays a Layer, a virtual canvas that can be larger than the viewport.
func ExampleLayerViewC() {
	// example:
	layer := NewLayer()

	LayerView(layer).Grow(1)
	// :example

	tree := VBox.Border(BorderRounded)(
		Text("viewport content"),
	)
	renderAndPrint("LayerViewC", tree, 22, 3)
	// Output:
	// ╭────────────────────╮
	// │viewport content    │
	// ╰────────────────────╯
}

// Manual content.
// Write lines directly to a Layer for manual content control. The Render callback fires when the viewport resizes.
func ExampleLayer() {
	// example:
	layer := NewLayer()
	layer.Render = func() {
		layer.SetLineString(0, "redrawn", DefaultStyle())
	}
	layer.SetLineString(0, "first line", DefaultStyle())
	layer.SetLineString(1, "second line", DefaultStyle().Bold())

	LayerView(layer).Grow(1)
	// :example

	tree := VBox(
		Text("first line"),
		Text("second line").Bold(),
	)
	renderAndPrint("Layer", tree, 20, 2)
	// Output:
	// first line
	// second line
}

// Jump target.
// Wrap any component as a jump target. When jump mode is active, labelled hints appear over each target.
func ExampleJumpC() {
	// example:
	selected := "none"
	Jump(Text("clickable item"), func() {
		selected = "clicked"
	})
	// :example
	_ = selected

	tree := Text("clickable item")
	renderAndPrint("JumpC", tree, 20, 1)
	// Output: clickable item
}

// App-level jump mode.
// Press the jump key to enter jump mode, then type a target label to select it.
func ExampleJumpC_app() {
	// example:
	app := NewApp()

	items := []string{"Save", "Load", "Quit"}
	app.SetView(VBox(
		Jump(Text(&items[0]), func() {}),
		Jump(Text(&items[1]), func() {}),
		Jump(Text(&items[2]), func() { app.Stop() }),
	))

	app.JumpKey("f")
	app.Run()
	// :example

	tree := VBox(
		Text("Save"),
		Text("Load"),
		Text("Quit"),
	)
	renderAndPrint("JumpC_app", tree, 20, 3)
	// Output:
	// Save
	// Load
	// Quit
}

// Compile a template.
// Build compiles a UI tree into a Template. This is the same step that SetView performs internally. Useful for rendering into a Layer or for headless testing.
func ExampleBuild() {
	// example:
	tree := VBox(
		Text("hello"),
		Text("world"),
	)
	tmpl := Build(tree)
	// :example
	_ = tmpl

	renderAndPrint("Build", tree, 20, 2)
	// Output:
	// hello
	// world
}

// Predefined theme.
// CascadeStyle sets the base style for all descendants; individual elements can override with per-element styles.
func ExampleThemeEx() {
	// example:
	theme := ThemeDark
	tree := VBox.CascadeStyle(&theme.Base).Border(BorderRounded).BorderFG(theme.Border.FG)(
		Text("normal text"),
		Text("muted").Style(theme.Muted),
		Text("accent").Style(theme.Accent),
		Text("error!").Style(theme.Error),
	)
	// :example

	renderAndPrint("ThemeEx", tree, 20, 6)
	// Output:
	// ╭──────────────────╮
	// │normal text       │
	// │muted             │
	// │accent            │
	// │error!            │
	// ╰──────────────────╯
}

// Built-in borders.
// Choose from BorderSingle, BorderRounded, or BorderDouble.
func ExampleBorderStyle() {
	// example:
	tree := VBox.Border(BorderDouble).BorderFG(Cyan)(
		Text("double-bordered"),
	)
	// :example

	renderAndPrint("BorderStyle", tree, 20, 3)
	// Output:
	// ╔══════════════════╗
	// ║double-bordered   ║
	// ╚══════════════════╝
}

// Custom borders.
// Define custom borders with any rune for each edge and corner.
func ExampleBorderStyle_custom() {
	// example:
	ascii := BorderStyle{
		Horizontal:  '-',
		Vertical:    '|',
		TopLeft:     '+',
		TopRight:    '+',
		BottomLeft:  '+',
		BottomRight: '+',
	}
	tree := VBox.Border(ascii)(Text("ascii box"))
	// :example

	renderAndPrint("BorderStyle_custom", tree, 20, 3)
	// Output:
	// +------------------+
	// |ascii box         |
	// +------------------+
}

// Text alignment.
// Align is set via the Style struct. AlignLeft, AlignCenter, or AlignRight within a fixed width.
func ExampleAlign() {
	// example:
	tree := VBox(
		Text("left-aligned"),
		Text("centered").Style(Style{Align: AlignCenter}).Width(40),
		Text("right-aligned").Style(Style{Align: AlignRight}).Width(40),
	)
	// :example

	renderAndPrint("Align", tree, 40, 3)
	// Output:
	// left-aligned
	//                 centered
	//                            right-aligned
}

// Tab shapes.
// Three tab styles: TabsStyleUnderline (default), TabsStyleBox, and TabsStyleBracket.
func ExampleTabsStyle() {
	// example:
	var active int
	tabs := []string{"Files", "Search", "Git"}
	tree := Tabs(tabs, &active).Kind(TabsStyleBracket)
	// :example

	renderAndPrint("TabsStyle", tree, 40, 1)
	// Output: [Files]  [Search]  [Git]
}

// Custom layout.
// Arrange lets you define a fully custom layout function. It receives child sizes and available space, returns rects.
func ExampleArrange() {
	// example:
	grid := Arrange(func(children []ChildSize, w, h int) []Rect {
		cols := 3
		cellW := w / cols
		rects := make([]Rect, len(children))
		for i := range children {
			rects[i] = Rect{
				X: (i % cols) * cellW,
				Y: (i / cols) * 2,
				W: cellW,
				H: 2,
			}
		}
		return rects
	})
	tree := grid(Text("a"), Text("b"), Text("c"))
	// :example

	renderAndPrint("Arrange", tree, 30, 2)
	// Output: a         b         c
}

// Scoped helpers.
// Define scopes local helper functions inside the view tree. The function runs at build time and returns a component.
func ExampleDefine() {
	// example:
	a, b, c := true, false, true
	tree := Define(func() Component {
		dot := func(v *bool) Component {
			return If(v).Then(Text("●").FG(Green)).Else(Text("○").FG(Red))
		}
		return HBox.Gap(1)(dot(&a), dot(&b), dot(&c))
	})
	// :example

	renderAndPrint("Define", tree, 10, 1)
	// Output: ● ○ ●
}

// Focus cycling.
// FocusManager coordinates focus between multiple inputs. Tab/Shift-Tab cycles through ManagedBy components.
func ExampleFocusManager() {
	// example:
	fm := NewFocusManager()

	VBox(
		Input().Placeholder("Name").ManagedBy(fm),
		Input().Placeholder("Email").ManagedBy(fm),
	)
	// :example
	_ = fm

	tree := VBox(
		Text("Name").Dim(),
		Text("Email").Dim(),
	)
	renderAndPrint("FocusManager", tree, 20, 2)
	// Output:
	// Name
	// Email
}

// Collapsible tree.
// TreeView renders a collapsible tree from TreeNode structs. Set Expanded to control which nodes start open.
func ExampleTreeView() {
	// example:
	root := &TreeNode{
		Label: "src",
		Children: []*TreeNode{
			{Label: "main.go"},
			{Label: "lib", Children: []*TreeNode{
				{Label: "utils.go"},
				{Label: "types.go"},
			}, Expanded: true},
		},
		Expanded: true,
	}

	tree := TreeView{Root: root, ShowRoot: true, Indent: 2, ShowLines: true}
	// :example

	renderAndPrint("TreeView", tree, 20, 5)
	// Output:
	// ▼ src
	// │   main.go
	//   ▼ lib
	//   │   utils.go
	//       types.go
}

// Case transform.
// Transform text case via the Style struct. Transforms run at render time without modifying the source string.
func ExampleTextTransform() {
	// example:
	tree := Text("hello world").Style(Style{Transform: TransformUppercase})
	// :example

	renderAndPrint("TextTransform", tree, 20, 1)
	// Output: HELLO WORLD
}

// Registration form.
// Form arranges labeled fields with aligned labels and automatic focus management. Mix inputs, checkboxes and validators.
func ExampleFormFn() {
	// example:
	var name, email, pass string
	var terms bool

	tree := Form.LabelBold()(
		Field("Name", Input(&name).Placeholder("Jane Doe")),
		Field("Email", Input(&email).Placeholder("jane@example.com")),
		Field("Password", Input(&pass).Placeholder("min 8 chars").Mask('*')),
		Field("Terms", Checkbox(&terms, "I agree")),
	)
	// :example

	terms = true
	renderAndPrint("FormFn", tree, 40, 4)
	// Output:
	// ▸    Name: Jane Doe
	//     Email: jane@example.com
	//  Password: min 8 chars
	//     Terms: ☑ I agree
}

// Submit handler.
// Submit handler.
// OnSubmit receives the form instance so the callback can validate, reset, or read field state.
func ExampleFormFn_onSubmit() {
	// example:
	var name, email, pass string
	var terms bool

	tree := Form.LabelBold().OnSubmit(func(f *FormC) {
		_ = f.ValidateAll()
	})(
		Field("Name", Input(&name).Validate(VRequired)),
		Field("Email", Input(&email).Validate(VEmail)),
		Field("Password", Input(&pass).Mask('*').Validate(VMinLen(8))),
		Field("Terms", Checkbox(&terms, "I agree").Validate(VTrue)),
	)
	// :example

	terms = true
	renderAndPrint("FormFn_onSubmit", tree, 40, 4)
	// Output:
	// ▸    Name:
	//     Email:
	//  Password:
	//     Terms: ☑ I agree
}

// Mixed control types.
// Inputs, checkboxes and radio groups all work as form fields with automatic focus cycling.
func ExampleFormFn_mixedControls() {
	// example:
	var name string
	var notify bool
	var role int

	tree := Form.LabelBold()(
		Field("Name", Input(&name).Placeholder("you")),
		Field("Notify", Checkbox(&notify, "Send me updates")),
		Field("Role", Radio(&role, "Viewer", "Editor", "Admin")),
	)
	// :example

	notify = true
	role = 1
	renderAndPrint("FormFn_mixedControls", tree, 40, 5)
	// Output:
	// ▸  Name: you
	//  Notify: ☑ Send me updates
	//    Role: ○ Viewer
	//          ◉ Editor
	//          ○ Admin
}

// ---------------------------------------------------------------------------
// Screen effects
// ---------------------------------------------------------------------------

// Single post-processing effect.
// Place ScreenEffect anywhere in the view tree — it applies to the whole screen.
func ExampleScreenEffect() {
	// example:
	tree := VBox(
		VBox.Border(BorderRounded)(
			HBox(
				Text("● online").FG(Green),
				Text("  "),
				Text("■ queue: 42").FG(Yellow),
			),
			HBox(
				Text("cpu ").FG(Cyan),
				Text("████░░░░").FG(RGB(100, 200, 255)),
			),
			Text("ready").FG(RGB(200, 196, 184)),
		),
		ScreenEffect(SEVignette()),
	)
	// :example

	renderWithEffects("ScreenEffect", tree, 40, 5)
	// Output:
	// ╭──────────────────────────────────────╮
	// │● online  ■ queue: 42                 │
	// │cpu ████░░░░                          │
	// │ready                                 │
	// ╰──────────────────────────────────────╯
}

// Stacked effects.
// Effects apply in order, left to right — each sees the output of the previous.
func ExampleScreenEffect_multiple() {
	// example:
	tree := VBox(
		HBox(
			Text("● red").FG(Red),
			Text("  "),
			Text("● green").FG(Green),
			Text("  "),
			Text("● blue").FG(Blue),
		),
		HBox(
			Text("████████").FG(RGB(255, 200, 50)),
			Text("████████").FG(RGB(50, 200, 255)),
		),
		ScreenEffect(
			SEDesaturate().Strength(0.5),
			SETint(Hex(0xFF6600)).Strength(0.1),
		),
	)
	// :example

	renderWithEffects("ScreenEffect_multiple", tree, 40, 2)
	// Output:
	// ● red  ● green  ● blue
	// ████████████████
}

// Conditional effect.
// Wrap in If() to toggle reactively based on state.
func ExampleScreenEffect_conditional() {
	// example:
	dimmed := true
	tree := VBox(
		HBox(
			Text("● status").FG(Green),
			Text("  "),
			Text("████").FG(Cyan),
			Text("░░░░").FG(RGB(80, 80, 80)),
		),
		If(&dimmed).Then(ScreenEffect(SEDim())),
	)
	// :example

	renderWithEffects("ScreenEffect_conditional", tree, 30, 1)
	// Output: ● status  ████░░░░
}

// Per-cell transform.
// The fragment shader equivalent — define per-cell logic, iteration is handled.
func ExampleEachCell() {
	// example:
	tree := VBox(
		HBox(Text("row 0 ").FG(Cyan), Text("████").FG(RGB(200, 100, 50))),
		HBox(Text("row 1 ").FG(Green), Text("████").FG(RGB(50, 100, 200))),
		HBox(Text("row 2 ").FG(Yellow), Text("████").FG(RGB(200, 50, 100))),
		HBox(Text("row 3 ").FG(Magenta), Text("████").FG(RGB(100, 200, 50))),
		ScreenEffect(EachCell(func(x, y int, c Cell, ctx PostContext) Cell {
			if y%2 == 0 {
				c.Style.Attr = c.Style.Attr.With(AttrDim)
			}
			return c
		})),
	)
	// :example

	renderWithEffects("EachCell", tree, 30, 4)
	// Output:
	// row 0 ████
	// row 1 ████
	// row 2 ████
	// row 3 ████
}

// Blend mode wrapper.
// Snapshots the buffer, runs the effect, then blends the result back using a Photoshop-style mode.
func ExampleWithBlend() {
	// example:
	tree := VBox(
		HBox(
			Text("████").FG(RGB(255, 100, 50)),
			Text("████").FG(RGB(50, 255, 100)),
			Text("████").FG(RGB(50, 100, 255)),
		),
		Text("blended glow").FG(RGB(200, 200, 200)),
		ScreenEffect(WithBlend(BlendScreen, SEBloom())),
	)
	// :example

	renderWithEffects("WithBlend", tree, 30, 2)
	// Output:
	// ████████████
	// blended glow
}

// Quantize wrapper.
// Snap colours to step-size buckets after an effect runs. Use step=32 to cut output size.
func ExampleWithQuantize() {
	// example:
	tree := VBox(
		HBox(
			Text("████").FG(RGB(255, 100, 50)),
			Text("████").FG(RGB(50, 255, 100)),
			Text("████").FG(RGB(50, 100, 255)),
		),
		Text("quantized bloom").FG(RGB(200, 200, 200)),
		ScreenEffect(WithQuantize(32, SEBloom())),
	)
	// :example

	renderWithEffects("WithQuantize", tree, 30, 2)
	// Output:
	// ████████████
	// quantized bloom
}

// Manual colour blend.
// Blend combines two colours with a Photoshop-style mode — useful inside custom effects.
func ExampleBlend() {
	// example:
	base := RGB(100, 150, 200)
	top := RGB(255, 100, 50)
	result := Blend(base, top, BlendScreen)
	// :example

	tree := HBox(
		Text("█ base").FG(base),
		Text("  █ top").FG(top),
		Text("  █ result").FG(result),
	)
	renderAndPrint("Blend", tree, 30, 1)
	// Output: █ base  █ top  █ result
}

// Dim entire screen uniformly.
func ExampleSEDim() {
	// example:
	tree := VBox(
		VBox.Border(BorderRounded)(
			HBox(
				Text("● active").FG(Green),
				Text("  "),
				Text("████").FG(Cyan),
				Text("░░░░").FG(RGB(80, 80, 80)),
			),
			Text("everything is dimmed").FG(RGB(200, 196, 184)),
		),
		ScreenEffect(SEDim()),
	)
	// :example

	renderWithEffects("SEDim", tree, 40, 4)
	// Output:
	// ╭──────────────────────────────────────╮
	// │● active  ████░░░░                    │
	// │everything is dimmed                  │
	// ╰──────────────────────────────────────╯
}

// Warm colour grade.
func ExampleSETint() {
	// example:
	tree := VBox(
		VBox.Border(BorderRounded)(
			HBox(
				Text("● online").FG(Green),
				Text("  "),
				Text("■ queue: 42").FG(Yellow),
			),
			HBox(
				Text("████").FG(Cyan),
				Text("████").FG(RGB(100, 200, 255)),
				Text("░░░░").FG(RGB(80, 80, 80)),
			),
			Text("warm tinted").FG(RGB(200, 196, 184)),
		),
		ScreenEffect(SETint(Hex(0xFF6600)).Strength(0.15)),
	)
	// :example

	renderWithEffects("SETint", tree, 40, 5)
	// Output:
	// ╭──────────────────────────────────────╮
	// │● online  ■ queue: 42                 │
	// │████████░░░░                          │
	// │warm tinted                           │
	// ╰──────────────────────────────────────╯
}

// Tint that spares a focused panel.
func ExampleSETint_dodge() {
	// example:
	var panel NodeRef
	tree := VBox(
		Text("tinted background").FG(RGB(180, 180, 180)),
		VBox.Border(BorderRounded).NodeRef(&panel)(
			Text("untinted panel").FG(Green),
			Text("████████").FG(Cyan),
		),
		Text("also tinted").FG(Yellow),
		ScreenEffect(SETint(Hex(0x0066FF)).Strength(0.3).Dodge(&panel)),
	)
	// :example

	renderWithEffects("SETint_dodge", tree, 40, 5)
	// Output:
	// tinted background
	// ╭──────────────────────────────────────╮
	// │untinted panel                        │
	// │████████                              │
	// ╰──────────────────────────────────────╯
}

// Cinematic edge darkening.
func ExampleSEVignette() {
	// example:
	tree := VBox(
		VBox.Border(BorderRounded)(
			HBox(
				Text("● red").FG(Red),
				Text("  "),
				Text("● green").FG(Green),
				Text("  "),
				Text("● blue").FG(Blue),
			),
			HBox(
				Text("████").FG(RGB(255, 100, 50)),
				Text("████").FG(RGB(50, 255, 100)),
				Text("████").FG(RGB(50, 100, 255)),
				Text("████").FG(RGB(255, 255, 50)),
			),
			Text("edges darken toward black").FG(RGB(200, 196, 184)),
		),
		ScreenEffect(SEVignette().Strength(0.7)),
	)
	// :example

	renderWithEffects("SEVignette", tree, 40, 5)
	// Output:
	// ╭──────────────────────────────────────╮
	// │● red  ● green  ● blue                │
	// │████████████████                      │
	// │edges darken toward black             │
	// ╰──────────────────────────────────────╯
}

// Vignette centred on a panel.
func ExampleSEVignette_focus() {
	// example:
	var panel NodeRef
	tree := VBox(
		Text("dim edges").FG(RGB(180, 180, 180)),
		VBox.Border(BorderRounded).NodeRef(&panel)(
			Text("focus centre").FG(Green),
			Text("████████").FG(Cyan),
		),
		Text("dim edges").FG(RGB(180, 180, 180)),
		ScreenEffect(SEVignette().Focus(&panel)),
	)
	// :example

	renderWithEffects("SEVignette_focus", tree, 40, 6)
	// Output:
	// dim edges
	// ╭──────────────────────────────────────╮
	// │focus centre                          │
	// │████████                              │
	// ╰──────────────────────────────────────╯
	// dim edges
}

// Vignette that skips a panel.
func ExampleSEVignette_dodge() {
	// example:
	var panel NodeRef
	tree := VBox(
		Text("darkened by vignette").FG(RGB(200, 200, 200)),
		VBox.Border(BorderRounded).NodeRef(&panel)(
			Text("exempt from vignette").FG(Green),
			Text("████████").FG(Cyan),
		),
		Text("also darkened").FG(RGB(200, 200, 200)),
		ScreenEffect(SEVignette().Dodge(&panel)),
	)
	// :example

	renderWithEffects("SEVignette_dodge", tree, 40, 6)
	// Output:
	// darkened by vignette
	// ╭──────────────────────────────────────╮
	// │exempt from vignette                  │
	// │████████                              │
	// ╰──────────────────────────────────────╯
	// also darkened
}

// Smooth vignette without quantization.
func ExampleSEVignette_smooth() {
	// example:
	tree := VBox(
		VBox.Border(BorderRounded)(
			HBox(
				Text("████").FG(RGB(255, 100, 50)),
				Text("████").FG(RGB(50, 255, 100)),
				Text("████").FG(RGB(50, 100, 255)),
			),
			Text("smooth gradient").FG(RGB(200, 196, 184)),
		),
		ScreenEffect(SEVignette().Smooth().Strength(0.6)),
	)
	// :example

	renderWithEffects("SEVignette_smooth", tree, 40, 4)
	// Output:
	// ╭──────────────────────────────────────╮
	// │████████████                          │
	// │smooth gradient                       │
	// ╰──────────────────────────────────────╯
}

// Wash out colour.
func ExampleSEDesaturate() {
	// example:
	tree := VBox(
		VBox.Border(BorderRounded)(
			HBox(
				Text("● red").FG(Red),
				Text("  "),
				Text("● green").FG(Green),
				Text("  "),
				Text("● blue").FG(Blue),
			),
			HBox(
				Text("████").FG(RGB(255, 100, 50)),
				Text("████").FG(RGB(50, 255, 100)),
				Text("████").FG(RGB(50, 100, 255)),
			),
			Text("colours washed out").FG(RGB(200, 196, 184)),
		),
		ScreenEffect(SEDesaturate().Strength(0.8)),
	)
	// :example

	renderWithEffects("SEDesaturate", tree, 40, 5)
	// Output:
	// ╭──────────────────────────────────────╮
	// │● red  ● green  ● blue                │
	// │████████████                          │
	// │colours washed out                    │
	// ╰──────────────────────────────────────╯
}

// Colour spotlight — grey world, one panel in colour.
func ExampleSEDesaturate_dodge() {
	// example:
	var panel NodeRef
	tree := VBox(
		HBox(
			Text("████").FG(Red),
			Text("████").FG(Yellow),
		),
		VBox.Border(BorderRounded).NodeRef(&panel)(
			Text("colour preserved").FG(Green),
			Text("████████").FG(Cyan),
		),
		HBox(
			Text("████").FG(Magenta),
			Text("████").FG(Blue),
		),
		ScreenEffect(SEDesaturate().Dodge(&panel)),
	)
	// :example

	renderWithEffects("SEDesaturate_dodge", tree, 40, 6)
	// Output:
	// ████████
	// ╭──────────────────────────────────────╮
	// │colour preserved                      │
	// │████████                              │
	// ╰──────────────────────────────────────╯
	// ████████
}

// Punch up contrast.
func ExampleSEContrast() {
	// example:
	tree := VBox(
		VBox.Border(BorderRounded)(
			HBox(
				Text("████").FG(RGB(180, 100, 80)),
				Text("████").FG(RGB(80, 180, 100)),
				Text("████").FG(RGB(100, 80, 180)),
			),
			Text("midtones pushed to extremes").FG(RGB(150, 150, 150)),
		),
		ScreenEffect(SEContrast().Strength(2.0)),
	)
	// :example

	renderWithEffects("SEContrast", tree, 40, 4)
	// Output:
	// ╭──────────────────────────────────────╮
	// │████████████                          │
	// │midtones pushed to extremes           │
	// ╰──────────────────────────────────────╯
}

// Contrast that spares a panel.
func ExampleSEContrast_dodge() {
	// example:
	var panel NodeRef
	tree := VBox(
		Text("high contrast").FG(RGB(150, 150, 150)),
		VBox.Border(BorderRounded).NodeRef(&panel)(
			Text("normal contrast").FG(RGB(150, 150, 150)),
			Text("████████").FG(RGB(100, 180, 100)),
		),
		Text("high contrast").FG(RGB(150, 150, 150)),
		ScreenEffect(SEContrast().Strength(2.0).Dodge(&panel)),
	)
	// :example

	renderWithEffects("SEContrast_dodge", tree, 40, 6)
	// Output:
	// high contrast
	// ╭──────────────────────────────────────╮
	// │normal contrast                       │
	// │████████                              │
	// ╰──────────────────────────────────────╯
	// high contrast
}

// Dim everything outside a node.
func ExampleSEFocusDim() {
	// example:
	var panel NodeRef
	tree := VBox(
		HBox(
			Text("████").FG(Red),
			Text(" dimmed ").FG(RGB(200, 200, 200)),
			Text("████").FG(Yellow),
		),
		VBox.Border(BorderRounded).NodeRef(&panel)(
			Text("focused panel").FG(Green),
			Text("████████").FG(Cyan),
		),
		HBox(
			Text("████").FG(Magenta),
			Text(" dimmed ").FG(RGB(200, 200, 200)),
			Text("████").FG(Blue),
		),
		ScreenEffect(SEFocusDim(&panel)),
	)
	// :example

	renderWithEffects("SEFocusDim", tree, 40, 6)
	// Output:
	// ████ dimmed ████
	// ╭──────────────────────────────────────╮
	// │focused panel                         │
	// │████████                              │
	// ╰──────────────────────────────────────╯
	// ████ dimmed ████
}

// Remap colours through a three-stop gradient.
func ExampleSEGradientMap() {
	// example:
	tree := VBox(
		VBox.Border(BorderRounded)(
			HBox(
				Text("████").FG(RGB(40, 40, 40)),
				Text("████").FG(RGB(128, 128, 128)),
				Text("████").FG(RGB(240, 240, 240)),
			),
			Text("shadows→blue  mids→teal  highs→mint").FG(RGB(200, 196, 184)),
		),
		ScreenEffect(SEGradientMap(
			RGB(0, 0, 50),
			RGB(0, 128, 128),
			RGB(200, 255, 200),
		)),
	)
	// :example

	renderWithEffects("SEGradientMap", tree, 40, 4)
	// Output:
	// ╭──────────────────────────────────────╮
	// │████████████                          │
	// │shadows→blue  mids→teal  highs→mint   │
	// ╰──────────────────────────────────────╯
}

// Directional drop shadow behind a panel.
func ExampleSEDropShadow() {
	// example:
	var panel NodeRef
	tree := VBox(
		Text(""),
		HBox(
			Text("   "),
			VBox.Border(BorderRounded).NodeRef(&panel)(
				Text("shadowed panel").FG(Green),
				Text("████████").FG(Cyan),
			),
		),
		Text(""),
		Text(""),
		ScreenEffect(SEDropShadow().Focus(&panel)),
	)
	// :example

	renderWithEffects("SEDropShadow", tree, 40, 7)
	// Output:
	//
	//    ╭───────────────────────────────────╮
	//    │shadowed panel                     │
	//    │████████                           │
	//    ╰───────────────────────────────────╯
}

// Symmetric glow — offset(0,0) centres the shadow source on the panel.
func ExampleSEDropShadow_glow() {
	// example:
	var panel NodeRef
	tree := VBox(
		Text(""),
		HBox(
			Text("   "),
			VBox.Border(BorderRounded).NodeRef(&panel)(
				Text("glowing panel").FG(RGB(255, 200, 50)),
				Text("████████").FG(RGB(255, 150, 50)),
			),
		),
		Text(""),
		Text(""),
		ScreenEffect(SEDropShadow().Focus(&panel).Offset(0, 0).Strength(0.4).Radius(12)),
	)
	// :example

	renderWithEffects("SEDropShadow_glow", tree, 40, 7)
	// Output:
	//
	//    ╭───────────────────────────────────╮
	//    │glowing panel                      │
	//    │████████                           │
	//    ╰───────────────────────────────────╯
}

// Colour-sampling glow that reads the panel's edge colours and spills them outward.
func ExampleSEGlow() {
	// example:
	var panel NodeRef
	tree := VBox(
		Text(""),
		HBox(
			Text("   "),
			VBox.Border(BorderRounded).NodeRef(&panel)(
				Text("glow source").FG(RGB(100, 255, 100)),
				Text("████████").FG(RGB(50, 200, 255)),
			),
		),
		Text(""),
		Text(""),
		ScreenEffect(SEGlow().Focus(&panel)),
	)
	// :example

	renderWithEffects("SEGlow", tree, 40, 7)
	// Output:
	//
	//    ╭───────────────────────────────────╮
	//    │glow source                        │
	//    │████████                           │
	//    ╰───────────────────────────────────╯
}

// Bloom around bright cells.
func ExampleSEBloom() {
	// example:
	tree := VBox(
		VBox.Border(BorderRounded)(
			HBox(
				Text("████").FG(RGB(255, 255, 200)),
				Text("    "),
				Text("████").FG(RGB(200, 255, 255)),
			),
			Text("dim text").FG(RGB(60, 60, 60)),
			Text("bright cells glow outward").FG(RGB(200, 196, 184)),
		),
		ScreenEffect(SEBloom().Threshold(0.6).Strength(0.3)),
	)
	// :example

	renderWithEffects("SEBloom", tree, 40, 5)
	// Output:
	// ╭──────────────────────────────────────╮
	// │████    ████                          │
	// │dim text                              │
	// │bright cells glow outward             │
	// ╰──────────────────────────────────────╯
}

// Bloom constrained to a panel.
func ExampleSEBloom_focus() {
	// example:
	var panel NodeRef
	tree := VBox(
		HBox(
			Text("████").FG(RGB(255, 255, 200)),
			Text(" no bloom here "),
		),
		VBox.Border(BorderRounded).NodeRef(&panel)(
			Text("████").FG(RGB(255, 255, 200)),
			Text("bloom only here").FG(RGB(200, 196, 184)),
		),
		ScreenEffect(SEBloom().Focus(&panel)),
	)
	// :example

	renderWithEffects("SEBloom_focus", tree, 40, 5)
	// Output:
	// ████ no bloom here
	// ╭──────────────────────────────────────╮
	// │████                                  │
	// │bloom only here                       │
	// ╰──────────────────────────────────────╯
}

// Green phosphor monochrome.
func ExampleSEMonochrome() {
	// example:
	tree := VBox(
		VBox.Border(BorderRounded)(
			HBox(
				Text("● red").FG(Red),
				Text("  "),
				Text("● green").FG(Green),
				Text("  "),
				Text("● blue").FG(Blue),
			),
			HBox(
				Text("████").FG(RGB(255, 100, 50)),
				Text("████").FG(RGB(50, 255, 100)),
				Text("████").FG(RGB(50, 100, 255)),
			),
			Text("all mapped to green phosphor").FG(RGB(200, 196, 184)),
		),
		ScreenEffect(SEMonochrome(RGB(0, 255, 80))),
	)
	// :example

	renderWithEffects("SEMonochrome", tree, 40, 5)
	// Output:
	// ╭──────────────────────────────────────╮
	// │● red  ● green  ● blue                │
	// │████████████                          │
	// │all mapped to green phosphor          │
	// ╰──────────────────────────────────────╯
}

// Monochrome with a colour spotlight.
func ExampleSEMonochrome_dodge() {
	// example:
	var panel NodeRef
	tree := VBox(
		HBox(
			Text("████").FG(Red),
			Text("████").FG(Yellow),
			Text(" monochrome "),
		),
		VBox.Border(BorderRounded).NodeRef(&panel)(
			Text("full colour").FG(Green),
			Text("████████").FG(Cyan),
		),
		HBox(
			Text("████").FG(Magenta),
			Text("████").FG(Blue),
			Text(" monochrome "),
		),
		ScreenEffect(SEMonochrome(RGB(0, 255, 80)).Dodge(&panel)),
	)
	// :example

	renderWithEffects("SEMonochrome_dodge", tree, 40, 6)
	// Output:
	// ████████ monochrome
	// ╭──────────────────────────────────────╮
	// │full colour                           │
	// │████████                              │
	// ╰──────────────────────────────────────╯
	// ████████ monochrome
}

// Snap colours to 32-level steps.
// Reduces escape output for animated effects with negligible visible banding.
func ExampleSEQuantize() {
	// example:
	tree := VBox(
		VBox.Border(BorderRounded)(
			HBox(
				Text("████").FG(RGB(255, 100, 50)),
				Text("████").FG(RGB(50, 255, 100)),
				Text("████").FG(RGB(50, 100, 255)),
			),
			Text("colours snapped to 32 steps").FG(RGB(200, 196, 184)),
		),
		ScreenEffect(SEQuantize(32)),
	)
	// :example

	renderWithEffects("SEQuantize", tree, 40, 4)
	// Output:
	// ╭──────────────────────────────────────╮
	// │████████████                          │
	// │colours snapped to 32 steps           │
	// ╰──────────────────────────────────────╯
}

// First-match-wins conditional.
// Evaluates cases top-to-bottom; the first true case renders.
func ExampleMatch() {
	// example:
	status := "error"
	tree := Match(&status,
		Eq("ok", Text("all clear")),
		Eq("warn", Text("warning")),
		Eq("error", Text("failure")),
	).Default(Text("unknown"))
	// :example

	renderAndPrint("Match", tree, 20, 1)
	// Output: failure
}

// Ordered thresholds.
// Order matters — the first matching case wins, so check the highest threshold first.
func ExampleMatch_ordered() {
	// example:
	cpu := 85.0
	tree := Match(&cpu,
		Gt(90.0, Text("CRITICAL")),
		Gt(70.0, Text("WARNING")),
		Lte(70.0, Text("OK")),
	)
	// :example

	renderAndPrint("Match_ordered", tree, 20, 1)
	// Output: WARNING
}

// Predicate matching.
// When accepts a function for cases that need custom logic.
func ExampleMatch_where() {
	// example:
	query := "hello world"
	tree := Match(&query,
		Eq("", Text("type to search")),
		Where(func(q string) bool { return len(q) < 3 }, Text("keep typing...")),
		Where(func(q string) bool { return len(q) >= 3 }, Text("searching")),
	)
	// :example

	renderAndPrint("Match_where", tree, 20, 1)
	// Output: searching
}

// Scrollable container.
// ScrollView wraps children in a scrollable layer. Content renders into an offscreen buffer and is blitted each frame.
func ExampleScrollViewFn() {
	// example:
	ScrollView.Grow(1)(
		Text("line one"),
		Text("line two"),
		Text("line three"),
	)
	// :example

	tree := VBox(
		Text("line one"),
		Text("line two"),
		Text("line three"),
	)
	renderAndPrint("ScrollViewFn", tree, 20, 3)
	// Output:
	// line one
	// line two
	// line three
}

// Word-wrapped text.
// TextBlock renders multi-line text inline with automatic word wrapping. Unlike TextView, it has no scroll layer.
func ExampleTextBlockC() {
	// example:
	tree := VBox(
		TextBlock("hello world this is a long line that wraps"),
		Text("---"),
	)
	// :example

	renderAndPrint("TextBlockC", tree, 20, 4)
	// Output:
	// hello world this is
	// a long line that
	// wraps
	// ---
}

// Scrollable text viewer.
// TextView wraps text with character-level wrapping and provides a scrollable layer. Bind keys to navigate.
func ExampleTextViewC() {
	// example:
	content := "first\nsecond\nthird"
	TextView(&content).Grow(1).BindNav("j", "k")
	// :example

	tree := VBox(
		Text("first"),
		Text("second"),
		Text("third"),
	)
	renderAndPrint("TextViewC", tree, 20, 3)
	// Output:
	// first
	// second
	// third
}

// Tree structure.
// TreeNode builds a hierarchical tree. Set Expanded to control which branches are open.
func ExampleTreeNode() {
	// example:
	root := &TreeNode{
		Label:    "project",
		Expanded: true,
		Children: []*TreeNode{
			{Label: "cmd", Expanded: true, Children: []*TreeNode{
				{Label: "main.go"},
			}},
			{Label: "README.md"},
		},
	}
	tree := TreeView{Root: root, ShowRoot: true, Indent: 2, ShowLines: true}
	// :example

	renderAndPrint("TreeNode", tree, 20, 4)
	// Output:
	// ▼ project
	// │ ▼ cmd
	// │     main.go
	//     README.md
}

// Labeled form field.
// Field pairs a label with a control. Used as arguments to Form().
func ExampleFormField() {
	// example:
	var name string
	tree := Form(
		Field("Name", Input(&name).Placeholder("enter name")),
	)
	// :example

	renderAndPrint("FormField", tree, 30, 1)
	// Output: ▸Name: enter name
}


// Animation tween.
// Animate interpolates toward a target value over time. Configure duration and easing, then apply to any numeric property.
func ExampleAnimateFn() {
	// example:
	smooth := Animate.Duration(300 * time.Millisecond).Ease(EaseOutCubic)

	var h int16 = 5
	tree := VBox.Height(smooth(&h)).Border(BorderRounded)(
		Text("animated"),
	)
	// :example

	renderAndPrint("AnimateFn", tree, 20, 7)
	// Output:
	// ╭──────────────────╮
	// │animated          │
	// │                  │
	// │                  │
	// ╰──────────────────╯
}

// Colour tween.
// Animate works on Color pointers. The colour lerps through RGB space toward the target.
func ExampleAnimateFn_color() {
	// example:
	fg := Red
	tree := VBox.Border(BorderRounded)(
		Text("status").FG(Animate.Duration(500 * time.Millisecond)(&fg)),
	)
	// :example

	renderAndPrint("AnimateFn_color", tree, 20, 3)
	// Output:
	// ╭──────────────────╮
	// │status            │
	// ╰──────────────────╯
}

// Conditional target.
// Wrap a conditional in Animate — the tween triggers whenever the condition changes.
func ExampleAnimateFn_conditional() {
	// example:
	expanded := true
	smooth := Animate.Duration(300 * time.Millisecond).Ease(EaseOutCubic)

	tree := VBox.Height(smooth(If(&expanded).Then(int16(5)).Else(int16(1)))).Border(BorderRounded)(
		Text("content"),
	)
	// :example

	renderAndPrint("AnimateFn_conditional", tree, 20, 7)
	// Output:
	// ╭──────────────────╮
	// │content           │
	// │                  │
	// │                  │
	// ╰──────────────────╯
}

// Completion callback.
// OnComplete fires once when the animation reaches its target value.
func ExampleAnimateFn_onComplete() {
	// example:
	done := false
	tree := VBox.Height(
		Animate.Duration(200 * time.Millisecond).OnComplete(func() { done = true })(int16(3)),
	).Border(BorderRounded)(
		Text("animating"),
	)
	// :example
	_ = done

	renderAndPrint("AnimateFn_onComplete", tree, 20, 5)
	// Output:
	// ╭──────────────────╮
	// │animating         │
	// ╰──────────────────╯
}

// Enter and exit transitions.
// Wrap any property value with In for the entry and Out for the exit. Pass a
// tween to either side to animate it; pass a plain value to snap. Both sides
// are tied to the surrounding conditional, and Glyph keeps the outgoing branch
// on screen until its Out tween completes.
func ExampleIn() {
	// example:
	visible := true
	tree := If(&visible).Then(
		VBox.Border(BorderRounded).
			Width(
				In(20).
				Out(Animate(0)),
			)(
				Text("hello"),
			),
	)
	// :example

	renderAndPrint("In", tree, 20, 3)
	// Output:
	// ╭──────────────────╮
	// │hello             │
	// ╰──────────────────╯
}

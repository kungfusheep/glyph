package tui

import (
	"fmt"
	"strings"
	"testing"
)

// =============================================================================
// Scenario 1: Filterable List with Dynamic Items
// =============================================================================

type Task struct {
	ID        int
	Title     string
	Completed bool
	Priority  string // "low", "medium", "high"
}

var taskListState = struct {
	Tasks      []Task
	Filter     string // "all", "active", "completed"
	ShowCount  bool
	SelectedID int
}{
	Tasks: []Task{
		{1, "Write tests", false, "high"},
		{2, "Review PR", true, "medium"},
		{3, "Update docs", false, "low"},
		{4, "Fix bug", false, "high"},
		{5, "Deploy", true, "medium"},
	},
	Filter:     "all",
	ShowCount:  true,
	SelectedID: 1,
}

func filterTasks() []Task {
	var result []Task
	for _, t := range taskListState.Tasks {
		switch taskListState.Filter {
		case "active":
			if !t.Completed {
				result = append(result, t)
			}
		case "completed":
			if t.Completed {
				result = append(result, t)
			}
		default:
			result = append(result, t)
		}
	}
	return result
}

var taskListUI = DCol{
	Gap: 1,
	Children: []any{
		// Header with filter buttons
		DRow{Gap: 1, Children: []any{
			DText{Content: "Tasks", Bold: true},
			DButton{Label: "All", OnClick: func() { taskListState.Filter = "all" }},
			DButton{Label: "Active", OnClick: func() { taskListState.Filter = "active" }},
			DButton{Label: "Done", OnClick: func() { taskListState.Filter = "completed" }},
		}},

		// Conditional count display
		DIf(&taskListState.ShowCount,
			DText{Content: func() string {
				return fmt.Sprintf("Showing %d tasks", len(filterTasks()))
			}},
		),

		// Dynamic task list
		DForEach(filterTasks, func(t *Task) any {
			return DRow{Gap: 1, Children: []any{
				DCheckbox{
					Checked: &t.Completed,
					Label:   t.Title,
				},
				DIf(func() bool { return t.Priority == "high" },
					DText{Content: "[!]", Bold: true},
				),
			}}
		}),
	},
}

func TestScenarioFilterableList(t *testing.T) {
	// Test "all" filter
	taskListState.Filter = "all"
	frame := Execute(taskListUI)
	frame.Layout(60, 20)
	buf := NewBuffer(60, 20)
	frame.Render(buf)
	output := buf.String()
	t.Logf("All tasks:\n%s", output)

	if !containsString(output, "Write tests") {
		t.Error("Should show active task")
	}
	if !containsString(output, "Review PR") {
		t.Error("Should show completed task")
	}
	if !containsString(output, "Showing 5 tasks") {
		t.Error("Should show count")
	}

	// Test "active" filter
	taskListState.Filter = "active"
	frame = Execute(taskListUI)
	frame.Layout(60, 20)
	buf.Clear()
	frame.Render(buf)
	output = buf.String()
	t.Logf("Active tasks:\n%s", output)

	if !containsString(output, "Showing 3 tasks") {
		t.Error("Should show 3 active tasks")
	}

	// Reset
	taskListState.Filter = "all"
}

// =============================================================================
// Scenario 2: Form with Validation
// =============================================================================

var formState = struct {
	Username string
	Email    string
	Password string
	Errors   map[string]string
	Touched  map[string]bool
}{
	Errors:  make(map[string]string),
	Touched: make(map[string]bool),
}

func validateForm() {
	formState.Errors = make(map[string]string)
	if len(formState.Username) < 3 {
		formState.Errors["username"] = "Username must be at least 3 chars"
	}
	if !containsString(formState.Email, "@") {
		formState.Errors["email"] = "Invalid email"
	}
	if len(formState.Password) < 8 {
		formState.Errors["password"] = "Password must be at least 8 chars"
	}
}

var formUI = DCol{
	Gap: 1,
	Children: []any{
		DText{Content: "Registration", Bold: true},

		// Username field with error
		DCol{Children: []any{
			DRow{Children: []any{
				DText{Content: "Username: "},
				DInput{
					Value: &formState.Username,
					Width: 20,
					OnChange: func(v string) {
						formState.Touched["username"] = true
						validateForm()
					},
				},
			}},
			DIf(func() bool {
				return formState.Touched["username"] && formState.Errors["username"] != ""
			},
				DText{Content: func() string { return "  " + formState.Errors["username"] }},
			),
		}},

		// Email field with error
		DCol{Children: []any{
			DRow{Children: []any{
				DText{Content: "Email:    "},
				DInput{
					Value: &formState.Email,
					Width: 20,
					OnChange: func(v string) {
						formState.Touched["email"] = true
						validateForm()
					},
				},
			}},
			DIf(func() bool {
				return formState.Touched["email"] && formState.Errors["email"] != ""
			},
				DText{Content: func() string { return "  " + formState.Errors["email"] }},
			),
		}},

		// Submit button - disabled if errors
		DButton{
			Label: "Submit",
			Disabled: func() bool {
				return len(formState.Errors) > 0
			},
			OnClick: func() {
				// Submit logic
			},
		},
	},
}

func TestScenarioFormValidation(t *testing.T) {
	// Reset state
	formState.Username = ""
	formState.Email = ""
	formState.Password = ""
	formState.Errors = make(map[string]string)
	formState.Touched = make(map[string]bool)

	frame := Execute(formUI)
	frame.Layout(60, 20)
	buf := NewBuffer(60, 20)
	frame.Render(buf)
	output := buf.String()
	t.Logf("Initial form:\n%s", output)

	// Simulate touching username with invalid value
	formState.Username = "ab"
	formState.Touched["username"] = true
	validateForm()

	frame = Execute(formUI)
	frame.Layout(60, 20)
	buf.Clear()
	frame.Render(buf)
	output = buf.String()
	t.Logf("With validation error:\n%s", output)

	if !containsString(output, "at least 3 chars") {
		t.Error("Should show username error")
	}
}

// =============================================================================
// Scenario 3: Dashboard with Multiple Dynamic Panels
// =============================================================================

type SystemMetrics struct {
	CPUCores    []int // usage per core
	MemoryUsed  int
	MemoryTotal int
	DiskUsed    int
	DiskTotal   int
	NetworkIn   float64
	NetworkOut  float64
	Alerts      []string
}

var metricsState = SystemMetrics{
	CPUCores:    []int{45, 67, 23, 89},
	MemoryUsed:  8192,
	MemoryTotal: 16384,
	DiskUsed:    450,
	DiskTotal:   1000,
	NetworkIn:   1.2,
	NetworkOut:  0.8,
	Alerts:      []string{"High CPU on core 3", "Disk 45% full"},
}

var dashboardUI = DCol{
	Gap: 1,
	Children: []any{
		DText{Content: "System Dashboard", Bold: true},

		// CPU section with per-core bars
		DCol{Children: []any{
			DText{Content: "CPU Usage:"},
			DForEach(&metricsState.CPUCores, func(usage *int) any {
				return DProgress{Value: usage, Width: 30}
			}),
		}},

		// Memory bar
		DRow{Children: []any{
			DText{Content: "Memory: "},
			DProgress{
				Value: func() int {
					return metricsState.MemoryUsed * 100 / metricsState.MemoryTotal
				},
				Width: 30,
			},
			DText{Content: func() string {
				return fmt.Sprintf(" %dMB/%dMB", metricsState.MemoryUsed, metricsState.MemoryTotal)
			}},
		}},

		// Network stats
		DRow{Gap: 2, Children: []any{
			DText{Content: func() string {
				return fmt.Sprintf("↑ %.1f MB/s", metricsState.NetworkOut)
			}},
			DText{Content: func() string {
				return fmt.Sprintf("↓ %.1f MB/s", metricsState.NetworkIn)
			}},
		}},

		// Alerts section - only shown if there are alerts
		DIf(func() bool { return len(metricsState.Alerts) > 0 },
			DCol{Children: []any{
				DText{Content: "Alerts:", Bold: true},
				DForEach(&metricsState.Alerts, func(alert *string) any {
					return DText{Content: func() string { return "  ⚠ " + *alert }}
				}),
			}},
		),
	},
}

func TestScenarioDashboard(t *testing.T) {
	frame := Execute(dashboardUI)
	frame.Layout(60, 25)
	buf := NewBuffer(60, 25)
	frame.Render(buf)
	output := buf.String()
	t.Logf("Dashboard:\n%s", output)

	// Verify CPU cores
	if !containsString(output, "CPU Usage") {
		t.Error("Should show CPU section")
	}

	// Verify alerts
	if !containsString(output, "High CPU") {
		t.Error("Should show alerts")
	}

	// Test with no alerts
	metricsState.Alerts = nil
	frame = Execute(dashboardUI)
	frame.Layout(60, 25)
	buf.Clear()
	frame.Render(buf)
	output = buf.String()
	t.Logf("Dashboard (no alerts):\n%s", output)

	if containsString(output, "Alerts:") {
		t.Error("Should hide alerts section when empty")
	}

	// Reset
	metricsState.Alerts = []string{"High CPU on core 3"}
}

// =============================================================================
// Scenario 4: Tabbed Interface with State Per Tab
// =============================================================================

type TabState struct {
	ActiveTab string
	Tabs      map[string]any // tab name -> tab-specific state
}

var tabState = TabState{
	ActiveTab: "home",
	Tabs: map[string]any{
		"home":     &struct{ Message string }{Message: "Welcome!"},
		"settings": &struct{ DarkMode bool }{DarkMode: false},
		"profile":  &struct{ Name string }{Name: "User"},
	},
}

func tabButton(name, label string) DButton {
	return DButton{
		Label: func() string {
			if tabState.ActiveTab == name {
				return "[" + label + "]"
			}
			return " " + label + " "
		},
		OnClick: func() {
			tabState.ActiveTab = name
		},
	}
}

var tabbedUI = DCol{
	Gap: 1,
	Children: []any{
		// Tab bar
		DRow{Children: []any{
			tabButton("home", "Home"),
			tabButton("settings", "Settings"),
			tabButton("profile", "Profile"),
		}},

		// Tab content
		DSwitch(&tabState.ActiveTab,
			DCase("home", DCol{Children: []any{
				DText{Content: "Home Tab", Bold: true},
				DText{Content: func() string {
					state := tabState.Tabs["home"].(*struct{ Message string })
					return state.Message
				}},
			}}),
			DCase("settings", DCol{Children: []any{
				DText{Content: "Settings Tab", Bold: true},
				DCheckbox{
					Label: "Dark Mode",
					Checked: func() *bool {
						state := tabState.Tabs["settings"].(*struct{ DarkMode bool })
						return &state.DarkMode
					}(),
				},
			}}),
			DCase("profile", DCol{Children: []any{
				DText{Content: "Profile Tab", Bold: true},
				DRow{Children: []any{
					DText{Content: "Name: "},
					DInput{
						Value: func() *string {
							state := tabState.Tabs["profile"].(*struct{ Name string })
							return &state.Name
						}(),
						Width: 20,
					},
				}},
			}}),
		),
	},
}

func TestScenarioTabbedInterface(t *testing.T) {
	tabState.ActiveTab = "home"

	frame := Execute(tabbedUI)
	frame.Layout(60, 15)
	buf := NewBuffer(60, 15)
	frame.Render(buf)
	output := buf.String()
	t.Logf("Home tab:\n%s", output)

	if !containsString(output, "[Home]") {
		t.Error("Home should be active")
	}
	if !containsString(output, "Welcome!") {
		t.Error("Should show home content")
	}

	// Switch to settings
	tabState.ActiveTab = "settings"
	frame = Execute(tabbedUI)
	frame.Layout(60, 15)
	buf.Clear()
	frame.Render(buf)
	output = buf.String()
	t.Logf("Settings tab:\n%s", output)

	if !containsString(output, "[Settings]") {
		t.Error("Settings should be active")
	}
	if !containsString(output, "Dark Mode") {
		t.Error("Should show settings content")
	}

	// Reset
	tabState.ActiveTab = "home"
}

// =============================================================================
// Scenario 5: Benchmark the Scenarios
// =============================================================================

func BenchmarkScenarioTaskList(b *testing.B) {
	taskListState.Filter = "all"
	frame := NewDFrame()
	buf := NewBuffer(60, 20)

	// Warm up
	ExecuteInto(frame, taskListUI)
	frame.Layout(60, 20)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteInto(frame, taskListUI)
		frame.Layout(60, 20)
		buf.Clear()
		frame.Render(buf)
	}
}

func BenchmarkScenarioDashboard(b *testing.B) {
	metricsState.Alerts = []string{"Alert 1", "Alert 2"}
	frame := NewDFrame()
	buf := NewBuffer(60, 25)

	// Warm up
	ExecuteInto(frame, dashboardUI)
	frame.Layout(60, 25)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteInto(frame, dashboardUI)
		frame.Layout(60, 25)
		buf.Clear()
		frame.Render(buf)
	}
}

// =============================================================================
// Scenario 6: Scrollable List with Viewport Culling
// =============================================================================

func TestScenarioScrollViewportCulling(t *testing.T) {
	// Create 100 items, but only show 5 at a time
	items := make([]any, 100)
	for i := range items {
		items[i] = DText{Content: fmt.Sprintf("Item %03d", i)}
	}

	scrollOffset := 0

	scrollUI := DScroll{
		Children:   items,
		Height:     5,
		ItemHeight: 1,
		Offset:     &scrollOffset,
	}

	// Test 1: Initial render (offset=0)
	frame := Execute(scrollUI)
	frame.Layout(40, 10)
	buf := NewBuffer(40, 10)
	frame.Render(buf)
	output := buf.String()
	t.Logf("Scroll at offset 0:\n%s", output)

	// Should show items 0-4
	if !containsString(output, "Item 000") {
		t.Error("Should show Item 000")
	}
	if !containsString(output, "Item 004") {
		t.Error("Should show Item 004")
	}
	if containsString(output, "Item 005") {
		t.Error("Should NOT show Item 005 (out of viewport)")
	}

	// Verify node count - should only have visible items + scroll container
	// With viewport culling, only 5 items should be in the tree (not 100)
	nodeCount := 0
	for _, n := range frame.nodes {
		if n.Type == "text" {
			nodeCount++
		}
	}
	if nodeCount != 5 {
		t.Errorf("Expected 5 text nodes (viewport culling), got %d", nodeCount)
	}

	// Test 2: Scroll to middle
	scrollOffset = 50
	frame = Execute(scrollUI)
	frame.Layout(40, 10)
	buf.Clear()
	frame.Render(buf)
	output = buf.String()
	t.Logf("Scroll at offset 50:\n%s", output)

	// Should show items 50-54
	if !containsString(output, "Item 050") {
		t.Error("Should show Item 050")
	}
	if !containsString(output, "Item 054") {
		t.Error("Should show Item 054")
	}
	if containsString(output, "Item 049") {
		t.Error("Should NOT show Item 049 (scrolled past)")
	}

	// Test 3: Scroll near end
	scrollOffset = 98
	frame = Execute(scrollUI)
	frame.Layout(40, 10)
	buf.Clear()
	frame.Render(buf)
	output = buf.String()
	t.Logf("Scroll at offset 98:\n%s", output)

	// Should show items 98-99 (only 2 remaining)
	if !containsString(output, "Item 098") {
		t.Error("Should show Item 098")
	}
	if !containsString(output, "Item 099") {
		t.Error("Should show Item 099")
	}
}

func TestScrollStateHelper(t *testing.T) {
	state := DScrollState{
		Offset:       0,
		ItemCount:    100,
		ViewportRows: 10,
	}

	// Scroll down
	state.ScrollBy(5)
	if state.Offset != 5 {
		t.Errorf("Expected offset 5, got %d", state.Offset)
	}

	// Scroll to specific position
	state.ScrollTo(50)
	if state.Offset != 50 {
		t.Errorf("Expected offset 50, got %d", state.Offset)
	}

	// Test clamping at end
	state.ScrollTo(95)
	if state.Offset != 90 { // max is 100-10=90
		t.Errorf("Expected offset clamped to 90, got %d", state.Offset)
	}

	// Test clamping at start
	state.ScrollTo(-10)
	if state.Offset != 0 {
		t.Errorf("Expected offset clamped to 0, got %d", state.Offset)
	}

	// Test visible range
	state.ScrollTo(25)
	start, end := state.VisibleRange()
	if start != 25 || end != 35 {
		t.Errorf("Expected range 25-35, got %d-%d", start, end)
	}
}

// =============================================================================
// Scenario 7: Text Input Handling
// =============================================================================

func TestTextInputBasic(t *testing.T) {
	value := "Hello"

	inputUI := DInput{
		Value: &value,
		Width: 20,
	}

	frame := Execute(inputUI)
	frame.Layout(30, 5)

	// Focus the input
	if len(frame.focusables) == 0 {
		t.Fatal("Input should be focusable")
	}

	// Test inserting characters
	frame.InputEnd() // Move cursor to end
	frame.InputInsert('!')
	if value != "Hello!" {
		t.Errorf("Expected 'Hello!', got '%s'", value)
	}

	// Test cursor movement
	frame.InputMoveCursor(-1) // Before !
	frame.InputInsert('?')
	if value != "Hello?!" {
		t.Errorf("Expected 'Hello?!', got '%s'", value)
	}

	// Test backspace
	frame.InputBackspace() // Delete ?
	if value != "Hello!" {
		t.Errorf("Expected 'Hello!', got '%s'", value)
	}

	// Test delete
	frame.InputHome()
	frame.InputDelete() // Delete H
	if value != "ello!" {
		t.Errorf("Expected 'ello!', got '%s'", value)
	}

	// Test home/end
	frame.InputEnd()
	frame.InputInsert('X')
	if value != "ello!X" {
		t.Errorf("Expected 'ello!X', got '%s'", value)
	}

	frame.InputHome()
	frame.InputInsert('Y')
	if value != "Yello!X" {
		t.Errorf("Expected 'Yello!X', got '%s'", value)
	}
}

func TestTextInputCursorBounds(t *testing.T) {
	value := "ABC"

	inputUI := DInput{Value: &value, Width: 10}
	frame := Execute(inputUI)
	frame.Layout(20, 5)

	// Cursor should clamp to valid positions
	frame.InputMoveCursor(-100)
	node := frame.FocusedNode()
	if node.CursorPos != 0 {
		t.Errorf("Cursor should clamp to 0, got %d", node.CursorPos)
	}

	frame.InputMoveCursor(100)
	if node.CursorPos != 3 {
		t.Errorf("Cursor should clamp to len(value)=3, got %d", node.CursorPos)
	}
}

func TestTextInputOnChange(t *testing.T) {
	value := ""
	changeCount := 0

	inputUI := DInput{
		Value: &value,
		Width: 10,
		OnChange: func(v string) {
			changeCount++
		},
	}

	frame := Execute(inputUI)
	frame.Layout(20, 5)

	frame.InputInsert('A')
	frame.InputInsert('B')
	frame.InputBackspace()

	if changeCount != 3 {
		t.Errorf("OnChange should have been called 3 times, got %d", changeCount)
	}
}

func TestTextInputRendering(t *testing.T) {
	value := "Test"

	inputUI := DCol{Children: []any{
		DInput{Value: &value, Width: 10},
	}}

	frame := Execute(inputUI)
	frame.Layout(30, 5)
	buf := NewBuffer(30, 5)
	frame.Render(buf)

	output := buf.String()
	t.Logf("Input render:\n%s", output)

	if !containsString(output, "Test") {
		t.Error("Should show the input value")
	}
}

// =============================================================================
// Scenario 8: Custom Components
// =============================================================================

// Counter is a custom component that displays a count with +/- buttons
type Counter struct {
	Label string
	Value *int
}

// DeclRender implements DComponent
func (c Counter) DeclRender() any {
	return DRow{Gap: 1, Children: []any{
		DText{Content: c.Label + ":"},
		DButton{Label: "-", OnClick: func() { *c.Value-- }},
		DText{Content: func() string { return fmt.Sprintf("%d", *c.Value) }},
		DButton{Label: "+", OnClick: func() { *c.Value++ }},
	}}
}

// StatusBadge is a custom component that shows status with color
type StatusBadge struct {
	Status string
}

func (s StatusBadge) DeclRender() any {
	return DText{
		Content: "[" + s.Status + "]",
		Bold:    s.Status == "active",
	}
}

// Card is a reusable container component
type Card struct {
	Title    string
	Children []any
}

func (c Card) DeclRender() any {
	return DCol{Gap: 0, Children: []any{
		DText{Content: "╭─ " + c.Title + " ─╮", Bold: true},
		DCol{Children: c.Children},
		DText{Content: "╰───────────────╯"},
	}}
}

func TestCustomComponent(t *testing.T) {
	count := 5

	ui := DCol{Children: []any{
		Counter{Label: "Items", Value: &count},
		StatusBadge{Status: "active"},
	}}

	frame := Execute(ui)
	frame.Layout(50, 10)
	buf := NewBuffer(50, 10)
	frame.Render(buf)

	output := buf.String()
	t.Logf("Custom component:\n%s", output)

	// Verify the counter rendered
	if !containsString(output, "Items:") {
		t.Error("Should show counter label")
	}
	if !containsString(output, "5") {
		t.Error("Should show counter value")
	}
	if !containsString(output, "[active]") {
		t.Error("Should show status badge")
	}

	// Verify buttons are focusable
	if len(frame.focusables) != 2 {
		t.Errorf("Expected 2 focusable buttons, got %d", len(frame.focusables))
	}

	// Activate the + button
	frame.FocusNext() // Skip -
	frame.Activate()  // Click +

	if count != 6 {
		t.Errorf("Expected count to be 6 after clicking +, got %d", count)
	}
}

func TestNestedCustomComponents(t *testing.T) {
	count := 0

	ui := Card{
		Title: "Test Card",
		Children: []any{
			DText{Content: "Inside the card"},
			Counter{Label: "Count", Value: &count},
		},
	}

	frame := Execute(ui)
	frame.Layout(50, 10)
	buf := NewBuffer(50, 10)
	frame.Render(buf)

	output := buf.String()
	t.Logf("Nested custom:\n%s", output)

	if !containsString(output, "Test Card") {
		t.Error("Should show card title")
	}
	if !containsString(output, "Inside the card") {
		t.Error("Should show card content")
	}
	if !containsString(output, "Count:") {
		t.Error("Should show nested counter")
	}
}

// =============================================================================
// Scenario 9: Nested Focus Contexts (Modal/Dialog)
// =============================================================================

func TestFocusContextBasic(t *testing.T) {
	// Create UI with 3 buttons in main, then 2 in "modal"
	ui := DCol{Children: []any{
		// Main content (buttons 0-2)
		DButton{Label: "Main 1"},
		DButton{Label: "Main 2"},
		DButton{Label: "Main 3"},
		// Modal content (buttons 3-4)
		DButton{Label: "Modal OK"},
		DButton{Label: "Modal Cancel"},
	}}

	frame := Execute(ui)
	frame.Layout(40, 10)

	// Initially should have 5 focusables
	if len(frame.focusables) != 5 {
		t.Fatalf("Expected 5 focusables, got %d", len(frame.focusables))
	}

	// Tab through main - should cycle through all 5
	frame.FocusNext()
	if frame.focusIndex != 1 {
		t.Errorf("Expected focusIndex 1, got %d", frame.focusIndex)
	}

	// Now push a focus context at index 3 (simulating modal open)
	frame.focusIndex = 3 // Set to first modal button
	frame.PushFocusContext()

	// Should now be trapped between indices 3-4
	if frame.FocusContextDepth() != 1 {
		t.Errorf("Expected depth 1, got %d", frame.FocusContextDepth())
	}

	// Tab should cycle between Modal OK and Modal Cancel
	frame.FocusNext()
	if frame.focusIndex != 4 {
		t.Errorf("Expected focusIndex 4, got %d", frame.focusIndex)
	}

	frame.FocusNext() // Should wrap to 3
	if frame.focusIndex != 3 {
		t.Errorf("Expected focusIndex 3 (wrap), got %d", frame.focusIndex)
	}

	// Pop context - should restore previous focus
	frame.PopFocusContext()
	if frame.focusIndex != 3 { // Was at index 3 when we pushed
		t.Errorf("Expected restored focusIndex 3, got %d", frame.focusIndex)
	}

	// Now we can navigate all 5 again
	frame.FocusNext()
	if frame.focusIndex != 4 {
		t.Errorf("Expected focusIndex 4, got %d", frame.focusIndex)
	}
	frame.FocusNext() // Should wrap to 0
	if frame.focusIndex != 0 {
		t.Errorf("Expected focusIndex 0 (wrap to start), got %d", frame.focusIndex)
	}
}

func TestFocusContextPrevNavigation(t *testing.T) {
	ui := DCol{Children: []any{
		DButton{Label: "A"},
		DButton{Label: "B"},
		DButton{Label: "C"},
		DButton{Label: "D"},
	}}

	frame := Execute(ui)
	frame.Layout(40, 10)

	// Start at index 2, push context
	frame.focusIndex = 2
	frame.PushFocusContext()

	// Now trapped in C, D (indices 2, 3)
	frame.FocusPrev() // Should go to D (wrap around)
	if frame.focusIndex != 3 {
		t.Errorf("Expected focusIndex 3 (prev wraps to end), got %d", frame.focusIndex)
	}

	frame.FocusPrev() // Back to C
	if frame.focusIndex != 2 {
		t.Errorf("Expected focusIndex 2, got %d", frame.focusIndex)
	}
}

// =============================================================================
// Scenario 10: Styling/Theming
// =============================================================================

func TestStyling(t *testing.T) {
	ui := DCol{Children: []any{
		// Plain text
		DText{Content: "Plain text"},

		// Bold text (shorthand)
		DText{Content: "Bold text", Bold: true},

		// Styled text with color
		DText{
			Content: "Red text",
			Style:   DStyle{FG: ColorPtr(Red)},
		},

		// Combined styles
		DText{
			Content: "Yellow on Blue, Bold",
			Style: DStyle{
				FG:   ColorPtr(Yellow),
				BG:   ColorPtr(Blue),
				Bold: true,
			},
		},

		// Dim text
		DText{
			Content: "Dim text",
			Style:   DStyle{Dim: true},
		},
	}}

	frame := Execute(ui)
	frame.Layout(40, 10)
	buf := NewBuffer(40, 10)
	frame.Render(buf)

	output := buf.String()
	t.Logf("Styled text:\n%s", output)

	if !containsString(output, "Plain text") {
		t.Error("Should show plain text")
	}
	if !containsString(output, "Bold text") {
		t.Error("Should show bold text")
	}
	if !containsString(output, "Red text") {
		t.Error("Should show red text")
	}

	// Verify the style is actually applied on the node
	nodes := frame.nodes
	for _, node := range nodes {
		if node.Type == "text" && node.Text == "Red text" {
			if node.Style.FG != Red {
				t.Error("Red text should have Red foreground color")
			}
		}
		if node.Type == "text" && node.Text == "Yellow on Blue, Bold" {
			if node.Style.FG != Yellow || node.Style.BG != Blue {
				t.Error("Styled text should have Yellow FG and Blue BG")
			}
			if node.Style.Attr&AttrBold == 0 {
				t.Error("Styled text should be bold")
			}
		}
	}
}

// =============================================================================
// Scenario 11: Comprehensive Test - All Features Together
// =============================================================================

// SettingsPanel is a custom component demonstrating all features
type SettingsPanel struct {
	Title     string
	BoolVal   *bool
	StringVal *string
	Counter   *int
}

func (s SettingsPanel) DeclRender() any {
	return DCol{Gap: 1, Children: []any{
		DText{Content: s.Title, Style: DStyle{Bold: true, FG: ColorPtr(Cyan)}},
		DCheckbox{Label: "Enable feature", Checked: s.BoolVal},
		DRow{Children: []any{
			DText{Content: "Name: "},
			DInput{Value: s.StringVal, Width: 15},
		}},
		DRow{Gap: 1, Children: []any{
			DText{Content: "Count:"},
			DButton{Label: "-", OnClick: func() { *s.Counter-- }},
			DText{Content: func() string { return fmt.Sprintf("%d", *s.Counter) }},
			DButton{Label: "+", OnClick: func() { *s.Counter++ }},
		}},
	}}
}

func TestComprehensiveAllFeatures(t *testing.T) {
	// State for the test
	enabled := true
	name := "Test"
	count := 5
	scrollOffset := 0
	showDetails := true

	// Build a complex UI using all features
	ui := DCol{Gap: 1, Children: []any{
		// Header with styling
		DText{Content: "=== Comprehensive Test ===", Style: DStyle{Bold: true, FG: ColorPtr(Yellow)}},

		// Custom component
		SettingsPanel{
			Title:     "Settings",
			BoolVal:   &enabled,
			StringVal: &name,
			Counter:   &count,
		},

		// Conditional content
		DIf(&showDetails,
			DCol{Children: []any{
				DText{Content: "Details visible", Style: DStyle{FG: ColorPtr(Green)}},
				DCheckbox{Checked: &showDetails, Label: "Show details"},
			}},
		),

		// Scrollable list with viewport culling
		DText{Content: "Scrollable Items:"},
		DScroll{
			Height:     3,
			ItemHeight: 1,
			Offset:     &scrollOffset,
			Children: func() []any {
				items := make([]any, 20)
				for i := range items {
					items[i] = DButton{Label: fmt.Sprintf("Item %02d", i)}
				}
				return items
			}(),
		},

		// Switch case
		DSwitch(&count,
			DCase(0, DText{Content: "Count is zero", Style: DStyle{FG: ColorPtr(Red)}}),
			DCase(5, DText{Content: "Count is five (default)"}),
			DDefault(DText{Content: func() string { return fmt.Sprintf("Count is %d", count) }}),
		),

		// Dynamic list with ForEach
		DForEach(func() []string { return []string{"A", "B", "C"} }, func(s *string) any {
			return DText{Content: *s}
		}),
	}}

	frame := Execute(ui)
	frame.Layout(50, 30)
	buf := NewBuffer(50, 30)
	frame.Render(buf)

	output := buf.String()
	t.Logf("Comprehensive UI:\n%s", output)

	// Verify all components rendered
	if !containsString(output, "Comprehensive Test") {
		t.Error("Should show header")
	}
	if !containsString(output, "Settings") {
		t.Error("Should show custom component")
	}
	if !containsString(output, "Details visible") {
		t.Error("Should show conditional content")
	}
	if !containsString(output, "Item 00") {
		t.Error("Should show scroll items")
	}
	if !containsString(output, "Count is five") {
		t.Error("Should show switch case")
	}

	// Verify interactions work
	// Count: 2 checkboxes + 1 input + 2 buttons (-, +) + 3 visible scroll items = 8
	// Note: scroll uses viewport culling, so only visible items are in the tree
	focusableCount := len(frame.focusables)
	if focusableCount != 8 {
		t.Errorf("Expected 8 focusables (with viewport culling), got %d", focusableCount)
	}

	// Test focus navigation
	startFocus := frame.focusIndex
	frame.FocusNext()
	if frame.focusIndex == startFocus {
		t.Error("FocusNext should change focus")
	}

	// Test activation
	frame.FocusNext() // Move to counter - button
	frame.FocusNext()
	frame.Activate()

	// Test scroll
	scrollOffset = 5
	frame2 := Execute(ui)
	frame2.Layout(50, 30)
	buf.Clear()
	frame2.Render(buf)
	output = buf.String()
	if !containsString(output, "Item 05") {
		t.Error("Scroll should show Item 05 at offset 5")
	}

	t.Logf("Focusable count: %d", focusableCount)
}

// =============================================================================
// Scenario 12: Performance Test - 100+ Interactive Elements
// =============================================================================

func TestPerformance100Elements(t *testing.T) {
	// Create UI with 100+ buttons
	buttons := make([]any, 100)
	for i := range buttons {
		buttons[i] = DButton{Label: fmt.Sprintf("Btn%03d", i)}
	}

	ui := DCol{Children: buttons}

	frame := Execute(ui)
	frame.Layout(60, 50)
	buf := NewBuffer(60, 50)
	frame.Render(buf)

	// Verify all buttons are focusable
	if len(frame.focusables) != 100 {
		t.Errorf("Expected 100 focusables, got %d", len(frame.focusables))
	}

	// Test focus navigation through all 100
	for i := 0; i < 100; i++ {
		frame.FocusNext()
	}
	// Should wrap back to start
	if frame.focusIndex != 0 {
		t.Errorf("After 100 FocusNext, should be back at 0, got %d", frame.focusIndex)
	}

	// Test jump labels
	frame.EnterJumpMode()
	if !frame.InJumpMode() {
		t.Error("Should be in jump mode")
	}

	// With 100 items, should use 2-character labels (26*26 = 676 > 100)
	targets := frame.JumpTargets()
	if len(targets) != 100 {
		t.Errorf("Expected 100 jump targets, got %d", len(targets))
	}
	// All labels should be 2 chars
	for i, target := range targets {
		if len(target.Label) != 2 {
			t.Errorf("Target %d: expected 2-char label, got %q", i, target.Label)
			break
		}
	}
	frame.ExitJumpMode()

	t.Logf("100 elements test passed: %d focusables, %d jump targets", len(frame.focusables), len(targets))
}

func BenchmarkComplex100Elements(b *testing.B) {
	// Create complex UI with 100+ interactive elements
	buttons := make([]any, 100)
	for i := range buttons {
		i := i
		buttons[i] = DButton{
			Label:   fmt.Sprintf("Button %03d", i),
			OnClick: func() {},
		}
	}

	ui := DCol{Children: append([]any{
		DText{Content: "Header", Style: DStyle{Bold: true}},
		DCheckbox{Checked: new(bool), Label: "Option"},
		DInput{Value: new(string), Width: 20},
	}, buttons...)}

	frame := NewDFrame()
	buf := NewBuffer(60, 50)

	// Warm up
	ExecuteInto(frame, ui)
	frame.Layout(60, 50)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ExecuteInto(frame, ui)
		frame.Layout(60, 50)
		buf.Clear()
		frame.Render(buf)
	}
}

func BenchmarkScrollViewportCulling(b *testing.B) {
	// Create 1000 items
	items := make([]any, 1000)
	for i := range items {
		items[i] = DText{Content: fmt.Sprintf("Item %04d with some extra text", i)}
	}

	scrollOffset := 0

	scrollUI := DScroll{
		Children:   items,
		Height:     20,
		ItemHeight: 1,
		Offset:     &scrollOffset,
	}

	frame := NewDFrame()
	buf := NewBuffer(60, 25)

	// Warm up
	ExecuteInto(frame, scrollUI)
	frame.Layout(60, 25)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate scrolling during benchmark
		scrollOffset = i % 980

		ExecuteInto(frame, scrollUI)
		frame.Layout(60, 25)
		buf.Clear()
		frame.Render(buf)
	}
}

// =============================================================================
// DScrollView Tests
// =============================================================================

func TestScrollViewBasic(t *testing.T) {
	offset := 0

	ui := DScrollView{
		Height: 5,
		Offset: &offset,
		Content: DCol{
			Children: []any{
				DText{Content: "Line 0"},
				DText{Content: "Line 1"},
				DText{Content: "Line 2"},
				DText{Content: "Line 3"},
				DText{Content: "Line 4"},
				DText{Content: "Line 5"},
				DText{Content: "Line 6"},
				DText{Content: "Line 7"},
				DText{Content: "Line 8"},
				DText{Content: "Line 9"},
			},
		},
	}

	frame := NewDFrame()
	buf := NewBuffer(40, 10)

	// Render at offset 0
	ExecuteInto(frame, ui)
	frame.Layout(40, 10)
	frame.Render(buf)

	// Check visible content (use HasPrefix to ignore scrollbar)
	line0 := buf.GetLine(0)
	if !strings.HasPrefix(line0, "Line 0") {
		t.Errorf("At offset 0, line 0 = %q, want prefix %q", line0, "Line 0")
	}

	// Scroll down
	offset = 3
	frame.Reset()
	buf.Clear()
	ExecuteInto(frame, ui)
	frame.Layout(40, 10)
	frame.Render(buf)

	line0 = buf.GetLine(0)
	if !strings.HasPrefix(line0, "Line 3") {
		t.Errorf("At offset 3, line 0 = %q, want prefix %q", line0, "Line 3")
	}

	t.Logf("ScrollView basic test passed")
}

func TestScrollViewNested(t *testing.T) {
	outerOffset := 0
	innerOffset := 0

	// Create inner scroll with 20 items
	innerItems := make([]any, 20)
	for i := range innerItems {
		innerItems[i] = DText{Content: fmt.Sprintf("Inner %02d", i)}
	}

	ui := DScrollView{
		Height: 10,
		Offset: &outerOffset,
		Content: DCol{
			Children: []any{
				DText{Content: "=== Outer Header ==="},
				DText{Content: "Some intro text"},
				DScrollView{
					Height: 5,
					Offset: &innerOffset,
					Content: DCol{
						Children: innerItems,
					},
				},
				DText{Content: "More outer content"},
				DText{Content: "Line A"},
				DText{Content: "Line B"},
				DText{Content: "Line C"},
				DText{Content: "Line D"},
				DText{Content: "Line E"},
				DText{Content: "Line F"},
			},
		},
	}

	frame := NewDFrame()
	buf := NewBuffer(40, 15)

	ExecuteInto(frame, ui)
	frame.Layout(40, 15)
	frame.Render(buf)

	// Check outer header visible
	line0 := buf.GetLine(0)
	if !strings.HasPrefix(line0, "=== Outer Header ===") {
		t.Errorf("Line 0 = %q, want header prefix", line0)
	}

	// Check inner scroll shows first items
	line2 := buf.GetLine(2)
	if !strings.HasPrefix(line2, "Inner 00") {
		t.Errorf("Line 2 (inner scroll) = %q, want prefix %q", line2, "Inner 00")
	}

	// Scroll inner
	innerOffset = 5
	frame.Reset()
	buf.Clear()
	ExecuteInto(frame, ui)
	frame.Layout(40, 15)
	frame.Render(buf)

	line2 = buf.GetLine(2)
	if !strings.HasPrefix(line2, "Inner 05") {
		t.Errorf("After inner scroll, line 2 = %q, want prefix %q", line2, "Inner 05")
	}

	// Scroll outer
	outerOffset = 3
	frame.Reset()
	buf.Clear()
	ExecuteInto(frame, ui)
	frame.Layout(40, 15)
	frame.Render(buf)

	t.Logf("Nested ScrollView test passed")
	t.Logf("Outer offset: %d, Inner offset: %d", outerOffset, innerOffset)
}

func TestScrollViewWithInteractives(t *testing.T) {
	offset := 0
	clicked := -1

	// Create buttons inside scrollview
	buttons := make([]any, 10)
	for i := range buttons {
		idx := i
		buttons[i] = DButton{
			Label:   fmt.Sprintf("Button %d", idx),
			OnClick: func() { clicked = idx },
		}
	}

	ui := DScrollView{
		Height: 5,
		Offset: &offset,
		Content: DCol{
			Children: buttons,
		},
	}

	frame := NewDFrame()
	buf := NewBuffer(40, 10)

	ExecuteInto(frame, ui)
	frame.Layout(40, 10)
	frame.Render(buf)

	// Check we have focusables
	if len(frame.focusables) != 10 {
		t.Errorf("Got %d focusables, want 10", len(frame.focusables))
	}

	// Activate first button
	frame.Activate()
	if clicked != 0 {
		t.Errorf("Clicked = %d, want 0", clicked)
	}

	// Navigate and activate
	frame.FocusNext()
	frame.FocusNext()
	frame.Activate()
	if clicked != 2 {
		t.Errorf("Clicked = %d, want 2", clicked)
	}

	t.Logf("ScrollView with interactives test passed")
}

func BenchmarkScrollView(b *testing.B) {
	offset := 0

	// Create 100 lines of content
	lines := make([]any, 100)
	for i := range lines {
		lines[i] = DText{Content: fmt.Sprintf("Line %03d with some additional text content", i)}
	}

	ui := DScrollView{
		Height: 20,
		Offset: &offset,
		Content: DCol{
			Children: lines,
		},
	}

	frame := NewDFrame()
	buf := NewBuffer(80, 25)

	// Warm up
	ExecuteInto(frame, ui)
	frame.Layout(80, 25)
	frame.Render(buf)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		offset = i % 80

		ExecuteInto(frame, ui)
		frame.Layout(80, 25)
		buf.Clear()
		frame.Render(buf)
	}
}

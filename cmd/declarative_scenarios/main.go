package main

import (
	"fmt"
	"log"
	"time"

	"riffkey"
	. "tui"
)

// =============================================================================
// Scenario selector
// =============================================================================

var currentScenario = 0
var scenarios = []string{"tasks", "dashboard", "form", "tabs", "scroll", "stress", "webpage"}

// =============================================================================
// Scenario 1: Filterable Task List
// =============================================================================

type Task struct {
	ID        int
	Title     string
	Completed bool
	Priority  string
}

var taskState = struct {
	Tasks  []Task
	Filter string
}{
	Tasks: []Task{
		{1, "Write tests", false, "high"},
		{2, "Review PR", true, "medium"},
		{3, "Update docs", false, "low"},
		{4, "Fix bug", false, "high"},
		{5, "Deploy", true, "medium"},
		{6, "Code review", false, "medium"},
		{7, "Write docs", true, "low"},
	},
	Filter: "all",
}

func filterTasks() []Task {
	var result []Task
	for _, t := range taskState.Tasks {
		switch taskState.Filter {
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
	Gap: 0,
	Children: []any{
		DText{Content: "━━━ Task List ━━━", Bold: true},
		DRow{Gap: 1, Children: []any{
			DButton{Label: "All", OnClick: func() { taskState.Filter = "all" }},
			DButton{Label: "Active", OnClick: func() { taskState.Filter = "active" }},
			DButton{Label: "Done", OnClick: func() { taskState.Filter = "completed" }},
		}},
		DText{Content: func() string {
			return fmt.Sprintf("Filter: %s (%d items)", taskState.Filter, len(filterTasks()))
		}},
		DText{Content: ""},
		DForEach(filterTasks, func(t *Task) any {
			return DRow{Gap: 1, Children: []any{
				DCheckbox{
					Checked: &t.Completed,
					Label:   t.Title,
				},
				DSwitch(&t.Priority,
					DCase("low", DText{Content: "Low"}),
					DCase("medium", DText{Content: "Medium"}),
					DCase("high", DText{Content: "High"}),
				),
				// DIf(func() bool { return t.Priority == "high" },
				// 	DText{Content: "⚡", Bold: true},
				// ),
				// DIf(func() bool { return t.Priority == "medium" },
				// 	DText{Content: "•"},
				// ),
			}}
		}),
	},
}

// =============================================================================
// Scenario 2: System Dashboard
// =============================================================================

var dashState = struct {
	CPUCores    []int
	MemUsed     int
	MemTotal    int
	NetIn       float64
	NetOut      float64
	Alerts      []string
	ShowDetails bool
}{
	CPUCores:    []int{45, 67, 23, 89, 34, 56, 78, 12},
	MemUsed:     12288,
	MemTotal:    16384,
	NetIn:       5.4,
	NetOut:      1.2,
	Alerts:      []string{"High CPU on core 4", "Memory usage above 75%"},
	ShowDetails: true,
}

var dashboardUI = DCol{
	Gap: 1,
	Children: []any{
		DRow{Children: []any{
			DText{Content: "━━━ System Dashboard ━━━", Bold: true},
			DText{Content: "  "},
			DText{Content: func() string {
				return time.Now().Format("15:04:05")
			}},
		}},

		DCheckbox{
			Checked: &dashState.ShowDetails,
			Label:   "Show details",
		},

		DText{Content: ""},
		DText{Content: "CPU Cores:", Bold: true},
		DForEach(&dashState.CPUCores, func(usage *int) any {
			return DRow{Children: []any{
				DProgress{Value: usage, Width: 25},
				DText{Content: func() string {
					return fmt.Sprintf(" %3d%%", *usage)
				}},
			}}
		}),

		DIf(&dashState.ShowDetails,
			DCol{Gap: 1, Children: []any{
				DText{Content: ""},
				DRow{Children: []any{
					DText{Content: "Memory: "},
					DProgress{
						Value: func() int { return dashState.MemUsed * 100 / dashState.MemTotal },
						Width: 20,
					},
					DText{Content: func() string {
						return fmt.Sprintf(" %dMB/%dMB", dashState.MemUsed, dashState.MemTotal)
					}},
				}},

				DRow{Gap: 2, Children: []any{
					DText{Content: func() string {
						return fmt.Sprintf("↑ %.1f MB/s", dashState.NetOut)
					}},
					DText{Content: func() string {
						return fmt.Sprintf("↓ %.1f MB/s", dashState.NetIn)
					}},
				}},
			}},
		),

		DIf(func() bool { return len(dashState.Alerts) > 0 },
			DCol{Children: []any{
				DText{Content: ""},
				DText{Content: "⚠ Alerts:", Bold: true},
				DForEach(&dashState.Alerts, func(alert *string) any {
					return DText{Content: func() string { return "  • " + *alert }}
				}),
			}},
		),
	},
}

// =============================================================================
// Scenario 3: Form with Validation
// =============================================================================

var formState = struct {
	Username  string
	Email     string
	Subscribe bool
	Submitted bool
	Error     string
}{
	Username:  "",
	Email:     "",
	Subscribe: true,
	Submitted: false,
}

func validateAndSubmit() {
	if len(formState.Username) < 3 {
		formState.Error = "Username must be at least 3 characters"
		return
	}
	if len(formState.Email) < 5 || !containsAt(formState.Email) {
		formState.Error = "Please enter a valid email"
		return
	}
	formState.Error = ""
	formState.Submitted = true
}

func containsAt(s string) bool {
	for _, c := range s {
		if c == '@' {
			return true
		}
	}
	return false
}

var formUI = DCol{
	Gap: 1,
	Children: []any{
		DText{Content: "━━━ Registration Form ━━━", Bold: true},

		DIf(&formState.Submitted,
			DCol{Children: []any{
				DText{Content: "✓ Registration successful!", Bold: true},
				DText{Content: func() string {
					return "Welcome, " + formState.Username + "!"
				}},
				DButton{Label: "Reset", OnClick: func() {
					formState.Username = ""
					formState.Email = ""
					formState.Submitted = false
					formState.Error = ""
				}},
			}},
		),

		DIf(func() bool { return !formState.Submitted },
			DCol{Gap: 1, Children: []any{
				DRow{Children: []any{
					DText{Content: "Username: "},
					DInput{Value: &formState.Username, Width: 20},
				}},

				DRow{Children: []any{
					DText{Content: "Email:    "},
					DInput{Value: &formState.Email, Width: 20},
				}},

				DCheckbox{
					Checked: &formState.Subscribe,
					Label:   "Subscribe to newsletter",
				},

				DIf(func() bool { return formState.Error != "" },
					DText{Content: func() string { return "✗ " + formState.Error }},
				),

				DButton{Label: "Submit", OnClick: validateAndSubmit},
			}},
		),
	},
}

// =============================================================================
// Scenario 4: Tabbed Interface
// =============================================================================

var tabState = struct {
	Active   string
	Counter  int
	Message  string
	DarkMode bool
}{
	Active:   "home",
	Counter:  0,
	Message:  "Hello!",
	DarkMode: false,
}

func tabBtn(id, label string) DButton {
	return DButton{
		Label: func() string {
			if tabState.Active == id {
				return "[" + label + "]"
			}
			return " " + label + " "
		},
		OnClick: func() { tabState.Active = id },
	}
}

var tabbedUI = DCol{
	Gap: 1,
	Children: []any{
		DText{Content: "━━━ Tabbed Interface ━━━", Bold: true},

		DRow{Children: []any{
			tabBtn("home", "Home"),
			tabBtn("counter", "Counter"),
			tabBtn("settings", "Settings"),
		}},

		DText{Content: ""},

		DSwitch(&tabState.Active,
			DCase("home", DCol{Children: []any{
				DText{Content: "Welcome Home!", Bold: true},
				DText{Content: "Use Tab/j/k to navigate, Enter to activate."},
				DRow{Children: []any{
					DText{Content: "Message: "},
					DInput{Value: &tabState.Message, Width: 20},
				}},
				DText{Content: func() string { return "You typed: " + tabState.Message }},
			}}),

			DCase("counter", DCol{Children: []any{
				DText{Content: "Counter Demo", Bold: true},
				DText{Content: func() string {
					return fmt.Sprintf("Count: %d", tabState.Counter)
				}},
				DRow{Gap: 1, Children: []any{
					DButton{Label: "-", OnClick: func() { tabState.Counter-- }},
					DButton{Label: "+", OnClick: func() { tabState.Counter++ }},
					DButton{Label: "Reset", OnClick: func() { tabState.Counter = 0 }},
				}},
				DProgress{
					Value: func() int {
						v := tabState.Counter % 101
						if v < 0 {
							v = -v
						}
						return v
					},
					Width: 30,
				},
			}}),

			DCase("settings", DCol{Children: []any{
				DText{Content: "Settings", Bold: true},
				DCheckbox{
					Checked: &tabState.DarkMode,
					Label:   "Dark mode (visual only)",
				},
				DText{Content: func() string {
					if tabState.DarkMode {
						return "Theme: Dark"
					}
					return "Theme: Light"
				}},
			}}),
		),
	},
}

// =============================================================================
// Scenario 5: Scrollable List (100+ items with viewport culling)
// =============================================================================

var scrollState = struct {
	Items    []string
	Offset   int
	Selected int
}{
	Items:    make([]string, 100),
	Offset:   0,
	Selected: -1,
}

func init() {
	for i := range scrollState.Items {
		scrollState.Items[i] = fmt.Sprintf("Item %03d - This is a list item with some content", i+1)
	}
}

var scrollUI = DCol{
	Gap: 1,
	Children: []any{
		DText{Content: "━━━ Scrollable List (100 items, 10 visible) ━━━", Bold: true},
		DText{Content: func() string {
			return fmt.Sprintf("Scroll offset: %d | Selected: %d", scrollState.Offset, scrollState.Selected)
		}},
		DRow{Gap: 1, Children: []any{
			DButton{Label: "Top", OnClick: func() { scrollState.Offset = 0 }},
			DButton{Label: "↑ Page", OnClick: func() {
				scrollState.Offset -= 10
				if scrollState.Offset < 0 {
					scrollState.Offset = 0
				}
			}},
			DButton{Label: "↓ Page", OnClick: func() {
				scrollState.Offset += 10
				if scrollState.Offset > 90 {
					scrollState.Offset = 90
				}
			}},
			DButton{Label: "Bottom", OnClick: func() { scrollState.Offset = 90 }},
		}},
		DText{Content: ""},
		DScroll{
			Height:     10,
			ItemHeight: 1,
			Offset:     &scrollState.Offset,
			Children: func() []any {
				var items []any
				for i := range scrollState.Items {
					idx := i
					items = append(items, DButton{
						Label: func() string {
							prefix := "  "
							if idx == scrollState.Selected {
								prefix = "► "
							}
							return prefix + scrollState.Items[idx]
						},
						OnClick: func() { scrollState.Selected = idx },
					})
				}
				return items
			}(),
		},
		DText{Content: ""},
		DText{Content: "Only 10 items rendered at a time (viewport culling)"},
	},
}

// =============================================================================
// Scenario 6: Stress Test (100 interactive buttons)
// =============================================================================

var stressState = struct {
	Buttons []int
	LastClicked int
}{
	Buttons:     make([]int, 100),
	LastClicked: -1,
}

var stressUI = DCol{
	Gap: 1,
	Children: []any{
		DText{Content: "━━━ Stress Test (100 buttons) ━━━", Bold: true},
		DText{Content: func() string {
			if stressState.LastClicked >= 0 {
				return fmt.Sprintf("Last clicked: Button %d (clicked %d times)",
					stressState.LastClicked+1, stressState.Buttons[stressState.LastClicked])
			}
			return "Click any button! Use ; for jump mode"
		}},
		DText{Content: ""},
		DCol{
			Gap: 0,
			Children: func() []any {
				var rows []any
				for row := 0; row < 10; row++ {
					var btns []any
					for col := 0; col < 10; col++ {
						idx := row*10 + col
						btns = append(btns, DButton{
							Label: func() string {
								return fmt.Sprintf("%02d", idx+1)
							},
							OnClick: func() {
								stressState.Buttons[idx]++
								stressState.LastClicked = idx
							},
						})
					}
					rows = append(rows, DRow{Gap: 1, Children: btns})
				}
				return rows
			}(),
		},
		DText{Content: ""},
		DText{Content: "Press ; to see 2-char jump labels for all 100 buttons!"},
	},
}

// =============================================================================
// Scenario 7: Web Page (DScrollView with variable-height content)
// =============================================================================

var webpageState = struct {
	ScrollOffset  int
	TOCOffset     int
	ShowTOC       bool
	ExpandedFAQ   map[int]bool
	ContactName   string
	ContactEmail  string
	Newsletter    bool
}{
	ScrollOffset:  0,
	TOCOffset:     0,
	ShowTOC:       true,
	ExpandedFAQ:   make(map[int]bool),
	ContactName:   "",
	ContactEmail:  "",
	Newsletter:    true,
}

// Simulated article paragraphs
var articleParagraphs = []string{
	"Welcome to the TUI Framework documentation. This page demonstrates the DScrollView component, which enables smooth scrolling of variable-height content.",
	"Unlike the item-based DScroll component, DScrollView renders all content to an off-screen buffer and then displays a viewport slice. This allows for mixed content types with different heights.",
	"The framework supports nested scroll views, interactive elements within scrollable areas, and automatic scrollbar rendering when content exceeds the viewport.",
}

var faqItems = []struct {
	Q string
	A string
}{
	{"What is DScrollView?", "DScrollView is a scroll container that renders content to an off-screen buffer, then displays a slice based on scroll offset. It supports variable-height content."},
	{"How is it different from DScroll?", "DScroll uses item-based viewport culling with fixed item heights. DScrollView renders everything and clips to viewport, supporting mixed content heights."},
	{"Can I nest scroll views?", "Yes! DScrollView fully supports nesting. You can have a scrollable table of contents inside a scrollable page, for example."},
	{"What about performance?", "DScrollView creates a buffer each frame (~100µs for 100 lines). For most UIs this is negligible. Use DScroll for very long uniform lists."},
}

func webpageTOC() any {
	return DScrollView{
		Height: 8,
		Offset: &webpageState.TOCOffset,
		Content: DCol{
			Gap: 0,
			Children: []any{
				DButton{Label: "→ Introduction", OnClick: func() { webpageState.ScrollOffset = 0 }},
				DButton{Label: "→ Features", OnClick: func() { webpageState.ScrollOffset = 8 }},
				DButton{Label: "→ Code Example", OnClick: func() { webpageState.ScrollOffset = 16 }},
				DButton{Label: "→ FAQ", OnClick: func() { webpageState.ScrollOffset = 24 }},
				DButton{Label: "→ API Reference", OnClick: func() { webpageState.ScrollOffset = 40 }},
				DButton{Label: "→ Contact", OnClick: func() { webpageState.ScrollOffset = 55 }},
				DButton{Label: "→ Footer", OnClick: func() { webpageState.ScrollOffset = 70 }},
			},
		},
	}
}

func webpageContent() any {
	return DCol{
		Gap: 0, // Use explicit empty lines for spacing control
		Children: []any{
			// Section: Introduction
			DText{Content: "╔══════════════════════════════════════════════════════════════╗", Bold: true},
			DText{Content: "║          DScrollView - Variable Height Scrolling             ║", Bold: true},
			DText{Content: "╚══════════════════════════════════════════════════════════════╝", Bold: true},
			DText{Content: ""},
			DText{Content: articleParagraphs[0]},
			DText{Content: ""},

			// Section: Features
			DText{Content: "━━━ Features ━━━", Bold: true},
			DText{Content: ""},
			DText{Content: "• Variable-height content support"},
			DText{Content: "• Row-based scrolling (not item-based)"},
			DText{Content: "• Nested scroll views"},
			DText{Content: "• Interactive elements inside scroll areas"},
			DText{Content: "• Automatic scrollbar rendering"},
			DText{Content: ""},

			// Section: Code Example
			DText{Content: "━━━ Code Example ━━━", Bold: true},
			DText{Content: ""},
			DText{Content: "┌─────────────────────────────────────────────────────────────┐"},
			DText{Content: "│  offset := 0                                                │"},
			DText{Content: "│                                                             │"},
			DText{Content: "│  ui := DScrollView{                                         │"},
			DText{Content: "│      Height: 20,                                            │"},
			DText{Content: "│      Offset: &offset,                                       │"},
			DText{Content: "│      Content: DCol{Children: []any{...}},                   │"},
			DText{Content: "│  }                                                          │"},
			DText{Content: "└─────────────────────────────────────────────────────────────┘"},
			DText{Content: ""},

			// Section: FAQ (collapsible)
			DText{Content: "━━━ Frequently Asked Questions ━━━", Bold: true},
			DText{Content: ""},
			DForEach(&faqItems, func(item *struct{ Q, A string }) any {
				idx := 0
				for i := range faqItems {
					if &faqItems[i] == item {
						idx = i
						break
					}
				}
				return DCol{
					Gap: 0,
					Children: []any{
						DButton{
							Label: func() string {
								prefix := "▶ "
								if webpageState.ExpandedFAQ[idx] {
									prefix = "▼ "
								}
								return prefix + item.Q
							},
							OnClick: func() {
								webpageState.ExpandedFAQ[idx] = !webpageState.ExpandedFAQ[idx]
							},
						},
						DIf(func() bool { return webpageState.ExpandedFAQ[idx] },
							DText{Content: "   " + item.A},
						),
						DText{Content: ""},
					},
				}
			}),

			// Section: API Reference
			DText{Content: "━━━ API Reference ━━━", Bold: true},
			DText{Content: ""},
			DText{Content: "┌──────────────┬───────────┬─────────────────────────────────┐"},
			DText{Content: "│ Field        │ Type      │ Description                     │"},
			DText{Content: "├──────────────┼───────────┼─────────────────────────────────┤"},
			DText{Content: "│ Content      │ any       │ Child component(s) to render    │"},
			DText{Content: "│ Height       │ int16     │ Viewport height in rows         │"},
			DText{Content: "│ Width        │ int16     │ Viewport width (0 = auto)       │"},
			DText{Content: "│ Offset       │ *int      │ Pointer to scroll offset        │"},
			DText{Content: "└──────────────┴───────────┴─────────────────────────────────┘"},
			DText{Content: ""},
			DText{Content: articleParagraphs[1]},
			DText{Content: ""},
			DText{Content: articleParagraphs[2]},
			DText{Content: ""},

			// Section: Contact Form
			DText{Content: "━━━ Contact Us ━━━", Bold: true},
			DText{Content: ""},
			DRow{Children: []any{
				DText{Content: "Name:  "},
				DInput{Value: &webpageState.ContactName, Width: 30},
			}},
			DRow{Children: []any{
				DText{Content: "Email: "},
				DInput{Value: &webpageState.ContactEmail, Width: 30},
			}},
			DCheckbox{
				Checked: &webpageState.Newsletter,
				Label:   "Subscribe to newsletter",
			},
			DButton{Label: "Submit", OnClick: func() {
				// Would submit form
			}},
			DText{Content: ""},

			// Footer
			DText{Content: "═══════════════════════════════════════════════════════════════"},
			DText{Content: "TUI Framework v1.0 | Built with Go | Powered by DScrollView"},
			DText{Content: "Press Ctrl+D/U to scroll, ; for jump mode"},
			DText{Content: "═══════════════════════════════════════════════════════════════"},
		},
	}
}

var webpageUI = DRow{
	Gap: 2,
	Children: []any{
		// Left sidebar: Table of Contents (nested scroll)
		DIf(&webpageState.ShowTOC,
			DCol{
				Gap: 0,
				Children: []any{
					DText{Content: "┌─ Contents ──┐", Bold: true},
					webpageTOC(),
					DText{Content: "└─────────────┘"},
					DCheckbox{
						Checked: &webpageState.ShowTOC,
						Label:   "Show TOC",
					},
				},
			},
		),

		// Main content area
		DCol{
			Gap: 0,
			Children: []any{
				DText{Content: func() string {
					return fmt.Sprintf("Scroll: %d/70", webpageState.ScrollOffset)
				}},
				DScrollView{
					Height: 18,
					Offset: &webpageState.ScrollOffset,
					Content: webpageContent(),
				},
			},
		},
	},
}

// =============================================================================
// Main UI - scenario switcher
// =============================================================================

func getScenarioUI() any {
	switch scenarios[currentScenario] {
	case "tasks":
		return taskListUI
	case "dashboard":
		return dashboardUI
	case "form":
		return formUI
	case "tabs":
		return tabbedUI
	case "scroll":
		return scrollUI
	case "stress":
		return stressUI
	case "webpage":
		return webpageUI
	}
	return taskListUI
}

var mainUI = DCol{
	Children: []any{
		DRow{Gap: 1, Children: []any{
			DText{Content: "Scenarios:"},
			DButton{Label: "←", OnClick: func() {
				currentScenario = (currentScenario - 1 + len(scenarios)) % len(scenarios)
			}},
			DText{Content: func() string {
				return fmt.Sprintf("[%d/%d] %s", currentScenario+1, len(scenarios), scenarios[currentScenario])
			}},
			DButton{Label: "→", OnClick: func() {
				currentScenario = (currentScenario + 1) % len(scenarios)
			}},
		}},
		DText{Content: "─────────────────────────────────────────"},
	},
}

func main() {
	app, err := NewDeclApp(mainUI)
	if err != nil {
		log.Fatal(err)
	}

	// Override the UI to include both the header and current scenario
	renderUI := func() any {
		return DCol{
			Gap: 0,
			Children: []any{
				mainUI,
				getScenarioUI(),
				DText{Content: ""},
				DText{Content: "─────────────────────────────────────────"},
				DText{Content: "Tab/j/k=Navigate  Enter=Activate  Ctrl+D/U=Scroll  ;=Jump  q=Quit"},
			},
		}
	}

	// Custom render loop that rebuilds UI each frame
	app.Handle("q", func(m riffkey.Match) { app.Stop() })
	app.Handle("<C-c>", func(m riffkey.Match) { app.Stop() })

	// Scroll keybindings (Ctrl+D/U for half-page, Ctrl+F/B for full page)
	// Works for both scroll and webpage scenarios
	app.Handle("<C-d>", func(m riffkey.Match) {
		switch scenarios[currentScenario] {
		case "scroll":
			scrollState.Offset += 5
			if scrollState.Offset > 90 {
				scrollState.Offset = 90
			}
		case "webpage":
			webpageState.ScrollOffset += 5
			if webpageState.ScrollOffset > 70 {
				webpageState.ScrollOffset = 70
			}
		}
	})
	app.Handle("<C-u>", func(m riffkey.Match) {
		switch scenarios[currentScenario] {
		case "scroll":
			scrollState.Offset -= 5
			if scrollState.Offset < 0 {
				scrollState.Offset = 0
			}
		case "webpage":
			webpageState.ScrollOffset -= 5
			if webpageState.ScrollOffset < 0 {
				webpageState.ScrollOffset = 0
			}
		}
	})
	app.Handle("<C-f>", func(m riffkey.Match) {
		switch scenarios[currentScenario] {
		case "scroll":
			scrollState.Offset += 10
			if scrollState.Offset > 90 {
				scrollState.Offset = 90
			}
		case "webpage":
			webpageState.ScrollOffset += 10
			if webpageState.ScrollOffset > 70 {
				webpageState.ScrollOffset = 70
			}
		}
	})
	app.Handle("<C-b>", func(m riffkey.Match) {
		switch scenarios[currentScenario] {
		case "scroll":
			scrollState.Offset -= 10
			if scrollState.Offset < 0 {
				scrollState.Offset = 0
			}
		case "webpage":
			webpageState.ScrollOffset -= 10
			if webpageState.ScrollOffset < 0 {
				webpageState.ScrollOffset = 0
			}
		}
	})
	// Page up/down keys
	app.Handle("<PageDown>", func(m riffkey.Match) {
		if scenarios[currentScenario] == "webpage" {
			webpageState.ScrollOffset += 10
			if webpageState.ScrollOffset > 70 {
				webpageState.ScrollOffset = 70
			}
		}
	})
	app.Handle("<PageUp>", func(m riffkey.Match) {
		if scenarios[currentScenario] == "webpage" {
			webpageState.ScrollOffset -= 10
			if webpageState.ScrollOffset < 0 {
				webpageState.ScrollOffset = 0
			}
		}
	})
	// Home/End
	app.Handle("<Home>", func(m riffkey.Match) {
		if scenarios[currentScenario] == "webpage" {
			webpageState.ScrollOffset = 0
		}
	})
	app.Handle("<End>", func(m riffkey.Match) {
		if scenarios[currentScenario] == "webpage" {
			webpageState.ScrollOffset = 70
		}
	})

	// Update UI definition before each render
	go func() {
		for {
			app.SetUI(renderUI())
			app.RequestRender()
			time.Sleep(100 * time.Millisecond)
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

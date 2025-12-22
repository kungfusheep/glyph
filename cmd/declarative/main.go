package main

import (
	"fmt"
	"log"
	"time"

	"riffkey"
	. "tui"
)

// Application state - pointers into this drive the UI
var state = struct {
	Count       int
	Name        string
	AutoRefresh bool
	DarkMode    bool
	Message     string
	Tab         string
	Tick        int
}{
	Count:       0,
	Name:        "",
	AutoRefresh: false,
	DarkMode:    false,
	Message:     "Welcome! Tab=Navigate, Enter=Activate, q=Quit",
	Tab:         "home",
	Tick:        0,
}

// UI Definition - declarative with pointer bindings
var ui = DCol{
	Gap: 1,
	Children: []any{
		// Header
		DRow{Children: []any{
			DText{Content: "Declarative UI Demo", Bold: true},
			DText{Content: " | "},
			DText{Content: func() string {
				return time.Now().Format("15:04:05")
			}},
		}},

		// Tab buttons
		DRow{Gap: 1, Children: []any{
			DButton{
				Label: "Home",
				OnClick: func() {
					state.Tab = "home"
					state.Message = "Switched to Home tab"
				},
			},
			DButton{
				Label: "Settings",
				OnClick: func() {
					state.Tab = "settings"
					state.Message = "Switched to Settings tab"
				},
			},
			DButton{
				Label: "About",
				OnClick: func() {
					state.Tab = "about"
					state.Message = "Switched to About tab"
				},
			},
		}},

		// Tab content
		DSwitch(&state.Tab,
			DCase("home", homeTab()),
			DCase("settings", settingsTab()),
			DCase("about", aboutTab()),
		),

		// Status bar
		DText{Content: &state.Message},

		// Help
		DText{Content: "Tab/j/k=Navigate  Enter/Space=Activate  q=Quit"},
	},
}

func homeTab() any {
	return DCol{
		Gap: 1,
		Children: []any{
			DText{Content: "--- Home ---", Bold: true},

			// Counter
			DRow{Gap: 1, Children: []any{
				DButton{
					Label: "+",
					OnClick: func() {
						state.Count++
						state.Message = fmt.Sprintf("Count: %d", state.Count)
					},
				},
				DText{Content: func() string {
					return fmt.Sprintf("Count: %d", state.Count)
				}},
				DButton{
					Label: "-",
					OnClick: func() {
						if state.Count > 0 {
							state.Count--
						}
						state.Message = fmt.Sprintf("Count: %d", state.Count)
					},
				},
				DButton{
					Label: "Reset",
					OnClick: func() {
						state.Count = 0
						state.Message = "Counter reset!"
					},
				},
			}},

			// Progress based on count
			DRow{Children: []any{
				DText{Content: "Progress: "},
				DProgress{Value: func() int {
					return (state.Count * 10) % 101
				}, Width: 30},
			}},

			// Tick counter (auto-updates if AutoRefresh is on)
			DIf(&state.AutoRefresh,
				DRow{Children: []any{
					DText{Content: "Auto tick: "},
					DText{Content: func() string {
						return fmt.Sprintf("%d", state.Tick)
					}},
				}},
			),
		},
	}
}

func settingsTab() any {
	return DCol{
		Gap: 1,
		Children: []any{
			DText{Content: "--- Settings ---", Bold: true},

			DRow{Children: []any{
				DText{Content: "Name: "},
				DInput{
					Value: &state.Name,
					Width: 20,
					OnChange: func(v string) {
						state.Message = "Name: " + v
					},
				},
			}},

			DCheckbox{
				Checked: &state.AutoRefresh,
				Label:   "Auto Refresh (shows tick on Home)",
				OnChange: func(checked bool) {
					if checked {
						state.Message = "Auto refresh enabled"
					} else {
						state.Message = "Auto refresh disabled"
					}
				},
			},

			DCheckbox{
				Checked: &state.DarkMode,
				Label:   "Dark Mode (visual only)",
				OnChange: func(checked bool) {
					state.Message = fmt.Sprintf("Dark mode: %v", checked)
				},
			},
		},
	}
}

func aboutTab() any {
	return DCol{
		Children: []any{
			DText{Content: "--- About ---", Bold: true},
			DText{Content: ""},
			DText{Content: "Declarative TUI Framework PoC"},
			DText{Content: ""},
			DText{Content: "Features:"},
			DText{Content: "  * Pointer-bound state"},
			DText{Content: "  * Control flow (If/Else/Switch)"},
			DText{Content: "  * ForEach iteration"},
			DText{Content: "  * Interactive components"},
			DText{Content: "  * Focus management"},
		},
	}
}

func main() {
	app, err := NewDeclApp(ui)
	if err != nil {
		log.Fatal(err)
	}

	// Quit handler
	app.Handle("q", func(m riffkey.Match) {
		app.Stop()
	})
	app.Handle("ctrl+c", func(m riffkey.Match) {
		app.Stop()
	})

	// Auto-refresh ticker
	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			if state.AutoRefresh {
				state.Tick++
				app.RequestRender()
			}
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"fmt"
	"log"

	"riffkey"
	"tui"
)

// Task is our data model for the Observable demo
type Task struct {
	Name     string
	Done     bool
	Selected bool
}

func main() {
	app, err := tui.NewApp()
	if err != nil {
		log.Fatal(err)
	}

	// === PATTERN 1: Generic List[T] ===
	// Direct component manipulation - you manage the components yourself.
	// Good for: static lists, menus, simple layouts.

	staticList := tui.NewList[*tui.TextComponent]()
	staticList.
		Add(
			tui.Text("Static List Items:").Bold().Color(tui.Cyan),
			tui.Text("  Item 1"),
			tui.Text("  Item 2"),
			tui.Text("  Item 3"),
		).
		Gap(0).
		Padding(1).
		Border(tui.BorderRounded)

	// === PATTERN 2: Observable + BoundList ===
	// Data-driven - you manipulate data, UI updates automatically.
	// Good for: dynamic data, selections, anything that changes.

	tasks := tui.NewObservable[Task]()
	tasks.Add(Task{Name: "Build TUI library", Done: true})
	tasks.Add(Task{Name: "Test list patterns", Done: false})
	tasks.Add(Task{Name: "Write documentation", Done: false})
	tasks.Add(Task{Name: "Ship it!", Done: false})

	selectedIdx := 0

	// Create bound list with a dispatcher that has both Create and Update
	taskList := tui.BindWith(tasks, tui.Dispatcher[Task, *tui.TextComponent]{
		Create: func(t Task, idx int) *tui.TextComponent {
			return renderTask(t, idx == selectedIdx)
		},
		Update: func(comp *tui.TextComponent, t Task, idx int) {
			// In-place update instead of recreating
			text := formatTask(t, idx == selectedIdx)
			comp.SetText(text)
			if idx == selectedIdx {
				comp.Style(tui.DefaultStyle().Bold().Foreground(tui.Yellow))
			} else if t.Done {
				comp.Style(tui.DefaultStyle().Dim())
			} else {
				comp.Style(tui.DefaultStyle())
			}
		},
	}).Padding(1).Border(tui.BorderRounded)

	// Status label
	statusLabel := tui.Text("Ready").Color(tui.Green)

	// Build the layout
	root := tui.VStack(
		tui.HStack(
			tui.Text(" List Patterns Demo ").Bold().Inverse(),
			tui.Spacer(),
			tui.Text("q to quit").Dim(),
		),
		tui.FixedSpacer(1),
		tui.HStack(
			// Left: Static List[T]
			tui.VStack(
				tui.Text("Pattern 1: List[T]").Bold().Color(tui.Magenta),
				tui.FixedSpacer(1),
				staticList,
				tui.FixedSpacer(1),
				tui.Text("Direct component management").Dim(),
				tui.Text("i/o: insert/remove items").Dim(),
			),
			tui.FixedSpacer(2),
			// Right: Observable BoundList
			tui.VStack(
				tui.Text("Pattern 2: Observable").Bold().Color(tui.Magenta),
				tui.FixedSpacer(1),
				tui.Text("Tasks:").Bold(),
				taskList,
				tui.FixedSpacer(1),
				tui.Text("Data-driven, auto-updates").Dim(),
				tui.Text("j/k: nav, space: toggle, a: add, x: del").Dim(),
			),
		).Gap(2),
		tui.FixedSpacer(1),
		tui.HStack(
			tui.Text("Status: ").Dim(),
			statusLabel,
		),
	).Padding(1)

	app.SetRoot(root)

	// Helper to update selection styling
	updateSelection := func(oldIdx, newIdx int) {
		if oldIdx >= 0 && oldIdx < tasks.Len() {
			tasks.Update(oldIdx, func(t *Task) { t.Selected = false })
		}
		if newIdx >= 0 && newIdx < tasks.Len() {
			tasks.Update(newIdx, func(t *Task) { t.Selected = true })
		}
	}

	// Initialize selection
	if tasks.Len() > 0 {
		tasks.Update(0, func(t *Task) { t.Selected = true })
	}

	// Key bindings
	app.Handle("q", func(m riffkey.Match) {
		app.Stop()
	})
	app.Handle("<C-c>", func(m riffkey.Match) {
		app.Stop()
	})

	// Pattern 1: Static list manipulation
	itemCount := 4
	app.Handle("i", func(m riffkey.Match) {
		itemCount++
		staticList.Add(tui.Text(fmt.Sprintf("  Item %d", itemCount)))
		statusLabel.SetText(fmt.Sprintf("Added item %d to static list", itemCount)).Color(tui.Green)
	})
	app.Handle("o", func(m riffkey.Match) {
		if staticList.Len() > 1 {
			staticList.RemoveAt(staticList.Len() - 1)
			statusLabel.SetText("Removed item from static list").Color(tui.Red)
		}
	})

	// Pattern 2: Observable list manipulation
	app.Handle("j", func(m riffkey.Match) {
		if selectedIdx < tasks.Len()-1 {
			oldIdx := selectedIdx
			selectedIdx++
			updateSelection(oldIdx, selectedIdx)
			statusLabel.SetText(fmt.Sprintf("Selected: %s", tasks.At(selectedIdx).Name)).Color(tui.Yellow)
		}
	})
	app.Handle("k", func(m riffkey.Match) {
		if selectedIdx > 0 {
			oldIdx := selectedIdx
			selectedIdx--
			updateSelection(oldIdx, selectedIdx)
			statusLabel.SetText(fmt.Sprintf("Selected: %s", tasks.At(selectedIdx).Name)).Color(tui.Yellow)
		}
	})
	app.Handle("<Down>", func(m riffkey.Match) {
		if selectedIdx < tasks.Len()-1 {
			oldIdx := selectedIdx
			selectedIdx++
			updateSelection(oldIdx, selectedIdx)
		}
	})
	app.Handle("<Up>", func(m riffkey.Match) {
		if selectedIdx > 0 {
			oldIdx := selectedIdx
			selectedIdx--
			updateSelection(oldIdx, selectedIdx)
		}
	})
	app.Handle(" ", func(m riffkey.Match) {
		if selectedIdx >= 0 && selectedIdx < tasks.Len() {
			tasks.Update(selectedIdx, func(t *Task) {
				t.Done = !t.Done
			})
			task := tasks.At(selectedIdx)
			if task.Done {
				statusLabel.SetText(fmt.Sprintf("Completed: %s", task.Name)).Color(tui.Green)
			} else {
				statusLabel.SetText(fmt.Sprintf("Uncompleted: %s", task.Name)).Color(tui.Yellow)
			}
		}
	})
	taskNum := 5
	app.Handle("a", func(m riffkey.Match) {
		tasks.Add(Task{Name: fmt.Sprintf("New task %d", taskNum), Done: false})
		taskNum++
		statusLabel.SetText("Added new task").Color(tui.Green)
	})
	app.Handle("x", func(m riffkey.Match) {
		if tasks.Len() > 0 && selectedIdx < tasks.Len() {
			name := tasks.At(selectedIdx).Name
			tasks.RemoveAt(selectedIdx)
			if selectedIdx >= tasks.Len() && selectedIdx > 0 {
				selectedIdx--
			}
			if tasks.Len() > 0 {
				updateSelection(-1, selectedIdx)
			}
			statusLabel.SetText(fmt.Sprintf("Deleted: %s", name)).Color(tui.Red)
		}
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func formatTask(t Task, selected bool) string {
	prefix := "  "
	if selected {
		prefix = "> "
	}
	check := "[ ]"
	if t.Done {
		check = "[x]"
	}
	return fmt.Sprintf("%s%s %s", prefix, check, t.Name)
}

func renderTask(t Task, selected bool) *tui.TextComponent {
	text := formatTask(t, selected)
	tc := tui.Text(text)
	if selected {
		tc.Style(tui.DefaultStyle().Bold().Foreground(tui.Yellow))
	} else if t.Done {
		tc.Style(tui.DefaultStyle().Dim())
	}
	return tc
}

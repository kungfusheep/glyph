package main

import (
	"fmt"
	"log"
	"time"

	"riffkey"
	"tui"
)

func main() {
	app, err := tui.NewApp()
	if err != nil {
		log.Fatal(err)
	}

	// Widget 1: Counter
	counter := tui.NewWidget(app, CounterData{Count: 0, Step: 1},
		func(d *CounterData, ctx *tui.Context) tui.Component {
			return tui.VStack(
				tui.Text("Counter").Bold().Color(tui.Cyan),
				tui.FixedSpacer(1),
				tui.Text(fmt.Sprintf("Count: %d", d.Count)),
				tui.Text(fmt.Sprintf("Step:  %d", d.Step)).Dim(),
				tui.FixedSpacer(1),
				tui.Text("+/-: count, [/]: step").Dim(),
			).Padding(1).Border(tui.BorderRounded)
		},
	)

	// Widget 2: Todo List
	todo := tui.NewWidget(app, TodoData{
		Items:    []string{"Build TUI library", "Test widgets", "Integrate riffkey", "Profit"},
		Selected: 0,
	},
		func(d *TodoData, ctx *tui.Context) tui.Component {
			items := make([]tui.ChildItem, len(d.Items))
			for i, item := range d.Items {
				prefix := "  "
				style := tui.DefaultStyle()
				if i == d.Selected {
					prefix = "> "
					style = style.Bold().Foreground(tui.Green)
				}
				items[i] = tui.Text(prefix + item).Style(style)
			}

			return tui.VStack(
				append([]tui.ChildItem{
					tui.Text("Todo List").Bold().Color(tui.Yellow),
					tui.FixedSpacer(1),
				}, items...)...,
			).Padding(1).Border(tui.BorderRounded)
		},
	)

	// Widget 3: Clock (auto-updating)
	clock := tui.NewWidget(app, ClockData{Time: time.Now()},
		func(d *ClockData, ctx *tui.Context) tui.Component {
			return tui.VStack(
				tui.Text("Clock").Bold().Color(tui.Magenta),
				tui.FixedSpacer(1),
				tui.Text(d.Time.Format("15:04:05")),
				tui.Text(d.Time.Format("2006-01-02")).Dim(),
			).Padding(1).Border(tui.BorderRounded)
		},
	)

	// Widget 4: Key log
	keylog := tui.NewWidget(app, KeyLogData{Keys: []string{}},
		func(d *KeyLogData, ctx *tui.Context) tui.Component {
			items := make([]tui.ChildItem, 0, len(d.Keys))
			start := len(d.Keys) - 8
			if start < 0 {
				start = 0
			}
			for _, k := range d.Keys[start:] {
				items = append(items, tui.Text("  "+k).Dim())
			}

			return tui.VStack(
				append([]tui.ChildItem{
					tui.Text("Key Log").Bold().Color(tui.Blue),
					tui.FixedSpacer(1),
				}, items...)...,
			).Padding(1).Border(tui.BorderRounded)
		},
	)

	// Start clock ticker
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			clock.Update(func(d *ClockData) {
				d.Time = time.Now()
			})
		}
	}()

	// Build the layout
	root := tui.VStack(
		tui.HStack(
			tui.Text(" Widget Demo (riffkey) ").Bold().Inverse(),
			tui.Spacer(),
			tui.Text("q to quit").Dim(),
		),
		tui.FixedSpacer(1),
		tui.HStack(
			tui.WrapWidget(counter),
			tui.WrapWidget(clock),
		).Gap(1),
		tui.HStack(
			tui.WrapWidget(todo),
			tui.WrapWidget(keylog),
		).Gap(1),
	).Padding(1)

	app.SetRoot(root)

	// Helper to log keys
	logKey := func(desc string) {
		keylog.Update(func(d *KeyLogData) {
			d.Keys = append(d.Keys, desc)
		})
	}

	// --- riffkey bindings! ---

	// Quit
	app.Handle("q", func(m riffkey.Match) {
		app.Stop()
	})
	app.Handle("<C-c>", func(m riffkey.Match) {
		app.Stop()
	})

	// Counter controls
	app.Handle("+", func(m riffkey.Match) {
		counter.Update(func(d *CounterData) { d.Count += d.Step })
		logKey("+")
	})
	app.Handle("=", func(m riffkey.Match) {
		counter.Update(func(d *CounterData) { d.Count += d.Step })
		logKey("=")
	})
	app.Handle("-", func(m riffkey.Match) {
		counter.Update(func(d *CounterData) { d.Count -= d.Step })
		logKey("-")
	})
	app.Handle("]", func(m riffkey.Match) {
		counter.Update(func(d *CounterData) { d.Step++ })
		logKey("]")
	})
	app.Handle("[", func(m riffkey.Match) {
		counter.Update(func(d *CounterData) {
			if d.Step > 1 {
				d.Step--
			}
		})
		logKey("[")
	})

	// Todo navigation - vim style!
	app.Handle("j", func(m riffkey.Match) {
		todo.Update(func(d *TodoData) {
			if d.Selected < len(d.Items)-1 {
				d.Selected++
			}
		})
		logKey("j")
	})
	app.Handle("k", func(m riffkey.Match) {
		todo.Update(func(d *TodoData) {
			if d.Selected > 0 {
				d.Selected--
			}
		})
		logKey("k")
	})

	// Arrow keys too
	app.Handle("<Down>", func(m riffkey.Match) {
		todo.Update(func(d *TodoData) {
			if d.Selected < len(d.Items)-1 {
				d.Selected++
			}
		})
		logKey("<Down>")
	})
	app.Handle("<Up>", func(m riffkey.Match) {
		todo.Update(func(d *TodoData) {
			if d.Selected > 0 {
				d.Selected--
			}
		})
		logKey("<Up>")
	})

	// Vim-style jump to top/bottom
	app.Handle("gg", func(m riffkey.Match) {
		todo.Update(func(d *TodoData) { d.Selected = 0 })
		logKey("gg")
	})
	app.Handle("G", func(m riffkey.Match) {
		todo.Update(func(d *TodoData) { d.Selected = len(d.Items) - 1 })
		logKey("G")
	})

	// Delete with count prefix!
	app.Handle("x", func(m riffkey.Match) {
		todo.Update(func(d *TodoData) {
			if len(d.Items) > 0 {
				d.Items = append(d.Items[:d.Selected], d.Items[d.Selected+1:]...)
				if d.Selected >= len(d.Items) && d.Selected > 0 {
					d.Selected--
				}
			}
		})
		logKey(fmt.Sprintf("x (count=%d)", m.Count))
	})

	// Add item
	app.Handle("a", func(m riffkey.Match) {
		todo.Update(func(d *TodoData) {
			d.Items = append(d.Items, fmt.Sprintf("New item %d", len(d.Items)+1))
		})
		logKey("a")
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// Data types for widgets

type CounterData struct {
	Count int
	Step  int
}

type TodoData struct {
	Items    []string
	Selected int
}

type ClockData struct {
	Time time.Time
}

type KeyLogData struct {
	Keys []string
}

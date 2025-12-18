package tui

import (
	"fmt"
	"testing"
)

func TestArenaVisualDashboard(t *testing.T) {
	frame := NewFrame(1000, 10000)
	buf := NewBuffer(80, 24)

	tick := 42

	frame.Build(func() {
		cpuRows := make([]NodeRef, 4)
		for i := 0; i < 4; i++ {
			cpuRows[i] = AHStack(
				AText("Core "),
				ATextInt(i),
				AText(": "),
				AProgress((tick+i*25)%100, 100),
			)
		}

		AVStack(
			// Header
			AHStack(
				AText("Arena Dashboard").Bold(),
				ASpacer(),
				AText("12:34:56"),
			),
			AText("build:50µs layout:20µs render:15µs flush:5µs"),
			// CPU section
			AVStack(append([]NodeRef{AText("CPU Usage").Bold()}, cpuRows...)...),
			ASpacer(),
			// Footer
			AHStack(
				AText("Tick: "),
				ATextInt(tick),
				ASpacer(),
				AText("Zero allocs!"),
			),
		)
	})

	frame.Layout(80, 24)
	frame.Render(buf)

	fmt.Println("=== Arena Dashboard ===")
	fmt.Println(buf.StringTrimmed())
	fmt.Println("=======================")
	fmt.Printf("Nodes: %d\n", len(frame.Nodes()))
}

func TestArenaVisualList(t *testing.T) {
	frame := NewFrame(1000, 10000)
	buf := NewBuffer(60, 20)

	frame.Build(func() {
		items := make([]NodeRef, 10)
		for i := 0; i < 10; i++ {
			items[i] = AHStack(
				ATextInt(i+1),
				AText(". Item number "),
				ATextInt(i),
				ASpacer(),
				AProgress(i*10, 100),
			)
		}

		AVStack(append([]NodeRef{AText("Item List").Bold()}, items...)...)
	})

	frame.Layout(60, 20)
	frame.Render(buf)

	fmt.Println("=== Arena List ===")
	fmt.Println(buf.StringTrimmed())
	fmt.Println("==================")
}

func TestArenaVisualGrid(t *testing.T) {
	frame := NewFrame(1000, 10000)
	buf := NewBuffer(80, 15)

	frame.Build(func() {
		rows := make([]NodeRef, 5)
		for row := 0; row < 5; row++ {
			cols := make([]NodeRef, 6)
			for col := 0; col < 3; col++ {
				idx := row*3 + col
				cols[col*2] = AProgress(idx*7%100, 100)
				cols[col*2+1] = AText(" ")
			}
			rows[row] = AHStack(cols...)
		}

		AVStack(append([]NodeRef{AText("Progress Grid").Bold()}, rows...)...)
	})

	frame.Layout(80, 15)
	frame.Render(buf)

	fmt.Println("=== Arena Grid ===")
	fmt.Println(buf.StringTrimmed())
	fmt.Println("==================")
}

func TestArenaNodeSize(t *testing.T) {
	var n Node
	fmt.Printf("Node size: ~%d bytes (approx)\n", 48)
	fmt.Printf("Node fields: Kind=%d Parent=%d FirstChild=%d LastChild=%d NextSib=%d\n",
		n.Kind, n.Parent, n.FirstChild, n.LastChild, n.NextSib)
}

package tui

import (
	"fmt"
	"testing"
)

func TestArenaDebugLayout(t *testing.T) {
	frame := NewFrame(100, 1000)
	buf := NewBuffer(40, 5)

	frame.Build(func() {
		AHStack(
			AText("Left"),
			ASpacer(),
			AText("Right"),
		)
	})

	// Debug: print node tree
	fmt.Println("=== Node Tree ===")
	for i, n := range frame.Nodes() {
		kind := []string{"Text", "VStack", "HStack", "Spacer", "Progress"}[n.Kind]
		text := ""
		if n.Kind == NodeText {
			text = fmt.Sprintf(" %q", frame.GetText(&n))
		}
		fmt.Printf("[%d] %s parent=%d first=%d next=%d W=%d H=%d flex=%.1f%s\n",
			i, kind, n.Parent, n.FirstChild, n.NextSib, n.W, n.H, n.FlexGrow, text)
	}

	frame.Layout(40, 1)

	fmt.Println("\n=== After Layout ===")
	for i, n := range frame.Nodes() {
		kind := []string{"Text", "VStack", "HStack", "Spacer", "Progress"}[n.Kind]
		fmt.Printf("[%d] %s x=%d y=%d w=%d h=%d\n", i, kind, n.X, n.Y, n.W, n.H)
	}

	frame.Render(buf)
	fmt.Println("\n=== Rendered ===")
	fmt.Printf("|%s|\n", buf.String()[:40])
}

func TestArenaDebugVStack(t *testing.T) {
	frame := NewFrame(100, 1000)
	buf := NewBuffer(20, 5)

	frame.Build(func() {
		AVStack(
			AText("Line 1"),
			AText("Line 2"),
			AText("Line 3"),
		)
	})

	fmt.Println("=== VStack Node Tree ===")
	for i, n := range frame.Nodes() {
		kind := []string{"Text", "VStack", "HStack", "Spacer", "Progress"}[n.Kind]
		text := ""
		if n.Kind == NodeText {
			text = fmt.Sprintf(" %q", frame.GetText(&n))
		}
		fmt.Printf("[%d] %s parent=%d first=%d next=%d W=%d H=%d%s\n",
			i, kind, n.Parent, n.FirstChild, n.NextSib, n.W, n.H, text)
	}

	frame.Layout(20, 5)

	fmt.Println("\n=== After Layout ===")
	for i, n := range frame.Nodes() {
		kind := []string{"Text", "VStack", "HStack", "Spacer", "Progress"}[n.Kind]
		fmt.Printf("[%d] %s x=%d y=%d w=%d h=%d\n", i, kind, n.X, n.Y, n.W, n.H)
	}

	frame.Render(buf)
	fmt.Println("\n=== Rendered ===")
	fmt.Println(buf.StringTrimmed())
}

func TestArenaDebugTwoColumn(t *testing.T) {
	frame := NewFrame(100, 1000)
	buf := NewBuffer(40, 8)

	frame.Build(func() {
		AVStack(
			AText("Header"),
			AHStack(
				AVStack(
					AText("Left 1"),
					AText("Left 2"),
				).Grow(1),
				AVStack(
					AText("Right 1"),
					AText("Right 2"),
				).Grow(1),
			).Grow(1),
		)
	})

	frame.Layout(40, 8)

	fmt.Println("=== Two Column Layout ===")
	for i, n := range frame.Nodes() {
		kind := []string{"Text", "VStack", "HStack", "Spacer", "Progress"}[n.Kind]
		text := ""
		if n.Kind == NodeText {
			text = fmt.Sprintf(" %q", frame.GetText(&n))
		}
		fmt.Printf("[%d] %s x=%d y=%d w=%d h=%d flex=%.1f%s\n",
			i, kind, n.X, n.Y, n.W, n.H, n.FlexGrow, text)
	}

	frame.Render(buf)
	fmt.Println("\n=== Rendered ===")
	fmt.Println(buf.StringTrimmed())
}

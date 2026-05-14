package main

import (
	"fmt"
	"log"

	. "github.com/kungfusheep/glyph"
)

func main() {
	app := NewApp()

	active := "files"
	files := []string{"README.md", "template.go", "app.go", "components.go", "on_test.go"}
	fileIndex := 0
	previewLines := []string{
		"declarative On(...) handlers render no cells.",
		"place them inside If/Switch/Match branches.",
		"glyph builds scoped riffkey routers once.",
		"frame-time work only toggles enabled routers.",
		"same keys can belong to different active panes.",
	}
	previewTop := 0

	nextPane := func() {
		if active == "files" {
			active = "preview"
			return
		}
		active = "files"
	}
	filesDown := func() {
		if fileIndex < len(files)-1 {
			fileIndex++
		}
	}
	filesUp := func() {
		if fileIndex > 0 {
			fileIndex--
		}
	}
	previewDown := func() {
		if previewTop < len(previewLines)-1 {
			previewTop++
		}
	}
	previewUp := func() {
		if previewTop > 0 {
			previewTop--
		}
	}

	app.SetView(
		VBox.Gap(1)(
			Text("tab switches panes; j/k route through the active pane").Dim(),
			HBox.Gap(1)(
				VBox.WidthPct(0.4).
					Border(BorderRounded).
					BorderFG(If(&active).Eq("files").Then(Cyan).Else(BrightBlack)).
					Title("Files")(
					ForEach(&files, func(name *string) Component {
						return Text(name)
					}),
					Text(func() string {
						return fmt.Sprintf("selected: %s", files[fileIndex])
					}).FG(Cyan),
					If(&active).Eq("files").Then(
						On(
							Key("j", filesDown),
							Key("k", filesUp),
						),
					),
				),
				VBox.Grow(1).
					Border(BorderRounded).
					BorderFG(If(&active).Eq("preview").Then(Green).Else(BrightBlack)).
					Title("Preview")(
					ForEach(&previewLines, func(line *string) Component {
						return Text(line).Dim()
					}),
					Text(func() string {
						return fmt.Sprintf("top line: %d", previewTop+1)
					}).FG(Green),
					If(&active).Eq("preview").Then(
						On(
							Key("j", previewDown),
							Key("k", previewUp),
						),
					),
				),
			),
			Text("q quits").Dim(),
			On(
				Key("<Tab>", nextPane),
				Key("q", app.Stop),
			),
		),
	)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// inline list picker — like gum choose / fzf
package main

import (
	"fmt"

	. "github.com/kungfusheep/glyph"
)

func main() {
	options := []string{"production", "staging", "development", "local"}
	cancelled := false
	var list *ListC[string]

	app := NewInlineApp()

	app.ClearOnExit(true).
		SetView(VBox.FitContent()(
			Text("Deploy target:").FG(Cyan).Bold(),
			List(&options).
				Render(func(s *string) Component { return Text(s) }).
				BindNav("j", "k").
				MarkerStyle(Style{FG: Green}).
				Ref(func(l *ListC[string]) { list = l }),
		)).
		Handle("<Enter>", app.Stop).
		Handle("<Escape>", func() { cancelled = true; app.Stop() }).
		Run()

	if !cancelled {
		if sel := list.Selected(); sel != nil {
			fmt.Println(*sel)
		}
	}
}

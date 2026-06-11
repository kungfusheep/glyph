package glyph

import (
	"testing"

	"github.com/kungfusheep/riffkey"
)

func TestFormTriggerJoinsFocusCycleAndActivates(t *testing.T) {
	var name string
	var activated int
	var submitted bool
	var focused bool
	label := "work"

	form := Form.OnSubmit(func(f *FormC) { submitted = true })(
		Field("Name", Input(&name)),
		Field("Calendar", Trigger(Text(&label)).
			OnActivate(func() { activated++ }).
			BindFocus(&focused)),
	)
	app := NewApp()
	app.SetView(VBox(form))
	app.RenderNow()

	if focused {
		t.Fatal("trigger focused before tabbing to it")
	}
	if !app.Input().Dispatch(riffkey.Key{Special: riffkey.SpecialTab}) {
		t.Fatal("tab was not handled")
	}
	if !focused {
		t.Fatal("trigger did not receive focus from tab cycle")
	}

	if !app.Input().Dispatch(riffkey.Key{Special: riffkey.SpecialEnter}) {
		t.Fatal("enter on focused trigger was not handled")
	}
	if activated != 1 {
		t.Fatalf("activated = %d, want 1 (enter)", activated)
	}
	if submitted {
		t.Fatal("enter on focused trigger submitted the form")
	}

	if !app.Input().Dispatch(riffkey.Key{Special: riffkey.SpecialSpace}) {
		t.Fatal("space on focused trigger was not handled")
	}
	if activated != 2 {
		t.Fatalf("activated = %d, want 2 (space)", activated)
	}

	// tab away: focus leaves the trigger and enter submits again
	if !app.Input().Dispatch(riffkey.Key{Special: riffkey.SpecialTab}) {
		t.Fatal("tab away was not handled")
	}
	if focused {
		t.Fatal("trigger still focused after tabbing away")
	}
	if !app.Input().Dispatch(riffkey.Key{Special: riffkey.SpecialEnter}) {
		t.Fatal("enter on input was not handled")
	}
	if !submitted {
		t.Fatal("enter away from trigger did not submit the form")
	}
	if activated != 2 {
		t.Fatalf("activated = %d, want 2 (no activation after blur)", activated)
	}
}

package glyph_test

import (
	"testing"

	. "github.com/kungfusheep/glyph"
	"github.com/kungfusheep/riffkey"
)

func TestInputBindsToString(t *testing.T) {
	name := ""
	input := Input(&name).Placeholder("name")

	// simulate typing by setting value directly
	input.SetValue("pete")
	// bound value should not update from SetValue (only from keystroke onChange)
	// but Value() should work
	if input.Value() != "pete" {
		t.Errorf("expected Value() = 'pete', got %q", input.Value())
	}
}

func TestInputValidateRequired(t *testing.T) {
	name := ""
	input := Input(&name).Validate(VRequired, VOnChange)

	// trigger validation manually
	input.SetValue("")
	// runValidation is unexported, but we can check via Err after construction
	// since VOnChange would fire from handleChange, let's test the validator directly
	if err := VRequired(""); err == nil {
		t.Error("VRequired should reject empty string")
	}
	if err := VRequired("hello"); err != nil {
		t.Errorf("VRequired should accept 'hello', got %v", err)
	}
}

func TestVEmailValidator(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"", false},
		{"user@example.com", true},
		{"user@sub.example.com", true},
		{"bad", false},
		{"@example.com", false},
		{"user@", false},
		{"user@example.", false},
	}
	for _, tt := range tests {
		err := VEmail(tt.input)
		if tt.valid && err != nil {
			t.Errorf("VEmail(%q) should be valid, got %v", tt.input, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("VEmail(%q) should be invalid", tt.input)
		}
	}
}

func TestVMinMaxLen(t *testing.T) {
	min3 := VMinLen(3)
	if err := min3("ab"); err == nil {
		t.Error("VMinLen(3) should reject 'ab'")
	}
	if err := min3("abc"); err != nil {
		t.Errorf("VMinLen(3) should accept 'abc', got %v", err)
	}

	max5 := VMaxLen(5)
	if err := max5("hello"); err != nil {
		t.Errorf("VMaxLen(5) should accept 'hello', got %v", err)
	}
	if err := max5("toolong"); err == nil {
		t.Error("VMaxLen(5) should reject 'toolong'")
	}
}

func TestVTrue(t *testing.T) {
	if err := VTrue(true); err != nil {
		t.Errorf("VTrue(true) should pass, got %v", err)
	}
	if err := VTrue(false); err == nil {
		t.Error("VTrue(false) should fail")
	}
}

func TestCheckboxValidate(t *testing.T) {
	agreed := false
	cb := Checkbox(&agreed, "I agree").Validate(VTrue, VOnChange)

	// toggle to true
	cb.Toggle()
	if cb.Err() != "" {
		t.Errorf("expected no error after toggling to true, got %q", cb.Err())
	}

	// toggle back to false
	cb.Toggle()
	if cb.Err() == "" {
		t.Error("expected error after toggling to false")
	}
}

func TestFormCompilesWithValidation(t *testing.T) {
	name := ""
	email := ""
	role := 0
	agree := false

	_ = Form.LabelBold()(
		Field("Name", Input(&name).Placeholder("name").Validate(VRequired, VOnBlur)),
		Field("Email", Input(&email).Placeholder("email").Validate(VEmail, VOnBlur)),
		Field("Role", Radio(&role, "Admin", "User", "Guest")),
		Field("Terms", Checkbox(&agree, "I accept").Validate(VTrue, VOnSubmit)),
	)
}

func TestFormInputFieldStateReceivesManagedTypingAndValidation(t *testing.T) {
	var name string
	field := InputState{Value: "prefill", Cursor: len("prefill")}
	var submitted bool
	form := Form.LabelBold().OnSubmit(func(f *FormC) {
		submitted = true
	})(
		Field("Name", Input(&name).Field(&field).Validate(VRequired, VOnSubmit)),
	)
	app := NewApp()
	app.SetView(VBox(form))
	app.RenderNow()

	if !app.Input().Dispatch(riffkey.Key{Rune: 'x'}) {
		t.Fatal("form input did not handle typed key")
	}
	if field.Value != "prefillx" || name != "prefillx" {
		t.Fatalf("field/name = %q/%q, want typed external state and bound value", field.Value, name)
	}
	field.Clear()
	if !app.Input().Dispatch(riffkey.Key{Special: riffkey.SpecialEnter}) {
		t.Fatal("form submit was not handled")
	}
	if submitted {
		t.Fatal("empty external field passed required validation")
	}
}

func TestFormSubmitValidatesBeforeCallback(t *testing.T) {
	var name string
	var submitted bool
	form := Form.OnSubmit(func(f *FormC) {
		submitted = true
	})(
		Field("Name", Input(&name).Validate(VRequired, VOnSubmit)),
	)
	app := NewApp()
	app.SetView(VBox(form))
	app.RenderNow()

	if !app.Input().Dispatch(riffkey.Key{Special: riffkey.SpecialEnter}) {
		t.Fatal("form submit was not handled")
	}
	if submitted {
		t.Fatal("submit callback fired for invalid form")
	}
	for _, r := range "ok" {
		if !app.Input().Dispatch(riffkey.Key{Rune: r}) {
			t.Fatalf("typing %q into form was not handled", r)
		}
	}
	if !app.Input().Dispatch(riffkey.Key{Special: riffkey.SpecialEnter}) {
		t.Fatal("valid form submit was not handled")
	}
	if !submitted {
		t.Fatal("submit callback did not fire for valid form")
	}
}

func TestFormInputBoundValueChangeUpdatesEditableState(t *testing.T) {
	var name string
	form := Form(
		Field("Name", Input(&name)),
	)
	app := NewApp()
	app.SetView(VBox(form))
	app.RenderNow()

	name = "prefill"
	app.RenderNow()
	if !app.Input().Dispatch(riffkey.Key{Rune: 'x'}) {
		t.Fatal("form input did not handle typed key")
	}
	if name != "prefillx" {
		t.Fatalf("name = %q, want typing to append to programmatic bound value", name)
	}
}

package glyph

// TriggerC is a focusable form control that wraps display content and fires
// an action when activated. Inside a Form it joins the Tab cycle like Input
// or Checkbox; while focused, Enter and Space call the OnActivate handler
// (Enter overrides form submit for this field only). Use it for fields whose
// value is chosen elsewhere — "press enter to choose" pickers, dialogs, etc.
//
// usage:
//
//	open := func() { pickerOpen = true }
//	Form(
//	    Field("name", Input(&name)),
//	    Field("calendar", Trigger(Text(&calendarLabel)).OnActivate(open)),
//	)
type TriggerC struct {
	child      Component
	onActivate func()
	focusPtr   *bool
	focused    bool
}

// Trigger creates a focusable control around any display component.
func Trigger(child Component) *TriggerC {
	return &TriggerC{child: child}
}

// OnActivate sets the handler fired when the focused trigger is activated.
func (tr *TriggerC) OnActivate(fn func()) *TriggerC {
	tr.onActivate = fn
	return tr
}

// BindFocus exposes the focus state to the template, e.g. for an
// If(&focused) hint next to the wrapped content.
func (tr *TriggerC) BindFocus(ptr *bool) *TriggerC {
	tr.focusPtr = ptr
	return tr
}

// Activate fires the OnActivate handler.
func (tr *TriggerC) Activate() {
	if tr.onActivate != nil {
		tr.onActivate()
	}
}

// focusBinding implements focusable. Trigger has no text input.
func (tr *TriggerC) focusBinding() *textInputBinding { return nil }

// setFocused implements focusable.
func (tr *TriggerC) setFocused(focused bool) {
	tr.focused = focused
	if tr.focusPtr != nil {
		*tr.focusPtr = focused
	}
}

// toTemplate implements templateTree — the trigger renders as its child.
func (tr *TriggerC) toTemplate() Component { return tr.child }

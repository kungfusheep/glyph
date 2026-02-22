package forme

// validatable is implemented by controls that support validation.
type validatable interface {
	Err() string
	runValidation()
}

// FormField pairs a label with an input control.
type FormField struct {
	label   string
	control any
	err     string // validation error for this field
	focused bool
}

// Field creates a form field pairing a label with any control component.
func Field(label string, control any) FormField {
	return FormField{label: label, control: control}
}

// FormC is a higher-order form component that arranges labeled fields
// in a vertical layout with aligned labels and automatic focus management.
//
// usage:
//
//	Form(
//	    Field("Name", Input().Placeholder("Enter your name")),
//	    Field("Email", Input().Placeholder("you@example.com")),
//	    Field("Password", Input().Placeholder("password").Mask('*')),
//	).Gap(1).LabelFG(BrightWhite)
type FormC struct {
	fields     []FormField
	fm         *FocusManager
	gap        int8
	labelWidth int16
	labelStyle Style
	grow       float32
	margin     [4]int16
	onSubmit   func()
}

// Form creates a form from labeled fields.
// Automatically creates a FocusManager and wires any focusable controls.
func Form(fields ...FormField) *FormC {
	f := &FormC{
		fields: fields,
		fm:     NewFocusManager(),
	}

	// auto-calculate label width from longest label + colon
	for _, ff := range fields {
		w := int16(len(ff.label) + 1) // +1 for ":"
		if w > f.labelWidth {
			f.labelWidth = w
		}
	}

	// auto-wire focusable controls and blur validation
	var focusableFields []*FormField // maps FM index → FormField
	for idx := range fields {
		ff := &f.fields[idx]
		if fc, ok := ff.control.(focusable); ok {
			fieldRef := ff
			focusableFields = append(focusableFields, fieldRef)
			switch ctrl := ff.control.(type) {
			case *InputC:
				ctrl.ManagedBy(f.fm)
				ctrl.onBlur = func() {
					fieldRef.err = ctrl.Err()
				}
			case *CheckboxC:
				f.fm.Register(fc)
				ctrl.onBlur = func() {
					fieldRef.err = ctrl.Err()
				}
				f.fm.ItemBindings(
					binding{pattern: "<Space>", handler: func() { ctrl.Toggle() }},
				)
			case *RadioC:
				f.fm.Register(fc)
				f.fm.ItemBindings(
					binding{pattern: "j", handler: func() { ctrl.Next() }},
					binding{pattern: "k", handler: func() { ctrl.Prev() }},
				)
			default:
				f.fm.Register(fc)
			}
		}
	}

	// first focusable field starts focused
	if len(focusableFields) > 0 {
		focusableFields[0].focused = true
	}

	// track focus changes to update visual indicator
	f.fm.OnChange(func(idx int) {
		for i, ff := range focusableFields {
			ff.focused = (i == idx)
		}
	})
	f.fm.OnBlur(func() {
		for _, ff := range focusableFields {
			ff.focused = false
		}
	})

	return f
}

// Gap sets the vertical gap between fields.
func (f *FormC) Gap(g int8) *FormC {
	f.gap = g
	return f
}

// LabelStyle sets the full style for all labels.
func (f *FormC) LabelStyle(s Style) *FormC {
	f.labelStyle = s
	return f
}

// LabelFG sets the foreground color for all labels.
func (f *FormC) LabelFG(c Color) *FormC {
	f.labelStyle.FG = c
	return f
}

// LabelBold sets labels to bold.
func (f *FormC) LabelBold() *FormC {
	f.labelStyle = f.labelStyle.Bold()
	return f
}

// NextKey sets the key for advancing focus (default: Tab).
func (f *FormC) NextKey(key string) *FormC {
	f.fm.NextKey(key)
	return f
}

// PrevKey sets the key for reversing focus (default: Shift-Tab).
func (f *FormC) PrevKey(key string) *FormC {
	f.fm.PrevKey(key)
	return f
}

// OnFocusChange sets a callback that fires when focus changes.
func (f *FormC) OnFocusChange(fn func(index int)) *FormC {
	f.fm.OnChange(fn)
	return f
}

// OnSubmit sets a callback that fires when Enter is pressed.
// Useful for wiring form submission without a separate app-level binding.
func (f *FormC) OnSubmit(fn func()) *FormC {
	f.onSubmit = fn
	return f
}

// Grow sets the flex grow factor.
func (f *FormC) Grow(g float32) *FormC {
	f.grow = g
	return f
}

// Margin sets equal margin on all sides.
func (f *FormC) Margin(m int16) *FormC {
	f.margin = [4]int16{m, m, m, m}
	return f
}

// MarginVH sets vertical and horizontal margin.
func (f *FormC) MarginVH(v, h int16) *FormC {
	f.margin = [4]int16{v, h, v, h}
	return f
}

// MarginTRBL sets top, right, bottom, left margin individually.
func (f *FormC) MarginTRBL(t, r, b, l int16) *FormC {
	f.margin = [4]int16{t, r, b, l}
	return f
}

// FocusManager returns the internal focus manager for external wiring.
func (f *FormC) FocusManager() *FocusManager {
	return f.fm
}

// ValidateAll runs validation on all fields that have VOnSubmit set.
// Returns true if all fields are valid.
func (f *FormC) ValidateAll() bool {
	valid := true
	for i := range f.fields {
		ff := &f.fields[i]
		if v, ok := ff.control.(validatable); ok {
			v.runValidation()
			ff.err = v.Err()
			if ff.err != "" {
				valid = false
			}
		}
	}
	return valid
}

// toTemplate builds the VBox of HBox rows with optional error display.
func (f *FormC) toTemplate() any {
	rows := make([]any, 0, len(f.fields)*2)
	for i := range f.fields {
		ff := &f.fields[i]
		ls := f.labelStyle
		ls.Align = AlignRight
		ls = ls.MarginTRBL(0, 1, 0, 0)

		label := Text(ff.label + ":").Width(f.labelWidth).Style(ls)
		indicator := If(&ff.focused).
			Then(Text("▸").Width(1)).
			Else(Text("").Width(1))
		rows = append(rows, HBox(indicator, label, ff.control))

		// add error display if the control supports validation
		if _, ok := ff.control.(validatable); ok {
			spacer := Text("").Width(f.labelWidth+2).MarginTRBL(0, 1, 0, 0)
			rows = append(rows, If(&ff.err).Then(
				HBox(spacer, Text(&ff.err).FG(Red)),
			))
		}
	}

	box := VBox.Gap(f.gap)
	if f.grow > 0 {
		box = box.Grow(f.grow)
	}
	if f.margin != [4]int16{} {
		box = box.MarginTRBL(f.margin[0], f.margin[1], f.margin[2], f.margin[3])
	}
	return box(rows...)
}

// bindings returns Form-specific bindings only.
// Tab/Shift-Tab are handled by the FocusManager in wireBindings.
func (f *FormC) bindings() []binding {
	if f.onSubmit != nil {
		enterBinding := binding{pattern: "<Enter>", handler: f.onSubmit}
		f.fm.subBindings = append(f.fm.subBindings, enterBinding)
		return []binding{enterBinding}
	}
	return nil
}

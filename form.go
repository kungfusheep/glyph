package forme

// FormField pairs a label with an input control.
type FormField struct {
	label   string
	control any
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

	// auto-wire focusable controls
	for _, ff := range fields {
		if fc, ok := ff.control.(focusable); ok {
			// use ManagedBy if it's an InputC (sets up the text input binding)
			if inp, ok := ff.control.(*InputC); ok {
				inp.ManagedBy(f.fm)
			} else {
				f.fm.Register(fc)
			}
		}
	}

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

// toTemplate builds the VBox of HBox rows.
func (f *FormC) toTemplate() any {
	rows := make([]any, len(f.fields))
	for i, ff := range f.fields {
		ls := f.labelStyle
		ls.Align = AlignRight
		ls = ls.MarginTRBL(0, 1, 0, 0)
		label := Text(ff.label + ":").Width(f.labelWidth).Style(ls)
		rows[i] = HBox(label, ff.control)
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

// bindings relays the FocusManager's focus-cycling bindings.
func (f *FormC) bindings() []binding {
	return f.fm.bindings()
}

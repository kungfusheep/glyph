package tui

// Context provides widget utilities during render.
type Context struct {
	app     *App
	refresh func()
}

// Refresh requests a re-render.
func (c *Context) Refresh() {
	if c.refresh != nil {
		c.refresh()
	}
}

// App returns the application.
func (c *Context) App() *App {
	return c.app
}

// Widget pairs data with a render function.
// It's a self-contained unit with state and UI.
type Widget[T any] struct {
	Data   T
	Render func(data *T, ctx *Context) Component

	// Internal
	app       *App
	component Component
	ctx       *Context
}

// Init initializes the widget with an app context.
func (w *Widget[T]) Init(app *App) *Widget[T] {
	w.app = app
	w.ctx = &Context{
		app:     app,
		refresh: w.Refresh,
	}
	return w
}

// Refresh re-renders the widget and requests a screen update.
func (w *Widget[T]) Refresh() {
	if w.app != nil {
		w.component = nil // Clear cached component
		w.app.RequestRender()
	}
}

// View returns the component tree for this widget.
// Lazily builds/rebuilds when needed.
func (w *Widget[T]) View() Component {
	if w.component == nil && w.Render != nil {
		w.component = w.Render(&w.Data, w.ctx)
	}
	return w.component
}

// Update applies a function to the data and refreshes.
func (w *Widget[T]) Update(fn func(*T)) {
	fn(&w.Data)
	w.Refresh()
}

// WidgetComponent wraps a Widget to implement Component interface.
type WidgetComponent[T any] struct {
	Base
	widget *Widget[T]
}

// WrapWidget creates a Component from a Widget.
func WrapWidget[T any](w *Widget[T]) *WidgetComponent[T] {
	return &WidgetComponent[T]{widget: w}
}

// SetConstraints implements Component.
func (wc *WidgetComponent[T]) SetConstraints(width, height int) {
	wc.Base.SetConstraints(width, height)
	if view := wc.widget.View(); view != nil {
		view.SetConstraints(width, height)
		w, h := view.Size()
		wc.SetSize(w, h)
	}
}

// MinSize implements Component.
func (wc *WidgetComponent[T]) MinSize() (int, int) {
	if view := wc.widget.View(); view != nil {
		return view.MinSize()
	}
	return 0, 0
}

// Render implements Component.
func (wc *WidgetComponent[T]) Render(buf *Buffer, x, y int) {
	if view := wc.widget.View(); view != nil {
		view.Render(buf, x, y)
	}
}

// NewWidget creates and initializes a widget in one go.
func NewWidget[T any](app *App, data T, render func(*T, *Context) Component) *Widget[T] {
	w := &Widget[T]{
		Data:   data,
		Render: render,
	}
	w.Init(app)
	return w
}

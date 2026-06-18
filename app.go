package glyph

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kungfusheep/riffkey"
)

// Debug timing
var (
	DebugTiming     bool
	DebugFullRedraw bool // force full redraws instead of diff-based (set TUI_FULL_REDRAW=1 to enable)
	DebugFlush      bool // dump flush debug info (set TUI_DEBUG_FLUSH=1 to enable)
	lastBuildTime   time.Duration
	lastLayoutTime  time.Duration
	lastRenderTime  time.Duration
	lastFlushTime   time.Duration

	// fine-grained post-processing phase timings (only populated when DebugTiming=true)
	lastEffectTime time.Duration // resolveColor16 + all Effect passes
	lastDiffTime   time.Duration // Flush(): diff + escape-sequence building
	lastWriteTime  time.Duration // FlushBuffer(): Write() syscall to terminal
)

func init() {
	if os.Getenv("TUI_FULL_REDRAW") != "" {
		DebugFullRedraw = true
	}
	if os.Getenv("TUI_DEBUG_FLUSH") != "" {
		DebugFlush = true
	}
}

// App is a TUI application with integrated input handling via riffkey.
type App struct {
	screen *Screen

	// riffkey integration
	router *riffkey.Router
	input  *riffkey.Input
	reader *riffkey.Reader

	// Template + BufferPool (for SetView single-view mode)
	template *Template
	pool     *BufferPool

	// Multi-view routing
	viewTemplates map[string]*Template
	viewRouters   map[string]*riffkey.Router
	currentView   string
	viewStack     []string // pushed views (for modal overlays)

	// State
	running        bool
	renderMu       sync.Mutex
	renderChan     chan struct{}
	// render generations: reqGen stamps every RequestRender; doneGen records the
	// latest request generation a completed render covered. The debounce timer
	// skips only when no request is newer than the last render, so an async
	// request landing just after an input-driven flush is never discarded.
	reqGen  atomic.Uint64
	doneGen atomic.Uint64

	// Apply queue: closures pushed by goroutines, drained at frame top under
	// the render lock before any reads. Double-buffered for zero
	// steady-state allocation.
	applyMu      sync.Mutex
	applied      []func()
	applyScratch []func()
	forceFullFlush bool        // set by Go() to force full redraw on next frame
	suspended      atomic.Bool // when set, render() is a no-op (terminal handed to an external program, e.g. $EDITOR)
	effectsActive  bool        // previous frame used post-processing; clear pooled buffers fully

	// cache-effects skip gate (ADR 19): on a frame whose only trigger is an
	// effect's animation, skip Execute and re-run effects over the cached render.
	appDirty          atomic.Bool // a RequestRender happened since the last full render — forces a full Execute. Set outside renderMu (any goroutine), cleared in render(); atomic to order those.
	effectFramePending atomic.Bool // this frame was requested by an effect's animation (the only case we skip Execute). Set outside renderMu; atomic.
	cleanValid        bool    // cleanBuf holds a valid pristine (pre-effects) render
	cleanBuf          *Buffer // persistent snapshot of the last full render, before effects
	lastAnimating     bool    // last Execute reported a template animation — don't skip while animating

	// Cursor state
	cursorX, cursorY int
	cursorVisible    bool
	cursorShape      CursorShape
	cursorColor      Color
	cursorColorSet   bool

	// default style for all cells (set via SetDefaultStyle)
	defaultStyle Style

	// Resize callback
	onResize func(width, height int)

	// Before-render callback (for syncing state before layout)
	onBeforeRender func()

	// After-render callback (for cursor updates after layout is known)
	onAfterRender func()

	// Active layer for cursor (set during template render)
	activeLayer *Layer

	// Inline mode
	inline         bool
	clearOnExit    bool
	linesUsed      int
	viewHeight     int16 // Height of the view for inline mode
	nonInteractive bool  // True when running via RunNonInteractive

	// Jump labels
	jumpMode  *JumpMode
	jumpStyle JumpStyle

	// Post-processing pipeline
	postProcess   []Effect
	frameCount    uint64
	startTime     time.Time
	lastFrameTime time.Time
	defaultFG     Color // terminal's default FG (detected via OSC 10)
	defaultBG     Color // terminal's default BG (detected via OSC 11)

	// SetView limit (for catching anti-patterns)
	setViewCount int
	setViewLimit int // 0 = unlimited
}

// NewApp creates a new TUI application (fullscreen, alternate buffer).
func NewApp() *App {
	screen := NewScreen(nil)
	router := riffkey.NewRouter()
	input := riffkey.NewInput(router)
	reader := riffkey.NewReader(os.Stdin).SetUTF8(true)

	app := &App{
		screen:     screen,
		router:     router,
		input:      input,
		reader:     reader,
		renderChan: make(chan struct{}, 1),
		jumpMode:   &JumpMode{},
		jumpStyle:  DefaultJumpStyle,
	}

	return app
}

// NewInlineApp creates a new inline TUI application.
// Inline apps render at the current cursor position without taking over the screen.
// Use this for progress bars, selection menus, spinners, etc.
func NewInlineApp() *App {
	app := NewApp()
	app.inline = true
	return app
}

// Ref provides access to the component for external references.
func (a *App) Ref(f func(*App)) *App { f(a); return a }

// ClearOnExit sets whether the inline app should clear its content on exit.
// If true, the rendered content disappears when the app stops.
// If false (default), the content remains visible and cursor moves below it.
func (a *App) ClearOnExit(clear bool) *App {
	a.clearOnExit = clear
	return a
}

// IsInline returns true if this is an inline app.
func (a *App) IsInline() bool {
	return a.inline
}

// Height sets the height for inline apps.
// This determines how many lines the inline view will use.
// If not set, defaults to 1.
func (a *App) Height(h int16) *App {
	a.viewHeight = h
	return a
}

// RunNonInteractive runs an inline app without an input loop.
// Use this for progress bars, spinners, etc. that don't need keyboard input.
// Call Stop() when done to clean up and exit.
func (a *App) RunNonInteractive() error {
	if !a.inline {
		return fmt.Errorf("RunNonInteractive only works with inline apps")
	}

	a.running = true
	a.nonInteractive = true

	// Clean up buffer pool on exit
	if a.pool != nil {
		defer a.pool.Stop()
	}

	// Enter inline mode (raw mode without alternate buffer)
	if err := a.screen.EnterInlineMode(); err != nil {
		return err
	}

	// Initial render
	a.render()

	// frame debounce: render at most once per 16ms (~60fps)
	frameTimer := time.NewTimer(0)
	if !frameTimer.Stop() {
		<-frameTimer.C
	}
	framePending := false

	for a.running {
		select {
		case <-a.renderChan:
			if !framePending {
				framePending = true
				frameTimer.Reset(16 * time.Millisecond)
			}
		case <-frameTimer.C:
			framePending = false
			// drain any render request that arrived during the frame window
			select {
			case <-a.renderChan:
			default:
			}
			a.render()
		case <-time.After(50 * time.Millisecond):
			// check running flag periodically
		}
	}

	// Clean up
	a.screen.ExitInlineMode(a.linesUsed, a.clearOnExit)
	return nil
}

// SetViewLimit sets the maximum number of times SetView can be called.
// Panics if exceeded. Use this to catch anti-patterns where SetView is called
// repeatedly instead of using reactive updates via pointers.
//
// Example:
//
//	app.SetViewLimit(1) // Panic if SetView called more than once
//	app.SetView(myView) // OK
//	app.SetView(other)  // PANIC: SetView called 2 times, limit is 1
//
// Set to 0 (default) for unlimited calls.
func (a *App) SetViewLimit(n int) *App {
	a.setViewLimit = n
	return a
}

// SetView sets a declarative view for fast rendering.
// Pointers in the view are captured at compile time - just mutate your state.
//
// Example:
//
//	state := &MyState{Title: "Hello", Progress: 50}
//	app.SetView(
//	    Col{Children: []Component{
//	        Text{Content: &state.Title},
//	        Progress{Value: &state.Progress},
//	    }},
//	)
func (a *App) SetView(view Component) *App {
	a.setViewCount++
	if a.setViewLimit > 0 && a.setViewCount > a.setViewLimit {
		panic(fmt.Sprintf("SetView called %d times, limit is %d. Use reactive updates via pointers instead of calling SetView repeatedly.", a.setViewCount, a.setViewLimit))
	}

	a.template = Build(view)
	a.template.SetApp(a) // Link for jump mode support
	a.template.requestRender = a.RequestRender
	a.wireBindings(a.template, a.router)
	// SetView's view is implicitly active — there's no separate activation
	// step for single-view apps, so push the focus manager's initial sub-router
	// here to preserve historical behaviour.
	a.activateTemplateFM(a.template)
	// Create buffer pool for async clearing (or reuse existing)
	size := a.screen.Size()
	if a.pool == nil {
		a.pool = NewBufferPool(size.Width, size.Height)
	} else if a.pool.Width() != size.Width || a.pool.Height() != size.Height {
		a.pool.Resize(size.Width, size.Height)
	}
	if a.defaultStyle.FG.Mode != ColorDefault || a.defaultStyle.BG.Mode != ColorDefault {
		for _, buf := range a.pool.buffers {
			buf.defaultStyle = a.defaultStyle
			buf.Clear()
		}
	}
	return a
}

// wireBindings registers all declarative component bindings on the given router.
//
// For the FocusManager, this only sets up routers and bindings; it does NOT
// push the focus manager's initial sub-router onto the global input stack.
// That push is deferred until the view is activated (see activate/deactivate
// below). Callers who treat their root template as immediately active — i.e.
// SetView, the single-view case — must call (*App).activateTemplateFM after
// wireBindings to keep the existing eager-push behaviour. Multi-view callers
// (View) leave the push to happen on Go/RunFrom/PushView.
func (a *App) wireBindings(tmpl *Template, router *riffkey.Router) {
	a.wireBindingList(router, tmpl.pendingRouteBindings)
	componentRouter := router
	if len(tmpl.pendingModalRouteBindings) > 0 {
		tmpl.routeModalRouter = riffkey.NewRouter().Disable()
		a.wireBindingList(tmpl.routeModalRouter, tmpl.pendingModalRouteBindings)
		componentRouter = tmpl.routeModalRouter
	}
	a.wireComponentBindings(tmpl, componentRouter)
	a.wireChildRouteScopes(tmpl)
}

func (a *App) wireComponentBindings(tmpl *Template, router *riffkey.Router) {
	for _, b := range tmpl.pendingBindings {
		a.wireBinding(router, b)
	}
	// focus manager takes precedence over single pendingTIB
	if fm := tmpl.pendingFocusManager; fm != nil {
		// wire focus manager bindings (Tab/Shift-Tab) on the base router
		for _, b := range fm.bindings() {
			if h, ok := b.handler.(func(riffkey.Match)); ok {
				pattern := b.pattern
				router.Handle(pattern, func(m riffkey.Match) { h(m); a.RequestRender() })
			}
		}

		// build a sub-router per focusable item.
		// each gets pushed on focus and popped on blur.
		fm.push = func(r *riffkey.Router) { a.PushRouter(r) }
		fm.pop = func() { a.PopRouter() }
		fm.routers = make([]*riffkey.Router, len(fm.items))

		for i, item := range fm.items {
			sub := riffkey.NewRouter()

			// common: Tab/Shift-Tab to cycle, Escape to blur
			sub.Handle(fm.nextKey, func(_ riffkey.Match) { fm.Next(); a.RequestRender() })
			if fm.prevKey != "" {
				sub.Handle(fm.prevKey, func(_ riffkey.Match) { fm.Prev(); a.RequestRender() })
			}
			sub.Handle("<Escape>", func(_ riffkey.Match) { fm.BlurCurrent(); a.RequestRender() })

			// sub-bindings (e.g., Enter for form submit)
			for _, sb := range fm.subBindings {
				switch h := sb.handler.(type) {
				case func():
					pattern := sb.pattern
					sub.Handle(pattern, func(_ riffkey.Match) { h(); a.RequestRender() })
				case func(riffkey.Match):
					pattern := sb.pattern
					sub.Handle(pattern, func(m riffkey.Match) { h(m); a.RequestRender() })
				}
			}

			// per-item bindings (e.g., j/k for Radio, Space for Checkbox)
			for _, cb := range item.bindings {
				switch h := cb.handler.(type) {
				case func():
					pattern := cb.pattern
					sub.Handle(pattern, func(_ riffkey.Match) { h(); a.RequestRender() })
				case func(riffkey.Match):
					pattern := cb.pattern
					sub.Handle(pattern, func(m riffkey.Match) { h(m); a.RequestRender() })
				}
			}

			if item.tib != nil {
				// text input: route unmatched keys to TextHandler
				th := fm.handlers[i]
				sub.HandleUnmatched(th.HandleKey)
				sub.NoCounts()
			}

			fm.routers[i] = sub
		}
		// initial push deferred to activation time; see activateTemplateFM.
	} else if tmpl.pendingTIB != nil {
		th := riffkey.NewTextHandler(tmpl.pendingTIB.value, tmpl.pendingTIB.cursor)
		th.OnChange = tmpl.pendingTIB.onChange
		th.AllowNewlines = tmpl.pendingTIB.multiline
		router.HandleUnmatched(th.HandleKey)
		router.NoCounts()
	}
	// wire Log invalidation
	for _, lv := range tmpl.pendingLogs {
		lv.onUpdate = a.RequestRender
	}
}

func (a *App) wireBindingList(router *riffkey.Router, bindings []binding) {
	for _, b := range bindings {
		a.wireBinding(router, b)
	}
}

func (a *App) wireBinding(router *riffkey.Router, b binding) {
	switch h := b.handler.(type) {
	case func(riffkey.Match):
		pattern := b.pattern
		router.Handle(pattern, func(m riffkey.Match) { h(m); a.RequestRender() })
	case func(any):
		pattern := b.pattern
		router.Handle(pattern, func(_ riffkey.Match) { h(nil); a.RequestRender() })
	case func():
		pattern := b.pattern
		router.Handle(pattern, func(_ riffkey.Match) { h(); a.RequestRender() })
	}
}

func (a *App) wireChildRouteScopes(tmpl *Template) {
	if tmpl == nil {
		return
	}
	for _, child := range routeChildTemplates(tmpl) {
		hasModal := len(child.pendingModalRouteBindings) > 0
		hasScopedComponents := len(child.pendingBindings) > 0 || child.pendingTIB != nil || child.pendingFocusManager != nil
		if len(child.pendingRouteBindings) > 0 || (!hasModal && hasScopedComponents) {
			child.routeRouter = riffkey.NewRouter().Disable()
			a.wireBindingList(child.routeRouter, child.pendingRouteBindings)
			a.wireComponentBindings(child, child.routeRouter)
		}
		if hasModal {
			child.routeModalRouter = riffkey.NewRouter().Disable()
			a.wireBindingList(child.routeModalRouter, child.pendingModalRouteBindings)
			a.wireComponentBindings(child, child.routeModalRouter)
			if !hasScopedComponents {
				a.wireComponentBindings(tmpl, child.routeModalRouter)
			}
		}
		a.wireChildRouteScopes(child)
	}
}

// ViewBuilder allows chaining Handle() calls after View().
type ViewBuilder struct {
	app    *App
	name   string
	router *riffkey.Router
}

// View registers a named view for multi-view routing.
// Returns a builder for chaining Handle() calls.
//
// Example:
//
//	app.View("home", homeView).
//	    Handle("j", moveDown).
//	    Handle("s", func(_ riffkey.Match) { app.Go("settings") })
func (a *App) View(name string, view Component) *ViewBuilder {
	// Initialize maps if needed
	if a.viewTemplates == nil {
		a.viewTemplates = make(map[string]*Template)
	}
	if a.viewRouters == nil {
		a.viewRouters = make(map[string]*riffkey.Router)
	}

	// Create buffer pool if not exists (shared across all views)
	if a.pool == nil {
		size := a.screen.Size()
		a.pool = NewBufferPool(size.Width, size.Height)
		if a.defaultStyle.FG.Mode != ColorDefault || a.defaultStyle.BG.Mode != ColorDefault {
			for _, buf := range a.pool.buffers {
				buf.defaultStyle = a.defaultStyle
			}
		}
	}

	// Compile template and create router for this view
	tmpl := Build(view)
	tmpl.SetApp(a) // Link for jump mode support
	router := riffkey.NewRouter()
	a.wireBindings(tmpl, router)
	a.viewTemplates[name] = tmpl
	a.viewRouters[name] = router

	return &ViewBuilder{
		app:    a,
		name:   name,
		router: router,
	}
}

// Ref provides access to the component for external references.
func (vb *ViewBuilder) Ref(f func(*ViewBuilder)) *ViewBuilder { f(vb); return vb }

// NoCounts disables vim-style count prefixes (e.g., 5j) for this view.
// Use this when the view has text input so digits can be typed.
func (vb *ViewBuilder) NoCounts() *ViewBuilder {
	vb.router.NoCounts()
	return vb
}

// Handle registers a key handler for this view.
// Accepts func(riffkey.Match), func(any), or func() for convenience.
// Automatically requests a re-render after the handler runs.
func (vb *ViewBuilder) Handle(pattern string, handler any) *ViewBuilder {
	switch h := handler.(type) {
	case func(riffkey.Match):
		vb.router.Handle(pattern, func(m riffkey.Match) { h(m); vb.app.RequestRender() })
	case func(any):
		vb.router.Handle(pattern, func(_ riffkey.Match) { h(nil); vb.app.RequestRender() })
	case func():
		vb.router.Handle(pattern, func(_ riffkey.Match) { h(); vb.app.RequestRender() })
	}
	return vb
}

// UpdateView recompiles a view with a new view definition.
// Use this when the view's structure changes and needs re-compilation.
func (a *App) UpdateView(name string, view Component) {
	if a.viewTemplates == nil {
		return
	}
	// if the named view is currently active and has a focus manager pushed,
	// deactivate it first so the input stack stays balanced across the swap.
	wasActive := a.currentView == name
	for _, n := range a.viewStack {
		if n == name {
			wasActive = true
			break
		}
	}
	if wasActive {
		a.deactivateView(name)
	}
	tmpl := Build(view)
	tmpl.SetApp(a) // Link for jump mode support
	if router, ok := a.viewRouters[name]; ok {
		a.wireBindings(tmpl, router)
	}
	a.viewTemplates[name] = tmpl
	if wasActive {
		a.activateView(name)
	}
}

// Go switches to a different view.
// Swaps the template and input handlers.
func (a *App) Go(name string) {
	if _, ok := a.viewTemplates[name]; !ok {
		return // View doesn't exist
	}
	if a.currentView != "" {
		a.deactivateView(a.currentView)
	}
	a.currentView = name
	a.input.SetRouter(a.viewRouters[name])
	a.activateView(name)
	// diff flush handles view switches correctly — every changed cell gets
	// emitted. FlushFull issues \x1b[2J (clear) which can produce a visible
	// microflash on some terminals (Ghostty) during a sync-wrapped update.
	// a.forceFullFlush = true
	a.RequestRender()
}

// activateTemplateFM pushes a template's focus-manager initial sub-router onto
// the input stack, if the template has a focus manager. Called when a view
// becomes active. Safe to call when the template has no focus manager.
func (a *App) activateTemplateFM(tmpl *Template) {
	if tmpl == nil {
		return
	}
	a.attachRouteScopes(tmpl)
	if fm := tmpl.pendingFocusManager; fm != nil {
		fm.initialPush()
	}
	if tmpl.routeModalRouter != nil && !tmpl.routeModalPushed {
		tmpl.routeModalRouter.Enable()
		a.input.Push(tmpl.routeModalRouter)
		tmpl.routeModalPushed = true
	}
}

// deactivateTemplateFM pops a template's focus-manager sub-router from the
// input stack, if one is currently pushed. Called when a view loses active
// status (Go to another view, PopView, etc).
func (a *App) deactivateTemplateFM(tmpl *Template) {
	if tmpl == nil {
		return
	}
	a.detachRouteScopes(tmpl)
	if tmpl.routeModalRouter != nil && tmpl.routeModalPushed {
		a.input.Pop()
		tmpl.routeModalRouter.Disable()
		tmpl.routeModalPushed = false
	}
	if fm := tmpl.pendingFocusManager; fm != nil && fm.pushed {
		if fm.pop != nil {
			fm.pop()
		}
		fm.pushed = false
	}
}

func (a *App) attachRouteScopes(tmpl *Template) {
	if tmpl == nil || a.input == nil {
		return
	}
	for _, child := range routeChildTemplates(tmpl) {
		if child.routeRouter != nil && !child.routeAttached {
			child.routeRouter.Disable()
			a.input.Attach(child.routeRouter)
			child.routeAttached = true
		}
		a.attachRouteScopes(child)
	}
}

func (a *App) detachRouteScopes(tmpl *Template) {
	if tmpl == nil || a.input == nil {
		return
	}
	for _, child := range routeChildTemplates(tmpl) {
		a.detachRouteScopes(child)
		if child.routeModalRouter != nil && child.routeModalPushed {
			a.input.Pop()
			child.routeModalRouter.Disable()
			child.routeModalPushed = false
		}
		if child.routeRouter != nil && child.routeAttached {
			child.routeRouter.Disable()
			a.input.Detach(child.routeRouter)
			child.routeAttached = false
		}
	}
}

// activateView runs activation work for the named view: focus manager push,
// any future per-view lifecycle hooks.
func (a *App) activateView(name string) {
	if a.viewTemplates == nil {
		return
	}
	a.activateTemplateFM(a.viewTemplates[name])
}

// deactivateView runs deactivation work for the named view.
func (a *App) deactivateView(name string) {
	if a.viewTemplates == nil {
		return
	}
	a.deactivateTemplateFM(a.viewTemplates[name])
}

// Back returns to the previous view.
// Currently an alias for Pop().
func (a *App) Back() {
	a.input.Pop()
	a.RequestRender()
}

// PushView pushes a view as a modal overlay.
// The modal's handlers take precedence until PopView() is called.
// The pushed view becomes the active rendered view until popped.
func (a *App) PushView(name string) {
	if router, ok := a.viewRouters[name]; ok {
		a.viewStack = append(a.viewStack, name)
		a.input.Push(router)
		a.activateView(name)
		a.RequestRender()
	}
}

// PopView removes the top modal overlay.
// Returns to the previous view in the stack.
func (a *App) PopView() {
	if len(a.viewStack) == 0 {
		a.input.Pop()
		a.RequestRender()
		return
	}
	topName := a.viewStack[len(a.viewStack)-1]
	// pop the focus manager's sub-router (if any) before popping the view
	// router itself, so the input stack ends up balanced.
	a.deactivateView(topName)
	a.viewStack = a.viewStack[:len(a.viewStack)-1]
	a.input.Pop()
	a.RequestRender()
}

// ViewRouter returns the router for a named view, if it exists.
// Useful for advanced configuration like HandleUnmatched.
func (a *App) ViewRouter(name string) (*riffkey.Router, bool) {
	if a.viewRouters == nil {
		return nil, false
	}
	router, ok := a.viewRouters[name]
	return router, ok
}

// Screen returns the screen.
func (a *App) Screen() *Screen {
	return a.screen
}

// Router returns the riffkey router for advanced configuration.
func (a *App) Router() *riffkey.Router {
	return a.router
}

// CurrentView returns the name of the active named view, or "" for a SetView app
// (or before RunFrom/Go has activated one).
func (a *App) CurrentView() string {
	return a.currentView
}

// Input returns the riffkey input for modal handling (push/pop).
func (a *App) Input() *riffkey.Input {
	return a.input
}

// Handle registers a key binding with a vim-style pattern.
// Examples: "j", "gg", "<C-c>", "<C-w>j", "<Up>"
// Accepts func(riffkey.Match), func(any), or func() for convenience.
// Automatically requests a re-render after the handler runs.
func (a *App) Handle(pattern string, handler any) *App {
	switch h := handler.(type) {
	case func(riffkey.Match):
		a.router.Handle(pattern, func(m riffkey.Match) { h(m); a.RequestRender() })
	case func(any):
		a.router.Handle(pattern, func(_ riffkey.Match) { h(nil); a.RequestRender() })
	case func():
		a.router.Handle(pattern, func(_ riffkey.Match) { h(); a.RequestRender() })
	}
	return a
}

// HandleNamed registers a named key binding (for rebinding support).
// Automatically requests a re-render after the handler runs.
func (a *App) HandleNamed(name, pattern string, handler func(riffkey.Match)) *App {
	a.router.HandleNamed(name, pattern, func(m riffkey.Match) { handler(m); a.RequestRender() })
	return a
}

// BindField routes unmatched keys to a text input field.
func (a *App) BindField(f *InputState) *App {
	a.router.TextInput(&f.Value, &f.Cursor)
	return a
}

// UnbindField clears the text input field binding.
func (a *App) UnbindField() *App {
	a.router.HandleUnmatched(nil)
	return a
}

// PushRouter pushes a new router onto the input stack (for modal input).
func (a *App) PushRouter(r *riffkey.Router) {
	a.input.Push(r)
}

// PopRouter pops the current router from the input stack.
func (a *App) PopRouter() {
	a.input.Pop()
}

// SetCursor sets the cursor position (0-indexed screen coordinates).
// The cursor will be positioned here after each render.
func (a *App) SetCursor(x, y int) {
	a.cursorX = x
	a.cursorY = y
}

// SetCursorStyle sets the cursor visual style.
func (a *App) SetCursorStyle(style CursorShape) {
	a.cursorShape = style
}

// ShowCursor makes the cursor visible.
func (a *App) ShowCursor() {
	a.cursorVisible = true
}

// HideCursor hides the cursor.
func (a *App) HideCursor() {
	a.cursorVisible = false
}

// SetCursorColor sets the cursor color using OSC 12 escape sequence.
// This changes the actual cursor color in supporting terminals.
// SetDefaultStyle sets the default style for all cells. Any cell not explicitly
// styled will use this FG/BG instead of the terminal default.
func (a *App) SetDefaultStyle(s Style) {
	a.defaultStyle = s
	if a.pool != nil {
		for _, buf := range a.pool.buffers {
			buf.defaultStyle = s
		}
	}
}

func (a *App) SetCursorColor(c Color) {
	a.cursorColor = c
	a.cursorColorSet = true
}

// Cursor returns the current cursor state.
func (a *App) Cursor() Cursor {
	return Cursor{
		X:       a.cursorX,
		Y:       a.cursorY,
		Style:   a.cursorShape,
		Visible: a.cursorVisible,
	}
}

// OnResize sets a callback to be called when the terminal is resized.
// The callback receives the new width and height.
// Use this to update viewport dimensions, reinitialize layers, etc.
func (a *App) OnResize(fn func(width, height int)) {
	a.onResize = fn
}

// OnBeforeRender sets a callback to be called before each render.
// Use this to sync state (e.g., filter updates) before layout runs.
func (a *App) OnBeforeRender(fn func()) {
	a.onBeforeRender = fn
}

// OnAfterRender sets a callback to be called after each render completes.
// Use this to update cursor position after layout is known.
func (a *App) OnAfterRender(fn func()) {
	a.onAfterRender = fn
}

// AddEffect appends a post-processing pass to the pipeline.
// Passes run in order after template rendering, before screen flush.
// Closures captured by the pass act as shader uniforms — mutate them to
// change behaviour next frame.
func (a *App) AddEffect(pp Effect) *App {
	a.postProcess = append(a.postProcess, pp)
	return a
}

// SetEffect replaces the entire post-processing pipeline.
// Call with no arguments to clear all passes.
func (a *App) SetEffect(passes ...Effect) *App {
	a.postProcess = passes
	return a
}

// Template returns the current template for debugging.
// Use with Template().DebugDump("") to inspect the op tree.
func (a *App) Template() *Template {
	return a.template
}

// RequestRender marks that a render is needed.
// Safe to call from any goroutine.
// Apply enqueues fn to run on the render thread at the top of the next
// frame, under the render lock, before any reads — the safe place to push a
// goroutine's result into bound state:
//
//	go func() {
//	    result := fetch()
//	    app.Apply(func() {
//	        if key == current { rows = result }
//	    })
//	}()
//
// Closures run in apply order, exactly once. A closure applied during the
// drain runs NEXT frame (anti-livelock: a self-applying chain cannot wedge a
// frame). Closures may spawn goroutines but must never render (RenderNow
// would deadlock on the render lock) and never block.
func (a *App) Apply(fn func()) {
	a.applyMu.Lock()
	a.applied = append(a.applied, fn)
	a.applyMu.Unlock()
	a.RequestRender()
}

// drainApplies runs the queued apply closures. Called inside render() under
// renderMu at frame top; the batch is swapped out first, so closures applied
// during the drain land in the next batch.
func (a *App) drainApplies() {
	a.applyMu.Lock()
	batch := a.applied
	a.applied = a.applyScratch[:0]
	a.applyMu.Unlock()
	for i, fn := range batch {
		fn()
		batch[i] = nil
	}
	a.applyMu.Lock()
	a.applyScratch = batch
	a.applyMu.Unlock()
}

func (a *App) RequestRender() {
	// any caller of RequestRender (input, Apply, resize, view change, and
	// template animations via t.requestRender) is a real state change — mark the
	// app dirty so the next frame does a full Execute, not an effect-only skip.
	a.appDirty.Store(true)
	a.reqGen.Add(1)
	select {
	case a.renderChan <- struct{}{}:
	default:
		// Already a render pending
	}
}

// requestEffectFrame schedules another frame WITHOUT marking the app dirty — used
// by an animating screen effect that wants to advance over unchanged app output.
// The next frame can then skip Execute and re-run effects over the cached render.
func (a *App) requestEffectFrame() {
	a.effectFramePending.Store(true)
	a.reqGen.Add(1)
	select {
	case a.renderChan <- struct{}{}:
	default:
	}
}

// RenderNow performs a render immediately without channel coordination.
// Use this from dedicated update goroutines to avoid scheduler overhead.
// The render is mutex-protected so it's safe to call concurrently.
func (a *App) RenderNow() {
	a.render()
}

// ForceRedraw schedules a FULL repaint (every cell) on the next frame instead of
// the usual diff against the front buffer. Needed after the terminal has been
// written to outside glyph's control — e.g. shelling out to $EDITOR and
// re-entering raw mode (which clears the screen) — where the stale front buffer
// would otherwise suppress the repaint and leave a blank screen.
func (a *App) ForceRedraw() {
	a.forceFullFlush = true
	a.RequestRender()
}

// Suspend stops all rendering until Resume. Use it around handing the terminal to an
// external program (e.g. shelling out to $EDITOR): while suspended, render() is a
// no-op, so no source — animation goroutines, reload signals, anything — can draw over
// the other program. One gate instead of teaching every render caller about the editor.
func (a *App) Suspend() { a.suspended.Store(true) }

// Resume re-enables rendering and forces a full repaint (the external program will have
// scribbled over the screen, so a diff against the stale front buffer isn't enough).
func (a *App) Resume() {
	a.suspended.Store(false)
	a.ForceRedraw()
}

// render performs the actual render if needed.
func (a *App) render() {
	if a.suspended.Load() {
		return // terminal handed to an external program — draw nothing
	}
	a.renderMu.Lock()
	defer a.renderMu.Unlock()

	// capture before reading state: any request stamped after this point may
	// carry state this frame won't include, so the debounce must re-render
	coveredGen := a.reqGen.Load()
	defer a.doneGen.Store(coveredGen)

	// apply queued state pushes first — frame top, under the lock, before
	// any reads
	a.drainApplies()

	// consume the effect-frame request: only a frame an effect explicitly asked
	// for (via requestEffectFrame) is eligible to skip Execute. A direct render()
	// or a RequestRender-driven frame always does a full Execute.
	effectFrame := a.effectFramePending.Swap(false)

	var t0, t1 time.Time
	if DebugTiming {
		t0 = time.Now()
	}

	if a.pool == nil {
		return // No pool
	}

	// sync state before layout (e.g., filter updates)
	if a.onBeforeRender != nil {
		a.onBeforeRender()
	}

	// clear active layer before render (will be set if a layer has visible cursor)
	a.activeLayer = nil

	size := a.screen.Size()
	buf := a.pool.Current()
	if a.effectsActive {
		buf.Clear()
	}

	// For inline mode, use view height instead of terminal height
	renderHeight := int16(size.Height)
	if a.inline && a.viewHeight > 0 {
		renderHeight = a.viewHeight
	} else if a.inline {
		// auto-size: give layout full terminal height, then trim to content
		renderHeight = int16(size.Height)
	}

	// Priority: pushed views > current view > base template
	var activeTmpl *Template
	if len(a.viewStack) > 0 {
		topView := a.viewStack[len(a.viewStack)-1]
		if a.viewTemplates != nil {
			if tmpl, ok := a.viewTemplates[topView]; ok {
				activeTmpl = tmpl
			}
		}
	}
	if activeTmpl == nil {
		if a.currentView != "" && a.viewTemplates != nil {
			if tmpl, ok := a.viewTemplates[a.currentView]; ok {
				activeTmpl = tmpl
			} else {
				return // View not found
			}
		} else if a.template != nil {
			activeTmpl = a.template
		} else {
			return // No view set
		}
	}
	// effect-only skip (ADR 19): when nothing in the app changed since the last
	// full render and that frame ran effects, reuse the cached pristine render and
	// re-run the effect pipeline over it instead of re-Executing the whole
	// template. Conservative — fullscreen only, never while animating a template,
	// in jump mode, or when a full flush is forced.
	if effectFrame && !a.appDirty.Load() && a.cleanValid && a.effectsActive && !a.lastAnimating &&
		!a.inline && !a.forceFullFlush && !DebugFullRedraw && !a.JumpModeActive() &&
		a.cleanBuf != nil && a.cleanBuf.Width() == size.Width &&
		a.cleanBuf.Height() == size.Height {
		a.copyToScreen(a.cleanBuf) // back = pristine cached render
		a.screen.forceRGB = true
		a.applyEffects(a.screen.Buffer(), activeTmpl.ScreenEffects(), size.Width, int(renderHeight))
		a.screen.Flush()
		if a.cursorColorSet {
			a.screen.BufferCursorColor(a.cursorColor)
		}
		a.screen.BufferCursor(a.cursorX, a.cursorY, a.cursorVisible, a.cursorShape)
		a.screen.FlushBuffer()
		return
	}
	// Jump-target rebuild WITHOUT holding a lock across Execute. active is atomic
	// so the gate is lock-free; targets accumulate into the build scratch during
	// Execute (render-goroutine-only) and AssignLabels swaps them into Targets in
	// one locked step. The swap is atomic to the input goroutine's reads, so its
	// len-check never sees Targets transiently empty (#400 no-op); and because no
	// jump lock is held across Execute, a consumer's layer Render calling
	// JumpModeActive() mid-Execute can't re-enter and deadlock.
	jumpActive := a.jumpMode != nil && a.jumpMode.isActive()
	if jumpActive {
		a.jumpMode.ClearTargets() // reset the build scratch for a fresh frame
	}
	activeTmpl.Execute(buf, int16(size.Width), renderHeight)
	if jumpActive {
		a.jumpMode.AssignLabels() // swap built→Targets + label, atomically
	}

	// for inline auto-size, use content height instead of full terminal height
	if a.inline && a.viewHeight == 0 {
		if h := buf.ContentHeight(); h > 0 {
			renderHeight = int16(h)
		}
	}

	// apply layer cursor if one was set during template render
	if a.activeLayer != nil {
		if x, y, visible := a.activeLayer.ScreenCursor(); visible {
			a.cursorX = x
			a.cursorY = y
			a.cursorVisible = true
			a.cursorShape = a.activeLayer.cursor.Style
		}
	}

	// call after-render callback (e.g., for additional cursor customization)
	if a.onAfterRender != nil {
		a.onAfterRender()
	}

	if DebugTiming {
		t1 = time.Now()
		lastBuildTime = 0
		lastLayoutTime = 0
		lastRenderTime = t1.Sub(t0)
	}

	// snapshot the pristine (pre-effects) render so an effect-only frame can
	// reuse it and skip Execute. cleanBuf is sized to the screen and reused.
	if a.cleanBuf == nil || a.cleanBuf.Width() != buf.Width() || a.cleanBuf.Height() != buf.Height() {
		a.cleanBuf = NewBuffer(buf.Width(), buf.Height())
	}
	a.cleanBuf.CopyFrom(buf)
	a.cleanValid = true
	a.appDirty.Store(false)
	a.lastAnimating = activeTmpl.Animating()

	// post-processing pipeline: tree-declared ScreenEffects first, then imperative
	treeEffects := activeTmpl.ScreenEffects()
	effectsActive := len(treeEffects) > 0 || len(a.postProcess) > 0
	a.effectsActive = effectsActive
	a.screen.forceRGB = effectsActive
	if a.screen.forceRGB {
		var tEffect time.Time
		if DebugTiming {
			tEffect = time.Now()
		}

		a.applyEffects(buf, treeEffects, size.Width, int(renderHeight))

		if DebugTiming {
			lastEffectTime = time.Since(tEffect)
		}
	}
	if a.JumpModeActive() {
		a.paintJumpLabels(buf, int(renderHeight))
	}

	// Copy to screen's back buffer for flush
	a.copyToScreen(buf)

	if a.inline {
		// Inline mode: render at cursor position
		a.linesUsed = a.screen.FlushInline(int(renderHeight), a.linesUsed)
		a.pool.Swap() // Queue async clear
	} else {
		// Fullscreen mode
		var tDiff time.Time
		if DebugTiming {
			tDiff = time.Now()
		}
		if DebugFullRedraw || a.forceFullFlush {
			a.forceFullFlush = false
			a.screen.FlushFull()
		} else {
			a.screen.Flush() // diff + escape-sequence building
		}
		if DebugTiming {
			lastDiffTime = time.Since(tDiff)
		}
		a.pool.Swap()

		// Add cursor ops to same buffer - one syscall for everything
		if a.cursorColorSet {
			a.screen.BufferCursorColor(a.cursorColor)
		}
		a.screen.BufferCursor(a.cursorX, a.cursorY, a.cursorVisible, a.cursorShape)

		var tWrite time.Time
		if DebugTiming {
			tWrite = time.Now()
		}
		a.screen.FlushBuffer() // single Write() syscall to terminal
		if DebugTiming {
			lastWriteTime = time.Since(tWrite)
		}
	}

	if DebugTiming {
		lastFlushTime = time.Since(t1)
	}
}

// copyToScreen copies pool buffer to screen's back buffer.
func (a *App) copyToScreen(src *Buffer) {
	dst := a.screen.Buffer()
	dst.CopyFrom(src) // Fast bulk copy
}

// applyEffects runs the post-processing pipeline over buf: resolve Color16,
// build the frame PostContext, apply tree + imperative effects, and request
// another frame if any effect is mid-animation. Shared by the full-render path
// and the effect-only skip path (ADR 19).
func (a *App) applyEffects(buf *Buffer, treeEffects []Effect, width, height int) {
	// resolve Color16 cells to detected palette RGB before effects run
	resolveColor16(buf, width, height)

	now := time.Now()
	if a.startTime.IsZero() {
		a.startTime = now
	}
	var delta time.Duration
	if !a.lastFrameTime.IsZero() {
		delta = now.Sub(a.lastFrameTime)
	}
	a.lastFrameTime = now

	// OSC 10/11 populates defaultFG/BG only if the terminal supports it; fall
	// back to the explicit SetDefaultStyle values so effects work either way.
	ppFG := a.defaultFG
	if ppFG.Mode == ColorDefault {
		ppFG = a.defaultStyle.FG
	}
	ppBG := a.defaultBG
	if ppBG.Mode == ColorDefault {
		ppBG = a.defaultStyle.BG
	}
	var animReq bool
	ppCtx := PostContext{
		Width:     width,
		Height:    height,
		Frame:     a.frameCount,
		Delta:     delta,
		Time:      now.Sub(a.startTime),
		DefaultFG: ppFG,
		DefaultBG: ppBG,
		animReq:   &animReq,
	}
	for _, pp := range treeEffects {
		pp.Apply(buf, ppCtx)
	}
	for _, pp := range a.postProcess {
		pp.Apply(buf, ppCtx)
	}
	if animReq {
		// effect wants another frame but app state is unchanged — schedule a
		// frame without dirtying the app so the next one can skip Execute.
		a.requestEffectFrame()
	}
	buf.MarkAllDirty()
	a.frameCount++
}

// TimingString returns a formatted timing string.
func TimingString() string {
	return fmt.Sprintf("render:%v effect:%v diff:%v write:%v",
		lastRenderTime.Round(time.Microsecond),
		lastEffectTime.Round(time.Microsecond),
		lastDiffTime.Round(time.Microsecond),
		lastWriteTime.Round(time.Microsecond))
}

// Timings holds timing data for the last frame.
type Timings struct {
	BuildUs  float64 // Build time in microseconds
	LayoutUs float64 // Layout time in microseconds
	RenderUs float64 // Render time in microseconds
	FlushUs  float64 // Flush time in microseconds

	// fine-grained post-processing breakdown (only valid when effects are active)
	EffectUs float64 // resolveColor16 + all Effect passes (pure Go)
	DiffUs   float64 // Flush(): diff comparison + escape-sequence building (pure Go)
	WriteUs  float64 // FlushBuffer(): Write() syscall — time spent waiting on terminal
}

// GetTimings returns the timing data for the last frame.
func GetTimings() Timings {
	return Timings{
		BuildUs:  float64(lastBuildTime.Microseconds()),
		LayoutUs: float64(lastLayoutTime.Microseconds()),
		RenderUs: float64(lastRenderTime.Microseconds()),
		FlushUs:  float64(lastFlushTime.Microseconds()),
		EffectUs: float64(lastEffectTime.Microseconds()),
		DiffUs:   float64(lastDiffTime.Microseconds()),
		WriteUs:  float64(lastWriteTime.Microseconds()),
	}
}

// Run starts the application. Blocks until Stop is called.
// For multi-view apps, use RunFrom(startView) instead.
func (a *App) Run() error {
	return a.run("")
}

// RunFrom starts the application on the specified view.
// Use this for multi-view apps.
func (a *App) RunFrom(startView string) error {
	return a.run(startView)
}

func (a *App) run(startView string) error {
	a.running = true

	// Set up starting view if specified
	if startView != "" && a.viewTemplates != nil {
		a.currentView = startView
		if router, ok := a.viewRouters[startView]; ok {
			a.input.SetRouter(router)
		}
		a.activateView(startView)
	}

	// Clean up buffer pool on exit if using fast path
	if a.pool != nil {
		defer a.pool.Stop()
	}

	// Enter raw mode (inline or fullscreen)
	if a.inline {
		if err := a.screen.EnterInlineMode(); err != nil {
			return err
		}
		// Use closure so linesUsed is read at defer time, not now (when it's 0)
		defer func() { a.screen.ExitInlineMode(a.linesUsed, a.clearOnExit) }()
	} else {
		if err := a.screen.EnterRawMode(); err != nil {
			return err
		}
		defer a.screen.ExitRawMode()
		// if a default style is set, paint the terminal background immediately
		// before the first frame renders (avoids flash of terminal default colours).
		// OSC 11 also tells the terminal to use this as its default bg, which
		// covers unfilled areas (e.g. partial bottom row below our content).
		if a.defaultStyle.BG.Mode == ColorRGB {
			a.screen.SetTerminalBG(a.defaultStyle.BG)
			bg := a.defaultStyle.BG
			fmt.Fprintf(a.screen.writer, "\x1b[48;2;%d;%d;%dm\x1b[2J\x1b[H", bg.R, bg.G, bg.B)
		}
	}

	// The initial size is a resize event too: deliver it before the first
	// frame so size-derived state never needs per-frame recomputation.
	if a.onResize != nil {
		size := a.screen.Size()
		a.onResize(size.Width, size.Height)
	}

	// Handle resize
	go a.handleResize()

	// Handle async render requests (from timers, data updates, etc)
	go a.handleRenderRequests()

	// Render first so the screen is populated before we block on the color query.
	// This eliminates the blank-screen flash between alternate buffer switch and first frame.
	a.render()

	// Detect terminal's default colours for post-processing and opacity fallback.
	// Runs after first render so the blank gap is gone; runs before input.Run so
	// there's no race on stdin.
	a.defaultFG, a.defaultBG = a.screen.QueryDefaultColors()
	// Plumb queried colours into the buffer default style so opacity blends and
	// any other path that reads buf.defaultStyle can lerp toward a concrete BG.
	// Caller-supplied SetDefaultStyle values still win.
	if a.defaultStyle.FG.Mode == ColorDefault && a.defaultFG.Mode != ColorDefault {
		a.defaultStyle.FG = a.defaultFG
	}
	if a.defaultStyle.BG.Mode == ColorDefault && a.defaultBG.Mode != ColorDefault {
		a.defaultStyle.BG = a.defaultBG
	}
	if a.pool != nil {
		for _, buf := range a.pool.buffers {
			buf.defaultStyle = a.defaultStyle
		}
	}
	a.RequestRender()

	// Run riffkey input loop
	// render immediately on input for zero-latency response;
	// signal debounce timer to skip its next frame since we just rendered
	err := a.input.Run(a.reader, func(handled bool) {
		if a.running {
			// drain any pending render request so the debounce timer won't double-render
			select {
			case <-a.renderChan:
			default:
			}
			a.render()
		}
	})

	// Normal termination via Stop() causes reader to return error
	if !a.running {
		// Reopen stdin for inline apps so subsequent apps can use it
		if a.inline {
			reopenStdin()
		}
		return nil
	}
	return err
}

// handleRenderRequests processes async render requests with frame debouncing.
// Renders at most once per 16ms (~60fps), coalescing multiple requests.
func (a *App) handleRenderRequests() {
	frameTimer := time.NewTimer(0)
	if !frameTimer.Stop() {
		<-frameTimer.C
	}
	framePending := false

	for {
		select {
		case <-a.renderChan:
			if !a.running {
				return
			}
			if !framePending {
				framePending = true
				frameTimer.Reset(8 * time.Millisecond)
			}
		case <-frameTimer.C:
			framePending = false
			if !a.running {
				return
			}
			// skip only when the last completed render already covered every
			// request; a request stamped after that render carries newer state
			// and must flush even if input rendered in between
			if a.reqGen.Load() == a.doneGen.Load() {
				continue
			}
			// drain any render request that arrived during the frame window
			select {
			case <-a.renderChan:
			default:
			}
			a.render()
		}
	}
}

// Stop signals the application to stop.
func (a *App) Stop() {
	a.running = false
	// Close stdin to unblock the input reader (not needed for non-interactive)
	if !a.nonInteractive {
		os.Stdin.Close()
	}
}

// reopenStdin reopens stdin from /dev/tty after it was closed.
// This allows running multiple inline apps in sequence.
func reopenStdin() {
	f, err := os.Open("/dev/tty")
	if err == nil {
		os.Stdin = f
	}
}

// handleResize watches for terminal resize events.
func (a *App) handleResize() {
	for size := range a.screen.ResizeChan() {
		a.applyResize(size.Width, size.Height)
		a.RequestRender()
	}
}

// applyResize mutates the pool and runs the OnResize callback under the render
// lock so an in-flight Execute never sees a torn buffer (cells/width/height/
// dirtyRows swap non-atomically in Buffer.Resize). OnResize callbacks must not
// render synchronously (RenderNow) — that would deadlock; RequestRender is fine.
func (a *App) applyResize(width, height int) {
	a.renderMu.Lock()
	defer a.renderMu.Unlock()
	if a.pool != nil {
		a.pool.Resize(width, height)
	}
	if a.onResize != nil {
		a.onResize(width, height)
	}
	a.forceFullFlush = true
}

// Size returns the current screen size.
func (a *App) Size() Size {
	return a.screen.Size()
}

// =============================================================================
// Jump Labels
// =============================================================================

// JumpKey registers a key pattern to trigger jump mode.
// This is a convenience method that calls EnterJumpMode when the key is pressed.
func (a *App) JumpKey(pattern string) *App {
	a.router.Handle(pattern, func(_ riffkey.Match) {
		a.EnterJumpMode()
	})
	return a
}

// EnterJumpScope activates jump mode restricted to one or more screen regions,
// each given as a *NodeRef populated by a container's .NodeRef() each frame.
// Only targets that render inside a region are labelled and selectable; the
// whole-screen EnterJumpMode is the no-region case.
func (a *App) EnterJumpScope(rects ...*NodeRef) {
	if a.jumpMode == nil {
		a.jumpMode = &JumpMode{}
	}
	a.jumpMode.setScope(rects)
	a.EnterJumpMode()
}

// JumpScopeKey registers a key pattern that enters jump mode scoped to the given
// regions. The scoped counterpart of JumpKey.
func (a *App) JumpScopeKey(pattern string, rects ...*NodeRef) *App {
	a.router.Handle(pattern, func(_ riffkey.Match) {
		a.EnterJumpScope(rects...)
	})
	return a
}

// SetJumpStyle sets the global style for jump labels.
func (a *App) SetJumpStyle(style JumpStyle) *App {
	a.jumpStyle = style
	return a
}

// JumpStyle returns the current jump style.
func (a *App) JumpStyle() JumpStyle {
	return a.jumpStyle
}

// JumpModeActive returns true if jump mode is currently active. Safe to call
// from any goroutine; reads the active flag under the jump lock.
//
// Do NOT call this from inside Execute (the render path) — use the per-frame
// a.frameJumpActive flag captured at render start instead, so a render that is
// mid-build doesn't re-lock and so the active state is consistent for the frame.
func (a *App) JumpModeActive() bool {
	return a.jumpMode != nil && a.jumpMode.isActive()
}

// JumpMode returns the jump mode state for use during rendering.
func (a *App) JumpMode() *JumpMode {
	return a.jumpMode
}

func (a *App) paintJumpLabels(buf *Buffer, height int) {
	if a.jumpMode == nil {
		return
	}
	// incremental feedback: the typed prefix recedes (matchedStyle) so the next
	// key stands out, and labels whose prefix no longer matches the input recede
	// whole. input is byte length — labels are ASCII home-row, so byte index ==
	// rune index here; slice on rune boundaries if labels ever leave ASCII.
	targets, input := a.jumpMode.snapshot()
	n := len(input)
	for _, target := range targets {
		x, y := int(target.X), int(target.Y)
		if y < 0 || y >= height || x >= buf.Width() {
			continue
		}
		base := a.jumpStyle.LabelStyle
		if !target.Style.Equal(Style{}) {
			base = target.Style
		}
		matched := a.jumpStyle.MatchedStyle
		if matched.Equal(Style{}) {
			matched = dimDerived(base) // default-on: derive a dim
		}
		// a label whose prefix diverged from the input is no longer a
		// candidate — dim the whole thing so the live targets stand out
		dead := n > 0 && !strings.HasPrefix(target.Label, input)
		for i, r := range target.Label {
			if x+i < 0 || x+i >= buf.Width() {
				continue
			}
			style := base
			if dead || i < n {
				style = matched
			}
			buf.Set(x+i, y, Cell{Rune: r, Style: style})
		}
	}
}

// EnterJumpMode activates jump label mode.
// A render is triggered to collect jump targets, then a temporary router
// is pushed to handle label input.
func (a *App) EnterJumpMode() {
	if a.jumpMode == nil {
		a.jumpMode = &JumpMode{}
	}
	if a.jumpMode.isActive() {
		return // Already in jump mode
	}

	a.jumpMode.setActive(true)
	a.jumpMode.ClearJumpTargets()

	// Render collects visible targets, assigns labels, and paints them.
	a.render()

	if a.jumpMode.targetCount() == 0 {
		// No targets (e.g. scoped to a region with none), exit immediately
		a.jumpMode.setActive(false)
		a.jumpMode.setScope(nil)
		return
	}

	// Create temporary router for jump input. Labels are NOT registered as
	// riffkey Handle sequences: a multi-char label ("as") would make riffkey
	// buffer the first key as a pending sequence prefix, so HandleUnmatched
	// (where the typed input is accumulated) would never fire mid-sequence and
	// the incremental feedback could never engage. Instead every key flows
	// through HandleUnmatched and we match against the labels ourselves, so
	// jumpMode.Input is the single source of truth for both selection and the
	// typed-prefix feedback.
	jumpRouter := riffkey.NewRouter().NoCounts()

	jumpRouter.Handle("<Esc>", func(_ riffkey.Match) {
		a.ExitJumpMode()
	})

	jumpRouter.HandleUnmatched(func(k riffkey.Key) bool {
		// backspace undoes the last typed key, restoring the previous label set;
		// on empty input it cancels jump mode.
		if k.Special == riffkey.SpecialBackspace {
			if _, ok := a.jumpMode.backspaceInput(); ok {
				a.RequestRender()
				return true
			}
			a.ExitJumpMode()
			return true
		}
		if k.Rune == 0 || k.Mod != riffkey.ModNone {
			a.ExitJumpMode()
			return true
		}
		input := a.jumpMode.appendInput(string(k.Rune))
		// exact label → select
		if onSelect, ok := a.jumpMode.selectTarget(input); ok {
			a.ExitJumpMode()
			if onSelect != nil {
				onSelect()
			}
			return true
		}
		// partial label → keep waiting; the typed prefix now recedes
		if a.jumpMode.partialMatch(input) {
			a.RequestRender()
			return true
		}
		// no match → cancel
		a.ExitJumpMode()
		return true
	})

	a.input.Push(jumpRouter)
}

// ExitJumpMode deactivates jump label mode.
func (a *App) ExitJumpMode() {
	if !a.JumpModeActive() {
		return
	}

	a.jumpMode.setActive(false)
	a.jumpMode.ClearJumpTargets()
	a.jumpMode.setScope(nil)
	a.input.Pop()
	a.RequestRender()
}

// AddJumpTarget registers a jump target during rendering, called by Jump
// components during Execute. The active gate is a lock-free atomic read;
// buildTarget appends to the render-goroutine-only scratch (published into
// Targets by AssignLabels at end of render). No jump lock is taken here, so it's
// safe to call from a consumer's layer Render mid-Execute.
func (a *App) AddJumpTarget(x, y int16, onSelect func(), style Style) {
	if a.jumpMode == nil || !a.jumpMode.isActive() {
		return
	}
	if !a.jumpMode.inScope(int(x), int(y)) {
		return // outside the active jump scope
	}
	a.jumpMode.buildTarget(x, y, onSelect, style)
}

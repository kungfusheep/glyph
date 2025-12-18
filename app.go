package tui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"riffkey"
)

// Debug timing
var (
	DebugTiming      bool
	lastBuildTime    time.Duration
	lastLayoutTime   time.Duration
	lastRenderTime   time.Duration
	lastFlushTime    time.Duration
)

// App is a TUI application with integrated input handling via riffkey.
type App struct {
	screen *Screen
	root   Component

	// riffkey integration
	router *riffkey.Router
	input  *riffkey.Input
	reader *riffkey.Reader

	// Build function - called before each render to rebuild UI
	buildFunc func() Component

	// State
	running     bool
	needsRender bool
	renderMu    sync.Mutex
	renderChan  chan struct{}
}

// NewApp creates a new TUI application.
func NewApp() (*App, error) {
	screen, err := NewScreen(nil)
	if err != nil {
		return nil, err
	}

	router := riffkey.NewRouter()
	input := riffkey.NewInput(router)
	reader := riffkey.NewReader(os.Stdin)

	app := &App{
		screen:     screen,
		router:     router,
		input:      input,
		reader:     reader,
		renderChan: make(chan struct{}, 1),
	}

	return app, nil
}

// SetBuildFunc sets a function that rebuilds the UI tree before each render.
// This enables the "rebuild everything" pattern for reactive UIs.
func (a *App) SetBuildFunc(fn func() Component) *App {
	a.buildFunc = fn
	return a
}

// SetRoot sets the root component of the application.
func (a *App) SetRoot(root Component) *App {
	a.root = root
	return a
}

// Root returns the root component.
func (a *App) Root() Component {
	return a.root
}

// Screen returns the screen.
func (a *App) Screen() *Screen {
	return a.screen
}

// Router returns the riffkey router for advanced configuration.
func (a *App) Router() *riffkey.Router {
	return a.router
}

// Input returns the riffkey input for modal handling (push/pop).
func (a *App) Input() *riffkey.Input {
	return a.input
}

// Handle registers a key binding with a vim-style pattern.
// Examples: "j", "gg", "<C-c>", "<C-w>j", "<Up>"
func (a *App) Handle(pattern string, handler func(riffkey.Match)) *App {
	a.router.Handle(pattern, handler)
	return a
}

// HandleNamed registers a named key binding (for rebinding support).
func (a *App) HandleNamed(name, pattern string, handler func(riffkey.Match)) *App {
	a.router.HandleNamed(name, pattern, handler)
	return a
}

// Push pushes a new router onto the input stack (for modal input).
func (a *App) Push(r *riffkey.Router) {
	a.input.Push(r)
}

// Pop pops the current router from the input stack.
func (a *App) Pop() {
	a.input.Pop()
}

// RequestRender marks that a render is needed.
// Safe to call from any goroutine.
func (a *App) RequestRender() {
	select {
	case a.renderChan <- struct{}{}:
	default:
		// Already a render pending
	}
}

// render performs the actual render if needed.
func (a *App) render() {
	a.renderMu.Lock()
	defer a.renderMu.Unlock()

	var t0, t1, t2, t3 time.Time
	if DebugTiming {
		t0 = time.Now()
	}

	// Rebuild UI if we have a build function
	if a.buildFunc != nil {
		a.root = a.buildFunc()
	}

	if DebugTiming {
		t1 = time.Now()
		lastBuildTime = t1.Sub(t0)
	}

	if a.root != nil {
		size := a.screen.Size()
		a.screen.Clear()
		a.root.SetConstraints(size.Width, size.Height)

		if DebugTiming {
			t2 = time.Now()
			lastLayoutTime = t2.Sub(t1)
		}

		a.root.Render(a.screen.Buffer(), 0, 0)

		if DebugTiming {
			t3 = time.Now()
			lastRenderTime = t3.Sub(t2)
		}
	}

	a.screen.Flush()

	if DebugTiming && !t3.IsZero() {
		lastFlushTime = time.Since(t3)
	}
}

// TimingString returns a formatted timing string.
func TimingString() string {
	return fmt.Sprintf("build:%v layout:%v render:%v flush:%v",
		lastBuildTime.Round(time.Microsecond),
		lastLayoutTime.Round(time.Microsecond),
		lastRenderTime.Round(time.Microsecond),
		lastFlushTime.Round(time.Microsecond))
}

// Run starts the application. Blocks until Stop is called.
func (a *App) Run() error {
	a.running = true

	// Enter raw mode
	if err := a.screen.EnterRawMode(); err != nil {
		return err
	}
	defer a.screen.ExitRawMode()

	// Handle resize
	go a.handleResize()

	// Handle async render requests (from timers, data updates, etc)
	go a.handleRenderRequests()

	// Initial render
	a.render()

	// Run riffkey input loop
	// afterDispatch is called after every key - perfect for rendering
	err := a.input.Run(a.reader, func(handled bool) {
		if !a.running {
			return
		}
		// Always render after input (state may have changed)
		a.render()
	})

	// Normal termination via Stop() causes reader to return error
	if !a.running {
		return nil
	}
	return err
}

// handleRenderRequests processes async render requests.
func (a *App) handleRenderRequests() {
	for {
		select {
		case <-a.renderChan:
			if !a.running {
				return
			}
			a.render()
		}
	}
}

// Stop signals the application to stop.
func (a *App) Stop() {
	a.running = false
	// Close stdin to unblock the reader
	os.Stdin.Close()
}

// handleResize watches for terminal resize events.
func (a *App) handleResize() {
	for range a.screen.ResizeChan() {
		a.RequestRender()
	}
}

// Size returns the current screen size.
func (a *App) Size() Size {
	return a.screen.Size()
}

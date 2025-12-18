package tui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"riffkey"
)

// ArenaApp is a TUI application using the zero-alloc arena renderer
type ArenaApp struct {
	screen *Screen
	frame  *Frame

	// riffkey integration
	router *riffkey.Router
	input  *riffkey.Input
	reader *riffkey.Reader

	// Build function
	buildFunc func()

	// State
	running    bool
	renderChan chan struct{}
	renderMu   sync.Mutex
}

// Debug timing for arena
var (
	arenaDebugTiming   bool
	arenaBuildTime     time.Duration
	arenaLayoutTime    time.Duration
	arenaRenderTime    time.Duration
	arenaFlushTime     time.Duration
)

// NewArenaApp creates a new arena-based TUI application
func NewArenaApp() (*ArenaApp, error) {
	screen, err := NewScreen(nil)
	if err != nil {
		return nil, err
	}

	router := riffkey.NewRouter()
	input := riffkey.NewInput(router)
	reader := riffkey.NewReader(os.Stdin)

	// Pre-allocate frame with generous capacity
	frame := NewFrame(10000, 100000)

	return &ArenaApp{
		screen:     screen,
		frame:      frame,
		router:     router,
		input:      input,
		reader:     reader,
		renderChan: make(chan struct{}, 1),
	}, nil
}

// SetBuildFunc sets the UI build function
func (a *ArenaApp) SetBuildFunc(fn func()) *ArenaApp {
	a.buildFunc = fn
	return a
}

// Handle registers a key binding
func (a *ArenaApp) Handle(pattern string, handler func(riffkey.Match)) *ArenaApp {
	a.router.Handle(pattern, handler)
	return a
}

// RequestRender requests a render on the next frame
func (a *ArenaApp) RequestRender() {
	select {
	case a.renderChan <- struct{}{}:
	default:
	}
}

// Frame returns the current frame (for advanced usage)
func (a *ArenaApp) Frame() *Frame {
	return a.frame
}

// render performs the actual render
func (a *ArenaApp) render() {
	a.renderMu.Lock()
	defer a.renderMu.Unlock()

	var t0, t1, t2, t3 time.Time
	if arenaDebugTiming {
		t0 = time.Now()
	}

	// Build
	if a.buildFunc != nil {
		a.frame.Build(a.buildFunc)
	}

	if arenaDebugTiming {
		t1 = time.Now()
		arenaBuildTime = t1.Sub(t0)
	}

	// Layout
	size := a.screen.Size()
	a.frame.Layout(size.Width, size.Height)

	if arenaDebugTiming {
		t2 = time.Now()
		arenaLayoutTime = t2.Sub(t1)
	}

	// Render
	a.screen.Clear()
	a.frame.Render(a.screen.Buffer())

	if arenaDebugTiming {
		t3 = time.Now()
		arenaRenderTime = t3.Sub(t2)
	}

	// Flush
	a.screen.Flush()

	if arenaDebugTiming {
		arenaFlushTime = time.Since(t3)
	}
}

// Run starts the application
func (a *ArenaApp) Run() error {
	a.running = true

	if err := a.screen.EnterRawMode(); err != nil {
		return err
	}
	defer a.screen.ExitRawMode()

	// Handle resize
	go func() {
		for range a.screen.ResizeChan() {
			a.RequestRender()
		}
	}()

	// Handle async render requests
	go func() {
		for range a.renderChan {
			if !a.running {
				return
			}
			a.render()
		}
	}()

	// Initial render
	a.render()

	// Input loop
	return a.input.Run(a.reader, func(handled bool) {
		if !a.running {
			return
		}
		a.render()
	})
}

// Stop stops the application
func (a *ArenaApp) Stop() {
	a.running = false
	os.Stdin.Close()
}

// Size returns the screen size
func (a *ArenaApp) Size() Size {
	return a.screen.Size()
}

// ArenaTimingString returns timing info
func ArenaTimingString() string {
	return fmt.Sprintf("build:%v layout:%v render:%v flush:%v",
		arenaBuildTime.Round(time.Microsecond),
		arenaLayoutTime.Round(time.Microsecond),
		arenaRenderTime.Round(time.Microsecond),
		arenaFlushTime.Round(time.Microsecond))
}

// EnableArenaTiming enables timing display
func EnableArenaTiming() {
	arenaDebugTiming = true
}

// ATitle and AMap are now in arena.go

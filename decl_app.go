package tui

import (
	"os"
	"sync"

	"riffkey"
)

// DeclApp is a TUI application using the declarative UI system
type DeclApp struct {
	screen *Screen
	frame  *DFrame

	// riffkey integration
	router *riffkey.Router
	input  *riffkey.Input
	reader *riffkey.Reader

	// UI definition
	ui any

	// State
	running    bool
	renderChan chan struct{}
	renderMu   sync.Mutex
}

// NewDeclApp creates a new declarative TUI application
func NewDeclApp(ui any) (*DeclApp, error) {
	screen, err := NewScreen(nil)
	if err != nil {
		return nil, err
	}

	router := riffkey.NewRouter()
	input := riffkey.NewInput(router)
	reader := riffkey.NewReader(os.Stdin)

	frame := NewDFrame()

	app := &DeclApp{
		screen:     screen,
		frame:      frame,
		router:     router,
		input:      input,
		reader:     reader,
		ui:         ui,
		renderChan: make(chan struct{}, 1),
	}

	// Default key bindings for focus navigation
	router.Handle("<Tab>", func(m riffkey.Match) {
		app.frame.FocusNext()
		app.RequestRender()
	})
	router.Handle("<S-Tab>", func(m riffkey.Match) {
		app.frame.FocusPrev()
		app.RequestRender()
	})
	// Vim-style navigation
	router.Handle("j", func(m riffkey.Match) {
		app.frame.FocusNext()
		app.RequestRender()
	})
	router.Handle("k", func(m riffkey.Match) {
		app.frame.FocusPrev()
		app.RequestRender()
	})
	// Arrow key navigation
	router.Handle("<Down>", func(m riffkey.Match) {
		app.frame.FocusNext()
		app.RequestRender()
	})
	router.Handle("<Up>", func(m riffkey.Match) {
		app.frame.FocusPrev()
		app.RequestRender()
	})
	router.Handle("<CR>", func(m riffkey.Match) {
		if app.frame.IsFocusedInput() {
			app.enterInputMode()
		} else {
			app.frame.Activate()
		}
		app.RequestRender()
	})
	router.Handle("<Space>", func(m riffkey.Match) {
		if app.frame.IsFocusedInput() {
			app.enterInputMode()
		} else {
			app.frame.Activate()
		}
		app.RequestRender()
	})

	// Jump mode: ; to enter
	router.Handle(";", func(m riffkey.Match) {
		app.enterJumpMode()
	})

	return app, nil
}

// enterInputMode activates text input mode for the focused input
func (a *DeclApp) enterInputMode() {
	if !a.frame.IsFocusedInput() {
		return
	}

	// Move cursor to end when entering input mode
	a.frame.InputEnd()

	inputRouter := riffkey.NewRouter()

	// Exit input mode
	exitInput := func(m riffkey.Match) {
		a.input.Pop()
		a.RequestRender()
	}
	inputRouter.Handle("<Esc>", exitInput)
	inputRouter.Handle("<Tab>", func(m riffkey.Match) {
		a.input.Pop()
		a.frame.FocusNext()
		a.RequestRender()
	})
	inputRouter.Handle("<S-Tab>", func(m riffkey.Match) {
		a.input.Pop()
		a.frame.FocusPrev()
		a.RequestRender()
	})

	// Submit on Enter
	inputRouter.Handle("<CR>", func(m riffkey.Match) {
		a.frame.InputSubmit()
		a.input.Pop()
		a.RequestRender()
	})

	// Cursor movement
	inputRouter.Handle("<Left>", func(m riffkey.Match) {
		a.frame.InputMoveCursor(-1)
		a.RequestRender()
	})
	inputRouter.Handle("<Right>", func(m riffkey.Match) {
		a.frame.InputMoveCursor(1)
		a.RequestRender()
	})
	inputRouter.Handle("<Home>", func(m riffkey.Match) {
		a.frame.InputHome()
		a.RequestRender()
	})
	inputRouter.Handle("<End>", func(m riffkey.Match) {
		a.frame.InputEnd()
		a.RequestRender()
	})

	// Deletion
	inputRouter.Handle("<BS>", func(m riffkey.Match) {
		a.frame.InputBackspace()
		a.RequestRender()
	})
	inputRouter.Handle("<Del>", func(m riffkey.Match) {
		a.frame.InputDelete()
		a.RequestRender()
	})

	// Register all printable character handlers
	registerPrintableChars(inputRouter, func(ch rune) {
		a.frame.InputInsert(ch)
		a.RequestRender()
	})

	a.input.Push(inputRouter)
	a.RequestRender()
}

// enterJumpMode activates jump labels and pushes a new router context
func (a *DeclApp) enterJumpMode() {
	a.frame.EnterJumpMode()
	if !a.frame.InJumpMode() {
		return // no focusables
	}

	jumpRouter := riffkey.NewRouter()

	// Esc or ; to exit
	exitJump := func(m riffkey.Match) {
		a.frame.ExitJumpMode()
		a.input.Pop()
		a.RequestRender()
	}
	jumpRouter.Handle("<Esc>", exitJump)
	jumpRouter.Handle(";", exitJump)

	// Register each jump target directly by its label
	for _, target := range a.frame.JumpTargets() {
		target := target // capture for closure
		jumpRouter.Handle(target.Label, func(m riffkey.Match) {
			a.frame.ActivateNode(target.NodeIdx)
			a.frame.ExitJumpMode()
			a.input.Pop()
			a.RequestRender()
		})
	}

	a.input.Push(jumpRouter)
	a.RequestRender()
}

// Handle registers a key binding (automatically triggers render after handler)
func (a *DeclApp) Handle(pattern string, handler func(riffkey.Match)) *DeclApp {
	a.router.Handle(pattern, func(m riffkey.Match) {
		handler(m)
		a.RequestRender()
	})
	return a
}

// RequestRender requests a render on the next frame
func (a *DeclApp) RequestRender() {
	select {
	case a.renderChan <- struct{}{}:
	default:
	}
}

// Frame returns the current frame
func (a *DeclApp) Frame() *DFrame {
	return a.frame
}

// SetUI updates the UI definition
func (a *DeclApp) SetUI(ui any) {
	a.ui = ui
}

// render performs the actual render
func (a *DeclApp) render() {
	a.renderMu.Lock()
	defer a.renderMu.Unlock()

	// Execute declarative UI
	ExecuteInto(a.frame, a.ui)

	// Layout
	size := a.screen.Size()
	a.frame.Layout(int16(size.Width), int16(size.Height))

	// Render to buffer
	a.screen.Clear()
	a.frame.Render(a.screen.Buffer())

	// Flush to terminal
	a.screen.Flush()
}

// Run starts the application main loop
func (a *DeclApp) Run() error {
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
func (a *DeclApp) Stop() {
	a.running = false
	os.Stdin.Close()
}

// registerPrintableChars registers handlers for all common printable characters
func registerPrintableChars(r *riffkey.Router, handler func(ch rune)) {
	// Lowercase letters
	for ch := 'a'; ch <= 'z'; ch++ {
		ch := ch // capture
		r.Handle(string(ch), func(m riffkey.Match) { handler(ch) })
	}
	// Uppercase letters
	for ch := 'A'; ch <= 'Z'; ch++ {
		ch := ch
		r.Handle(string(ch), func(m riffkey.Match) { handler(ch) })
	}
	// Digits
	for ch := '0'; ch <= '9'; ch++ {
		ch := ch
		r.Handle(string(ch), func(m riffkey.Match) { handler(ch) })
	}
	// Common punctuation and symbols
	for _, ch := range " !\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~" {
		ch := ch
		pattern := string(ch)
		// Space needs special handling
		if ch == ' ' {
			pattern = "<Space>"
		}
		r.Handle(pattern, func(m riffkey.Match) { handler(ch) })
	}
}

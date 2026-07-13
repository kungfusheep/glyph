package main

import (
	"sync"
	"testing"

	. "github.com/kungfusheep/glyph"
	termpkg "github.com/kungfusheep/glyph/term"
)

// TestBoundStateRaceUnderConcurrentRender reproduces the production composition:
// glyph's render goroutine executes the template — iterating the bound chips
// slice — while a pty reader goroutine reports its shell exited and rewrites
// that same slice.
//
// The other tests render synchronously on the test goroutine, so they cannot see
// this. The real app always runs a render goroutine, which is why the race only
// shows up here.
func TestBoundStateRaceUnderConcurrentRender(t *testing.T) {
	const W, H = 60, 16

	// model glyph's apply queue: closures land on the render goroutine and are
	// drained at frame top, before Execute reads any bound state.
	applies := make(chan func(), 1024)
	u := newUI(
		func(fn func()) { applies <- fn },
		func() {}, func() {},
		func(tc *termpkg.TermC) { tc.Shell("/bin/sh").Env("PS1=", "TERM=dumb") },
	)
	drain := func() {
		for {
			select {
			case fn := <-applies:
				fn()
			default:
				return
			}
		}
	}
	u.resize(W, H)
	drain()
	tmpl := Build(u.view())
	t.Cleanup(func() {
		for _, p := range u.tree.leaves() {
			u.slots[p.slot].Close()
		}
	})

	u.split(true)
	u.split(false) // three panes, so the chip slice has something to churn
	drain()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() { // the render goroutine
		defer wg.Done()
		buf := NewBuffer(W, H)
		for {
			select {
			case <-stop:
				return
			default:
				drain() // frame top, as glyph does
				tmpl.Execute(buf, W, H)
			}
		}
	}()

	wg.Add(1)
	go func() { // a pty reader goroutine, doing what the exit callback does
		defer wg.Done()
		for i := 0; i < 300; i++ {
			u.focusNext() // mutates the tree and rewrites the bound chips slice
		}
		close(stop)
	}()

	wg.Wait()
}

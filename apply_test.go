package glyph

import (
	"sync"
	"testing"
)

// ADR 2 contract: closures run on the render thread at frame top, in apply
// order, exactly once.
func TestApplyRunsInOrderExactlyOnce(t *testing.T) {
	app := &App{}
	var got []int
	for i := 0; i < 5; i++ {
		i := i
		app.Apply(func() { got = append(got, i) })
	}
	app.render()
	if len(got) != 5 {
		t.Fatalf("ran %d closures, want 5", len(got))
	}
	for i, v := range got {
		if v != i {
			t.Fatalf("order broken: got[%d] = %d", i, v)
		}
	}
	app.render()
	if len(got) != 5 {
		t.Fatalf("closures re-ran on second frame: %d", len(got))
	}
}

// a closure applied during the drain lands in the NEXT batch — the
// anti-livelock invariant a self-applying chain depends on.
func TestApplyDuringDrainRunsNextFrame(t *testing.T) {
	app := &App{}
	runs := 0
	app.Apply(func() {
		runs++
		app.Apply(func() { runs++ })
	})
	app.render()
	if runs != 1 {
		t.Fatalf("apply-during-drain ran same frame: runs = %d, want 1", runs)
	}
	app.render()
	if runs != 2 {
		t.Fatalf("chained apply did not run next frame: runs = %d, want 2", runs)
	}
}

// concurrent producers against a rendering consumer: every closure runs
// exactly once and per-producer order is preserved.
func TestApplyConcurrentProducersExactlyOnce(t *testing.T) {
	app := &App{}
	const producers, perProducer = 8, 200

	var mu sync.Mutex
	seen := make(map[int][]int)

	var wg sync.WaitGroup
	wg.Add(producers)
	stop := make(chan struct{})
	var renderWg sync.WaitGroup
	renderWg.Add(1)
	go func() {
		defer renderWg.Done()
		for {
			select {
			case <-stop:
				app.render() // final drain
				return
			default:
				app.render()
			}
		}
	}()

	for p := 0; p < producers; p++ {
		p := p
		go func() {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				p, i := p, i
				app.Apply(func() {
					mu.Lock()
					seen[p] = append(seen[p], i)
					mu.Unlock()
				})
			}
		}()
	}
	wg.Wait()
	close(stop)
	renderWg.Wait()

	for p := 0; p < producers; p++ {
		vals := seen[p]
		if len(vals) != perProducer {
			t.Fatalf("producer %d: ran %d closures, want %d", p, len(vals), perProducer)
		}
		for i, v := range vals {
			if v != i {
				t.Fatalf("producer %d order broken at %d: %d", p, i, v)
			}
		}
	}
}

// TestStopFromApplyClosureDoesNotRaceRunLoop pins the threading contract around
// Stop. Stop is public and an Apply closure runs on the RENDER goroutine, so a
// closure that stops the app writes `running` while the Run loop is reading it.
// That is the exact shape glyph tells consumers to use: marshal work through
// Apply, and stop from there when the last pane's process exits.
//
// The race detector is the oracle here — this test exists to be run under
// -race, where a plain bool field fails and an atomic one does not.
func TestStopFromApplyClosureDoesNotRaceRunLoop(t *testing.T) {
	app := &App{
		renderChan:     make(chan struct{}, 1),
		nonInteractive: true, // Stop must not close os.Stdin under test
	}
	app.running.Store(true)

	// the render goroutine drains applies at frame top; one of them stops the app
	app.Apply(func() { app.Stop() })

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.drainApplies()
	}()

	// the Run loop's read of the same field, concurrently
	spins := 0
	for app.running.Load() && spins < 1_000_000 {
		spins++
	}
	<-done

	if app.running.Load() {
		t.Fatal("Stop() from an Apply closure did not stop the run loop")
	}
}

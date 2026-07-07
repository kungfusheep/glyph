package glyph

import (
	"testing"
	"unsafe"
)

// orphaned keys (from a reallocated ForEach slice) must be swept so the map stays
// bounded. Eviction is round-relative (age >= one full round of accesses), so it is
// EVENTUAL, not instant: a fresh burst survives its own round, then dies once the
// live set is re-touched for a few rounds. (Cap-relative age was the old contract; it
// falsely evicted live entries every frame on lists larger than the cap — churn.)
func TestPerItemCacheEvictsOrphans(t *testing.T) {
	var c perItemCache[int]
	keys := make([]int, perItemCacheCap*3) // distinct real addresses as fake elemBases
	for i := range keys {
		v := i
		c.getOrCreate(unsafe.Pointer(&keys[i]), func() *int { return &v })
	}
	// the last generation is the live set now; earlier generations are orphans.
	live := keys[perItemCacheCap*2:]
	for round := 0; round < 8; round++ {
		for i := range live {
			c.getOrCreate(unsafe.Pointer(&live[i]), func() *int { return new(int) })
		}
	}
	if len(c.m) > perItemCacheCap*2 {
		t.Fatalf("unbounded: %d entries (live %d) — orphaned keys not evicted within bounded rounds", len(c.m), len(live))
	}
	// and the live set survived the sweeps
	for i := range live {
		if c.peek(unsafe.Pointer(&live[i])) == nil {
			t.Fatalf("live key %d evicted by the orphan sweep", i)
		}
	}
}

// a live set LARGER than the cap, touched every round, must not churn: entries stay
// stable (no re-creation) across frames. This was the defect the round-relative
// horizon fixes — cap-relative age evicted every entry of a >cap list every frame.
func TestPerItemCacheNoChurnOnLargeLiveSet(t *testing.T) {
	var c perItemCache[int]
	live := make([]int, perItemCacheCap*2) // 2x the cap, all live
	// warm-up: initial population plus a couple of rounds while sweep boundaries
	// settle (a handful of just-created entries can be clipped at the first sweeps)
	for round := 0; round < 3; round++ {
		for i := range live {
			c.getOrCreate(unsafe.Pointer(&live[i]), func() *int { return new(int) })
		}
	}
	// steady state: every access must hit — zero re-creation, every round.
	creates := 0
	for round := 0; round < 5; round++ {
		for i := range live {
			c.getOrCreate(unsafe.Pointer(&live[i]), func() *int { creates++; return new(int) })
		}
	}
	if creates != 0 {
		t.Fatalf("large live set churned in steady state: %d creates for %d live keys over 5 rounds", creates, len(live))
	}
}

// a live working set re-touched every frame must NEVER be evicted, even while a stream of
// orphan keys churns through — this is the "safe by construction: never evicts a live item"
// guarantee the exit-animation case relies on (an exiting item is touched every frame).
func TestPerItemCacheKeepsLiveKeysUnderChurn(t *testing.T) {
	var c perItemCache[int]
	live := make([]int, 64) // the on-screen working set
	for i := range live {
		v := i
		c.getOrCreate(unsafe.Pointer(&live[i]), func() *int { return &v })
	}
	// many frames: each frame re-touches the live set, then a realloc orphans fresh keys.
	for frame := 0; frame < 50; frame++ {
		for i := range live {
			if c.getOrCreate(unsafe.Pointer(&live[i]), func() *int { return new(int) }) == nil {
				t.Fatalf("frame %d: live key %d returned nil — evicted while live", frame, i)
			}
		}
		orphans := make([]int, 64)
		for i := range orphans {
			v := i
			c.getOrCreate(unsafe.Pointer(&orphans[i]), func() *int { return &v })
		}
	}
	for i := range live {
		if c.peek(unsafe.Pointer(&live[i])) == nil {
			t.Fatalf("live key %d was evicted across churn — live-safety invariant broken", i)
		}
	}
}

// peek must NOT stamp the access seq: a secondary read of an already-live item keeps the
// cap invariant at 1 stamping-access/item/frame, and a peek alone must not keep an orphan alive.
func TestPerItemCachePeekDoesNotStamp(t *testing.T) {
	var c perItemCache[int]
	v := 1
	var key int
	c.getOrCreate(unsafe.Pointer(&key), func() *int { return &v })
	before := c.seq
	if c.peek(unsafe.Pointer(&key)) == nil {
		t.Fatal("peek returned nil for present key")
	}
	if c.seq != before {
		t.Fatalf("peek advanced seq %d -> %d; must be non-stamping", before, c.seq)
	}
	if c.peek(unsafe.Pointer(&v)) != nil { // absent key
		t.Fatal("peek returned non-nil for absent key")
	}
}

// get/set is the markdown path: get stamps + returns the entry (nil if absent), set stores
// reusing the get's seq so the pair is one access.
func TestPerItemCacheGetSet(t *testing.T) {
	var c perItemCache[int]
	var key int
	if c.get(unsafe.Pointer(&key)) != nil {
		t.Fatal("get on empty cache should be nil")
	}
	v := 42
	c.set(unsafe.Pointer(&key), &v)
	got := c.get(unsafe.Pointer(&key))
	if got == nil || *got != 42 {
		t.Fatalf("get after set = %v, want 42", got)
	}
}

// the steady-state hit (the per-frame hot path) must be alloc-free — performance is a feature.
func TestPerItemCacheHitAllocFree(t *testing.T) {
	var c perItemCache[int]
	var key int
	v := 0
	c.getOrCreate(unsafe.Pointer(&key), func() *int { return &v })
	if allocs := testing.AllocsPerRun(1000, func() {
		c.getOrCreate(unsafe.Pointer(&key), func() *int { return &v })
	}); allocs != 0 {
		t.Fatalf("steady-state getOrCreate hit allocates %.1f/op, want 0", allocs)
	}
}

func BenchmarkPerItemCacheGetOrCreateHit(b *testing.B) {
	var c perItemCache[int]
	var key int
	v := 0
	c.getOrCreate(unsafe.Pointer(&key), func() *int { return &v })
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.getOrCreate(unsafe.Pointer(&key), func() *int { return &v })
	}
}

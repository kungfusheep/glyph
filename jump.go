package glyph

import (
	"sync"
	"sync/atomic"
)

// JumpStyle configures the appearance of jump labels.
type JumpStyle struct {
	LabelStyle Style // Style for the not-yet-typed remainder of a label

	// MatchedStyle paints the already-typed prefix of a multi-char label, and
	// the whole label once it no longer matches the input, so progress is
	// visible mid-sequence. A zero MatchedStyle derives one from LabelStyle. At
	// rest the render is identical to LabelStyle alone.
	MatchedStyle Style
}

// dimDerived returns a receded variant of a style for the matched/dead label
// feedback. It drops the background band, greys the foreground and drops bold,
// so a receded label is plain text rather than a coloured label — a categorical
// change that reads on any theme, where a faint attribute or a foreground tweak
// alone can be swamped by an explicit label colour.
func dimDerived(s Style) Style {
	s.BG = Color{}
	s.FG = BrightBlack
	s.Attr = (s.Attr &^ AttrBold) | AttrDim
	return s
}

// DefaultJumpStyle is the default styling for jump labels.
var DefaultJumpStyle = JumpStyle{
	LabelStyle: Style{FG: Magenta, Attr: AttrBold},
}

// JumpTarget represents a single jumpable location.
type JumpTarget struct {
	X, Y     int16
	Label    string
	OnSelect func()
	Style    Style // Per-target override (zero value = use default)
}

// JumpMode holds the state for jump label mode.
//
// State is shared across two goroutines: the input goroutine (EnterJumpMode, key
// handlers) and the frame-timer render goroutine (target rebuild via
// render→Execute). The split that keeps it correct AND deadlock-free:
//
//   - active is an atomic.Bool, so JumpModeActive() is a lock-free read. Consumer
//     code (a custom layer's Render callback, e.g. recap's diff view) calls it
//     DURING the main render's Execute — a locking accessor there would re-enter
//     and deadlock, since render is the same goroutine. atomic dodges that.
//   - Targets/Input/ScopeRects are guarded by mu, but mu is NEVER held across
//     Execute. Per-frame targets accumulate into building (render-goroutine-only,
//     no lock — renders are serialised by renderMu) and are swapped into Targets
//     in one locked step by AssignLabels, so a concurrent reader (the input
//     goroutine's len-check / FindTarget) never observes Targets transiently
//     empty mid-rebuild — the #400 jump-pick no-op.
type JumpMode struct {
	active atomic.Bool

	mu       sync.Mutex
	Targets  []JumpTarget // committed, labelled set; read under mu
	building []JumpTarget // render-goroutine scratch; swapped into Targets by AssignLabels
	Input    string       // accumulated input for multi-char labels
	// ScopeRects restricts collection to one or more screen regions (a pane's
	// rendered NodeRef). Empty means the whole screen is in scope.
	ScopeRects []*NodeRef
}

func (jm *JumpMode) isActive() bool  { return jm.active.Load() }
func (jm *JumpMode) setActive(b bool) { jm.active.Store(b) }

func (jm *JumpMode) setScope(rects []*NodeRef) {
	jm.mu.Lock()
	jm.ScopeRects = rects
	jm.mu.Unlock()
}

func (jm *JumpMode) targetCount() int {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	return len(jm.Targets)
}

// snapshot copies the labelled targets and current input for the paint pass, so
// painting reads a consistent set without holding the lock across buffer writes.
func (jm *JumpMode) snapshot() ([]JumpTarget, string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	out := make([]JumpTarget, len(jm.Targets))
	copy(out, jm.Targets)
	return out, jm.Input
}

// appendInput adds typed runes to the multi-char label buffer, returning the
// resulting input so the (single) input goroutine can match without a second lock.
func (jm *JumpMode) appendInput(s string) string {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	jm.Input += s
	return jm.Input
}

// backspaceInput drops the last typed byte; ok is false when the buffer is empty.
func (jm *JumpMode) backspaceInput() (input string, ok bool) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if len(jm.Input) == 0 {
		return "", false
	}
	jm.Input = jm.Input[:len(jm.Input)-1] // ASCII labels: byte == rune
	return jm.Input, true
}

// selectTarget returns the OnSelect for an exact label match, under the lock.
func (jm *JumpMode) selectTarget(label string) (func(), bool) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	for i := range jm.Targets {
		if jm.Targets[i].Label == label {
			return jm.Targets[i].OnSelect, true
		}
	}
	return nil, false
}

// partialMatch reports whether any label still extends the typed prefix.
func (jm *JumpMode) partialMatch(prefix string) bool {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	for _, t := range jm.Targets {
		if len(t.Label) > len(prefix) && t.Label[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// buildTarget appends to the per-frame scratch during Execute. Render-goroutine
// only (renders are serialised by renderMu), so no lock; AssignLabels publishes
// the batch into Targets under mu.
func (jm *JumpMode) buildTarget(x, y int16, onSelect func(), style Style) {
	jm.building = append(jm.building, JumpTarget{X: x, Y: y, OnSelect: onSelect, Style: style})
}

// inScope reports whether the screen position (x,y) is within the active jump
// scope. With no scope rects the whole screen is in scope. Bounds are half-open;
// an empty rect (W<=0 || H<=0, e.g. a pane not rendered this frame) matches
// nothing, so scoping to an unrendered pane simply yields no targets.
func (jm *JumpMode) inScope(x, y int) bool {
	jm.mu.Lock()
	rects := jm.ScopeRects
	jm.mu.Unlock()
	return inScopeRects(rects, x, y)
}

func inScopeRects(rects []*NodeRef, x, y int) bool {
	if len(rects) == 0 {
		return true
	}
	for _, r := range rects {
		if r == nil || r.W <= 0 || r.H <= 0 {
			continue
		}
		if x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
			return true
		}
	}
	return false
}

// labelChars are the characters used for jump labels.
// Home row keys first for ergonomics, then other letters.
var labelChars = []rune{
	'a', 's', 'd', 'f', 'g', 'h', 'j', 'k', 'l',
	'q', 'w', 'e', 'r', 't', 'y', 'u', 'i', 'o', 'p',
	'z', 'x', 'c', 'v', 'b', 'n', 'm',
}

// GenerateLabels creates n unique labels for jump targets.
// For small sets (<=27): single chars (a, s, d, f, ...)
// For larger sets: two chars (aa, as, ad, ...)
func GenerateLabels(n int) []string {
	if n <= 0 {
		return nil
	}

	labels := make([]string, n)

	if n <= len(labelChars) {
		// Single character labels
		for i := 0; i < n; i++ {
			labels[i] = string(labelChars[i])
		}
	} else {
		// Two character labels
		idx := 0
		for _, first := range labelChars {
			for _, second := range labelChars {
				if idx >= n {
					return labels
				}
				labels[idx] = string(first) + string(second)
				idx++
			}
		}
	}

	return labels
}

// ClearJumpTargets resets the committed targets and the input. It does NOT touch
// building — that scratch belongs solely to the render goroutine (reset each frame
// by ClearTargets); touching it from the input goroutine would race the rebuild.
func (jm *JumpMode) ClearJumpTargets() {
	jm.mu.Lock()
	jm.Targets = jm.Targets[:0]
	jm.Input = ""
	jm.mu.Unlock()
}

// ClearTargets resets the per-frame build scratch ahead of a fresh Execute.
// Render-goroutine only; the committed Targets are untouched until AssignLabels
// swaps the freshly-built set in, so readers keep seeing the last good set.
func (jm *JumpMode) ClearTargets() { jm.building = jm.building[:0] }

// AddTarget appends a target to the committed set directly, under the lock.
// (Kept for any external caller; the render path uses buildTarget + AssignLabels.)
func (jm *JumpMode) AddTarget(x, y int16, onSelect func(), style Style) {
	jm.mu.Lock()
	jm.Targets = append(jm.Targets, JumpTarget{X: x, Y: y, OnSelect: onSelect, Style: style})
	jm.mu.Unlock()
}

// AssignLabels publishes the frame's built targets: swap building→Targets, label
// them, and reset the scratch — all in one locked step so the swap is atomic to a
// concurrent reader (Targets never seen empty mid-rebuild).
func (jm *JumpMode) AssignLabels() {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	jm.Targets = append(jm.Targets[:0], jm.building...)
	jm.building = jm.building[:0]
	labels := GenerateLabels(len(jm.Targets))
	for i := range jm.Targets {
		jm.Targets[i].Label = labels[i]
	}
}

// FindTarget finds a target by its label.
func (jm *JumpMode) FindTarget(label string) *JumpTarget {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	for i := range jm.Targets {
		if jm.Targets[i].Label == label {
			return &jm.Targets[i]
		}
	}
	return nil
}

// HasPartialMatch checks if any target label starts with the given prefix.
func (jm *JumpMode) HasPartialMatch(prefix string) bool {
	return jm.partialMatch(prefix)
}

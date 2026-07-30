package glyph

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// Layer is a pre-rendered buffer with scroll management.
// Content is rendered once (expensive), then blitted to screen each frame (cheap).
//
// If Render is set, the framework automatically calls it before blitting when
// the viewport dimensions change. This ensures content is always rendered at
// the correct size without manual timing coordination.
type Layer struct {
	// scrollMu guards the scroll/viewport state below — scrollY, maxScroll,
	// viewWidth, viewHeight. render() writes them (SetViewport→updateMaxScroll,
	// blit reads scrollY) on the frame goroutine while input handlers scroll
	// (ScrollTo/ScrollDown/PageDown/…) and read ViewportHeight/ScrollY on another.
	// NEVER hold scrollMu across the Render callback: consumer Render code calls
	// back into ScrollY()/ScrollTo()/ViewportWidth() (a consumer's diff/scroll layer), so
	// holding it there would re-enter and deadlock.
	scrollMu  sync.Mutex
	buffer    *Buffer
	scrollY   int
	maxScroll int

	// Viewport dimensions (set during layout)
	viewWidth  int
	viewHeight int

	// Track dimensions at last render to detect when re-render needed
	lastRenderWidth  int
	lastRenderHeight int
	renderDirty      atomic.Bool

	// Cursor state (buffer-relative coordinates)
	cursor Cursor

	// Screen offset (set by framework during blit for cursor translation)
	screenX, screenY int

	// Render populates the layer buffer. Called automatically by the framework
	// before blitting when viewport dimensions change. The layer ensures its
	// buffer exists and is sized appropriately before calling this.
	//
	// Width changes always trigger a re-render (text wrapping changes).
	// Height changes trigger a re-render if content height depends on viewport.
	Render func()

	// AlwaysRender causes Render to fire every frame, not just on width changes.
	// Used by components that track external pointer mutations (e.g. TextViewC).
	AlwaysRender bool

	// defaultStyle inherited from the app for buffer creation
	defaultStyle Style
	app          *App

	// feather > 0 fades that many rows toward the background at an overflowing edge:
	// the top edge when scrolled down from the top, the bottom edge when not yet at the
	// end. 0 (default) leaves blit byte-for-byte unchanged.
	feather int

	// Bound scroll offset + easing machinery (ADR 38), grouped so the scroll
	// concern reads as one thing rather than a dozen scroll*-prefixed fields on
	// Layer. See scrollEase.
	ease scrollEase
}

// scrollEase is the ADR 38 bound-offset + easing state for a Layer. When target is
// non-nil it is the SINGLE source of truth for the scroll position: every scroll
// method writes it (clamped), and blit reads the eased displayed value moving toward
// it — dissolving the stale-pending vs manual-scroll race by construction (one value,
// last-write-wins). A nil target keeps the legacy scrollY path. dur 0 = instant; >0
// eases the displayed offset toward the target over that duration with fn.
type scrollEase struct {
	target    *int
	dur       time.Duration
	fn        func(float64) float64
	shown     float64 // current displayed offset (eased); valid once shownSet
	shownSet  bool
	animFrom  float64   // displayed value when the current ease began
	animT0    time.Time // when the current ease began
	animTo    int       // target the current ease heads to (detects retargets)
	animating bool
	nowFn     func() time.Time // clock hook for tests; nil = time.Now
}

// clock returns the ease's clock (real time, or a test hook). The ease OWNS it: a second
// accessor over the same field is what made Layer.now() call itself through nowFn and
// overflow the stack on the only path with no test clock installed.
func (e *scrollEase) clock() time.Time {
	if e.nowFn != nil {
		return e.nowFn()
	}
	return time.Now()
}

// arm binds target as the source of truth for the eased offset, seeding the displayed
// position from cur so the first frame does not jump. A nil target unarms and reports
// the position to resume at, so the caller can restore its own unbound field.
//
// It is the driver-agnostic half of Layer.armScrollOffset: the List seam arms the same
// state machine over its own row offset (ADR 128).
func (e *scrollEase) arm(target *int, dur time.Duration, fn func(float64) float64, cur int) (resumeAt int, unarmed bool) {
	if target == nil {
		if e.target != nil {
			resumeAt = *e.target
			e.target = nil
			e.animating = false
			e.shownSet = false
			return resumeAt, true
		}
		return 0, false
	}
	if e.target != target {
		if e.target != nil {
			*target = *e.target
		} else {
			*target = cur
		}
		e.target = target
	}
	// duration and easing are configuration, not position — always the latest spelling
	e.dur = dur
	e.fn = fn
	return 0, false
}

// observe reports where the content is drawn RIGHT NOW without advancing anything, so
// a reader on another goroutine cannot perturb animation timing.
func (e *scrollEase) observe(maxScroll int) int {
	if !e.shownSet {
		return clampInt(*e.target, 0, maxScroll)
	}
	return int(math.Round(e.shown))
}

// advance moves the ease one frame toward its target and returns the offset to draw at.
// It writes the clamp back to the target so a content grow snaps rather than leaving the
// target out of range. animating is left true while an ease is in flight.
func (e *scrollEase) advance(maxScroll int) int {
	target := clampInt(*e.target, 0, maxScroll)
	*e.target = target // write back the clamp (grow-snap guard)

	if e.dur <= 0 || !e.shownSet {
		e.shown = float64(target)
		e.shownSet = true
		e.animating = false
		return target
	}
	// (re)start an ease when the target moves away from where we're shown/heading.
	if int(math.Round(e.shown)) == target {
		e.animating = false
		e.shown = float64(target)
		return target
	}
	if !e.animating || e.animTo != target {
		e.animFrom = e.shown
		e.animT0 = e.clock()
		e.animTo = target
		e.animating = true
	}
	p := float64(e.clock().Sub(e.animT0)) / float64(e.dur)
	if p >= 1 {
		e.shown = float64(target)
		e.animating = false
		return target
	}
	if e.fn != nil {
		p = e.fn(p)
	}
	e.shown = e.animFrom + p*(float64(target)-e.animFrom)
	return int(math.Round(e.shown))
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// NewLayer creates a new empty layer.
func NewLayer() *Layer {
	return &Layer{}
}

// SetContent renders a template to the layer's internal buffer.
// Call this when content changes (e.g., page navigation).
func (l *Layer) SetContent(tmpl *Template, width, height int) {
	l.buffer = NewBuffer(width, height)
	tmpl.Execute(l.buffer, int16(width), int16(height))
	l.renderDirty.Store(false)
	l.scrollMu.Lock()
	l.resetScrollLocked()
	l.updateMaxScroll()
	l.scrollMu.Unlock()
}

// SetBuffer directly sets the layer's buffer.
// Use this if you're managing the buffer yourself.
func (l *Layer) SetBuffer(buf *Buffer) {
	l.buffer = buf
	l.renderDirty.Store(false)
	l.scrollMu.Lock()
	l.scrollY = 0
	l.updateMaxScroll()
	l.scrollMu.Unlock()
}

// resetScrollLocked puts a new document at the top; caller holds scrollMu. Resets the
// PAIR — target, drawn offset and any ease in flight — not just the legacy field: an
// armed pane that only zeroed the latter would open every new document at the previous
// one's offset, and page navigation is what those panes do.
//
// SetContent only. SetBuffer is the RE-RENDER path (LogC on every append, TextViewC,
// ScrollView itself) where resetting would yank an armed pane to the top on a refresh;
// ScrollView restores around it, LogC does not.
func (l *Layer) resetScrollLocked() {
	l.scrollY = 0
	if l.ease.target != nil {
		*l.ease.target = 0
	}
	l.ease.shown = 0
	l.ease.shownSet = false
	l.ease.animating = false
}

// Buffer returns the underlying buffer (for direct manipulation if needed).
func (l *Layer) Buffer() *Buffer {
	return l.buffer
}

// updateMaxScroll recalculates the maximum scroll position.
func (l *Layer) updateMaxScroll() {
	if l.buffer == nil || l.viewHeight <= 0 {
		l.maxScroll = 0
		return
	}
	l.maxScroll = l.buffer.Height() - l.viewHeight
	if l.maxScroll < 0 {
		l.maxScroll = 0
	}
	// Clamp current scroll to new bounds. For a bound offset, clamp the target itself so
	// it never sits out of range and snaps when content later grows (ADR 38 grow-guard).
	if l.ease.target != nil {
		if *l.ease.target > l.maxScroll {
			*l.ease.target = l.maxScroll
		}
		if *l.ease.target < 0 {
			*l.ease.target = 0
		}
	} else if l.scrollY > l.maxScroll {
		l.scrollY = l.maxScroll
	}
}

// SetViewport sets the viewport dimensions for the layer.
// Called internally by the framework during layout.
func (l *Layer) SetViewport(width, height int) {
	l.scrollMu.Lock()
	l.viewWidth = width
	l.viewHeight = height
	l.updateMaxScroll() // unlocked inner; we hold scrollMu
	l.scrollMu.Unlock()
}

// SetFeather fades n rows toward the layer background at an overflowing edge: the top
// when scrolled down from the top, the bottom when there is more below. The fade
// appears only where content actually overflows, so it doubles as a scroll-position
// cue. n == 0 (the default) leaves blit byte-for-byte unchanged — non-feathered
// layers pay nothing. Blends toward the layer's background (inherited from the app's
// default style); with a terminal-default background it is a no-op.
func (l *Layer) SetFeather(n int) *Layer {
	l.feather = n
	return l
}

// Feather returns the current edge-fade depth (0 = off).
func (l *Layer) Feather() int { return l.feather }

// NeedsRender returns true if the layer needs to re-render before blitting.
// Width changes always require re-render (text wrapping). Height changes
// require re-render if this is the first render or content is height-dependent.
func (l *Layer) NeedsRender() bool {
	if l.Render == nil {
		return false
	}
	l.scrollMu.Lock()
	vw, vh, lrw, lrh := l.viewWidth, l.viewHeight, l.lastRenderWidth, l.lastRenderHeight
	l.scrollMu.Unlock()
	return l.AlwaysRender || l.renderDirty.Load() || lrw == 0 || lrw != vw || lrh != vh
}

// Invalidate marks the layer content dirty so Render runs on the next display
// pass. This is a non-blocking signal; the render thread still owns the actual
// buffer/template rebuild.
func (l *Layer) Invalidate() {
	l.renderDirty.Store(true)
}

// prepare ensures the layer is ready to blit. Called by the framework before
// blitting. If Render is set and dimensions changed, calls Render automatically.
func (l *Layer) prepare() {
	if !l.NeedsRender() {
		return
	}
	l.scrollMu.Lock()
	l.lastRenderWidth = l.viewWidth
	l.lastRenderHeight = l.viewHeight
	l.scrollMu.Unlock()
	l.renderDirty.Store(false)
	// Render is consumer code that calls back into ScrollY()/ScrollTo()/
	// ViewportWidth() — must run with NO scroll lock held, or it re-enters.
	l.Render()
}

// ScrollY returns the current scroll position. When an offset is bound this is the
// logical target (where scrolling is headed), clamped — not the mid-animation displayed
// value — so consumers checking "am I near the bottom?" see the destination.
func (l *Layer) ScrollY() int {
	l.scrollMu.Lock()
	defer l.scrollMu.Unlock()
	if l.ease.target != nil {
		y := *l.ease.target
		if y < 0 {
			y = 0
		}
		if y > l.maxScroll {
			y = l.maxScroll
		}
		return y
	}
	return l.scrollY
}

// MaxScroll returns the maximum scroll position.
func (l *Layer) MaxScroll() int {
	l.scrollMu.Lock()
	defer l.scrollMu.Unlock()
	return l.maxScroll
}

// ContentHeight returns the total content height.
func (l *Layer) ContentHeight() int {
	if l.buffer == nil {
		return 0
	}
	return l.buffer.Height()
}

// ViewportHeight returns the visible viewport height.
func (l *Layer) ViewportHeight() int {
	l.scrollMu.Lock()
	defer l.scrollMu.Unlock()
	return l.viewHeight
}

// ViewportWidth returns the visible viewport width.
func (l *Layer) ViewportWidth() int {
	l.scrollMu.Lock()
	defer l.scrollMu.Unlock()
	return l.viewWidth
}

// armScrollOffset binds offset as the layer's scroll cell (ADR 38): a *int is instant, an
// Animate tween over one eases the displayed offset toward it. Binding a cell the layer
// isn't already using seeds it from the position in effect, so arming a scrolled layer
// holds its place instead of snapping to the top. The guard is on pointer IDENTITY, not
// value — a fresh cell and a live one both read 0 — which makes the re-arm every rebuild
// performs a genuine no-op rather than one that merely looks like it.
//
// A nil or wrong-typed offset UNARMS: dropping the offset on a rebuild (or passing nil)
// writes the layer's position back to the legacy field and clears the cell, so the layer
// resumes on the legacy path. On an unarmed layer this is a no-op, so a view that never
// armed can't be broken by the compile-time call. Unarm writes the DESTINATION the layer
// was heading for (the target), not the mid-glide drawn offset — ADR 137 slice-2 follow-up.
func (l *Layer) armScrollOffset(offset any) {
	var (
		target *int
		dur    time.Duration
		fn     func(float64) float64
	)
	switch o := offset.(type) {
	case *int:
		target = o
	case tweenNode:
		if p, ok := o.getTarget().(*int); ok {
			target = p
			dur = o.getTweenDuration()
			fn = o.getTweenEasing()
		}
	}

	l.scrollMu.Lock()
	defer l.scrollMu.Unlock()

	if resumeAt, unarmed := l.ease.arm(target, dur, fn, l.scrollY); unarmed {
		l.scrollY = resumeAt // resume at the destination, not the frozen legacy 0
	}
}

// scrollToLocked clamps and sets the scroll position; caller holds scrollMu. When an
// offset is bound (ADR 38) it writes the bound target — the single source of truth the
// displayed offset eases toward — instead of the legacy scrollY field.
func (l *Layer) scrollToLocked(y int) {
	if y < 0 {
		y = 0
	}
	if y > l.maxScroll {
		y = l.maxScroll
	}
	if l.ease.target != nil {
		*l.ease.target = y
		return
	}
	l.scrollY = y
}

// currentScrollLocked is the logical scroll position relative scrolls build on; caller
// holds scrollMu. Bound: the target (where we're headed), so HalfPageDown advances from
// the destination, not a stale scrollY. Unbound: scrollY.
func (l *Layer) currentScrollLocked() int {
	if l.ease.target != nil {
		return *l.ease.target
	}
	return l.scrollY
}

// shownOffsetLocked is where the content is drawn RIGHT NOW, read-only; caller holds
// scrollMu. It is the observer half of displayedOffsetLocked, which is the frame driver
// — that one starts and advances eases as a side effect, so anything that isn't blit
// must read through here or it perturbs animation timing from another goroutine.
//
// Unbound: the legacy field. Bound but never drawn: the target, since that is where the
// first frame will put it. Mid-ease: the eased value blit last computed.
func (l *Layer) shownOffsetLocked() int {
	if l.ease.target == nil {
		return l.scrollY
	}
	return l.ease.observe(l.maxScroll)
}

// DisplayedScrollY returns the offset the content is currently DRAWN at. While an eased
// scroll is in flight this lags ScrollY, which returns the destination — so use this one
// to ask "which rows are visible" (jump labels, hit-testing) and ScrollY to ask "am I
// near the bottom". With no ease bound, or once one settles, the two agree.
func (l *Layer) DisplayedScrollY() int {
	l.scrollMu.Lock()
	defer l.scrollMu.Unlock()
	return l.shownOffsetLocked()
}

// displayedOffsetLocked returns the offset blit should draw at; caller holds scrollMu.
// Unbound: the legacy scrollY. Bound: the target clamped to [0,maxScroll] (written back
// so it never sits out of range and snaps after a content grow), eased toward over
// scrollEaseDur. Sets scrollAnimating while an ease is in flight so blit can request
// the next frame. Instant (dur 0) snaps to the target.
func (l *Layer) displayedOffsetLocked() int {
	if l.ease.target == nil {
		return l.scrollY
	}
	return l.ease.advance(l.maxScroll)
}

// ScrollTo sets the scroll position, clamping to valid range.
func (l *Layer) ScrollTo(y int) {
	l.scrollMu.Lock()
	l.scrollToLocked(y)
	l.scrollMu.Unlock()
}

// ScrollDown scrolls down by n lines.
func (l *Layer) ScrollDown(n int) {
	l.scrollMu.Lock()
	l.scrollToLocked(l.currentScrollLocked() + n)
	l.scrollMu.Unlock()
}

// ScrollUp scrolls up by n lines.
func (l *Layer) ScrollUp(n int) {
	l.scrollMu.Lock()
	l.scrollToLocked(l.currentScrollLocked() - n)
	l.scrollMu.Unlock()
}

// ScrollToTop scrolls to the top.
func (l *Layer) ScrollToTop() {
	l.scrollMu.Lock()
	l.scrollToLocked(0)
	l.scrollMu.Unlock()
}

// ScrollToEnd scrolls to the bottom.
func (l *Layer) ScrollToEnd() {
	l.scrollMu.Lock()
	l.scrollToLocked(l.maxScroll)
	l.scrollMu.Unlock()
}

// PageDown scrolls down by one viewport height.
func (l *Layer) PageDown() {
	l.scrollMu.Lock()
	l.scrollToLocked(l.currentScrollLocked() + l.viewHeight)
	l.scrollMu.Unlock()
}

// PageUp scrolls up by one viewport height.
func (l *Layer) PageUp() {
	l.scrollMu.Lock()
	l.scrollToLocked(l.currentScrollLocked() - l.viewHeight)
	l.scrollMu.Unlock()
}

// HalfPageDown scrolls down by half a viewport.
func (l *Layer) HalfPageDown() {
	l.scrollMu.Lock()
	l.scrollToLocked(l.currentScrollLocked() + l.viewHeight/2)
	l.scrollMu.Unlock()
}

// HalfPageUp scrolls up by half a viewport.
func (l *Layer) HalfPageUp() {
	l.scrollMu.Lock()
	l.scrollToLocked(l.currentScrollLocked() - l.viewHeight/2)
	l.scrollMu.Unlock()
}

// blit copies the visible portion of the layer to the destination buffer.
func (l *Layer) blit(dst *Buffer, dstX, dstY, width, height int) {
	if l.buffer == nil {
		return
	}
	l.scrollMu.Lock()
	sy := l.displayedOffsetLocked()
	ms := l.maxScroll
	animating := l.ease.animating
	l.scrollMu.Unlock()
	if l.feather > 0 {
		l.blitFeathered(dst, dstX, dstY, width, height, sy, ms)
	} else {
		// off-path: ordinary layers pay nothing beyond this branch — same plain Blit.
		dst.Blit(l.buffer, 0, sy, dstX, dstY, width, height)
	}
	// While an offset ease is in flight, request the next frame so it advances to the
	// target. RequestRender only signals (no scroll lock), so this is safe after unlock.
	if animating && l.app != nil {
		l.app.RequestRender()
	}
}

// blitFeathered copies the visible region while fading the top/bottom edge rows toward
// the layer background in the SAME pass — the fade is native to the copy, not a read-back
// post-process. Edges fade only where content overflows: the top when scrolled down
// (sy > 0) and the bottom when not yet at the end (sy < maxScroll), so the fade encodes
// scroll state — it appears exactly when there is more to see in that direction. The
// unfaded middle band is one bulk Blit; only the edge rows go cell-by-cell.
func (l *Layer) blitFeathered(dst *Buffer, dstX, dstY, width, height, sy, ms int) {
	target := l.defaultStyle.BG
	topActive := sy > 0
	botActive := sy < ms
	if target.Mode == ColorDefault || (!topActive && !botActive) {
		// nothing to fade toward, or no overflow in either direction — plain copy.
		dst.Blit(l.buffer, 0, sy, dstX, dstY, width, height)
		return
	}
	n := l.feather
	if n > height {
		n = height
	}
	topN := 0
	if topActive {
		topN = n
	}
	botN := 0
	if botActive {
		botN = n
	}
	// middle band never fades — copy it in one bulk Blit.
	if midH := height - topN - botN; midH > 0 {
		dst.Blit(l.buffer, 0, sy+topN, dstX, dstY+topN, width, midH)
	}
	// Only the active edge bands are visited (the middle is the bulk Blit above), and
	// featherRowT is strictly > 0 across both bands — including the short-viewport
	// overlap — so every visited row genuinely fades; there is no unfaded row needing a
	// plain-copy fallback.
	for r := 0; r < height; r++ {
		if r >= topN && r < height-botN {
			continue // covered by the bulk middle Blit
		}
		t := featherRowT(r, n, height, topActive, botActive)
		y := dstY + r
		for x := 0; x < width; x++ {
			c := l.buffer.Get(x, sy+r)
			// Fade toward the background actually BEHIND this cell — the destination
			// cell holds the panel/fill drawn before the layer blits — so content
			// dissolves into whatever it sits on. Fall back to the layer default only
			// when the destination has no explicit background.
			bg := dst.Get(dstX+x, y).Style.BG
			if bg.Mode == ColorDefault {
				bg = target
			}
			c.Style.FG = lerpIfRGB(c.Style.FG, bg, t)
			if c.Style.BG.Mode != ColorDefault {
				c.Style.BG = lerpIfRGB(c.Style.BG, bg, t)
			}
			dst.SetFast(dstX+x, y, c)
		}
	}
}

// featherRowT is the per-row fade strength: strongest at the very edge, vanishing n rows
// in. When both edges reach a row (short viewport), take the stronger.
func featherRowT(r, n, height int, topActive, botActive bool) float64 {
	t := 0.0
	if topActive && r < n {
		if tt := float64(n-r) / float64(n+1); tt > t {
			t = tt
		}
	}
	if botActive && r >= height-n {
		if bb := float64(n-(height-1-r)) / float64(n+1); bb > t {
			t = bb
		}
	}
	return t
}

// SetLine updates a single line in the layer buffer with styled spans.
// This is the efficient path for partial updates (e.g., cursor moved).
// Clears the line first to prevent ghost content from shorter lines.
func (l *Layer) SetLine(y int, spans []Span) {
	if l.buffer == nil || y < 0 || y >= l.buffer.Height() {
		return
	}
	l.buffer.ClearLine(y)
	l.buffer.WriteSpans(0, y, spans, l.buffer.Width())
}

// SetLineString updates a single line with a plain string and style.
// Clears the line first to prevent ghost content from shorter lines.
func (l *Layer) SetLineString(y int, s string, style Style) {
	if l.buffer == nil || y < 0 || y >= l.buffer.Height() {
		return
	}
	l.buffer.ClearLine(y)
	l.buffer.WriteStringFast(0, y, s, style, l.buffer.Width())
}

// SetLineAt updates a line with spans at a given x offset.
// Clears the entire line with clearStyle first, then writes spans at offset x.
// Use this to avoid creating padding spans for margins.
func (l *Layer) SetLineAt(y, x int, spans []Span, clearStyle Style) {
	if l.buffer == nil || y < 0 || y >= l.buffer.Height() {
		return
	}
	l.buffer.ClearLineWithStyle(y, clearStyle)
	l.buffer.WriteSpans(x, y, spans, l.buffer.Width()-x)
}

// EnsureSize ensures the buffer is at least the given size.
// If the buffer needs to grow, existing content is preserved.
func (l *Layer) EnsureSize(width, height int) {
	if l.buffer == nil {
		l.buffer = NewBuffer(width, height)
		return
	}
	if l.buffer.Width() >= width && l.buffer.Height() >= height {
		return
	}
	// Need to grow - create new buffer and copy
	newWidth := max(l.buffer.Width(), width)
	newHeight := max(l.buffer.Height(), height)
	newBuf := NewBuffer(newWidth, newHeight)
	newBuf.Blit(l.buffer, 0, 0, 0, 0, l.buffer.Width(), l.buffer.Height())
	l.buffer = newBuf
	l.scrollMu.Lock()
	l.updateMaxScroll()
	l.scrollMu.Unlock()
}

// Clear clears the entire layer buffer.
func (l *Layer) Clear() {
	if l.buffer != nil {
		l.buffer.Clear()
	}
}

// =============================================================================
// Cursor API
// =============================================================================

// SetCursor sets the cursor position in buffer coordinates.
// The framework translates this to screen coordinates when rendering.
func (l *Layer) SetCursor(x, y int) {
	l.cursor.X = x
	l.cursor.Y = y
}

// SetCursorStyle sets the cursor visual style.
func (l *Layer) SetCursorStyle(style CursorShape) {
	l.cursor.Style = style
}

// ShowCursor makes the cursor visible.
func (l *Layer) ShowCursor() {
	l.cursor.Visible = true
}

// HideCursor hides the cursor.
func (l *Layer) HideCursor() {
	l.cursor.Visible = false
}

// Cursor returns the full cursor state.
func (l *Layer) Cursor() Cursor {
	return l.cursor
}

// ScreenCursor returns the cursor position in screen coordinates.
// This accounts for the layer's position on screen and scroll offset.
// Returns the cursor and whether it's visible and within the viewport.
func (l *Layer) ScreenCursor() (x, y int, visible bool) {
	if !l.cursor.Visible {
		return 0, 0, false
	}

	// snapshot scroll state under the lock: ScreenCursor runs on the render
	// goroutine (render()) while input handlers scroll via ScrollTo etc. Reads the
	// DRAWN offset, not the destination — mid-ease the cursor must sit with the text
	// it belongs to, and on an armed layer the legacy field never moves at all.
	l.scrollMu.Lock()
	scrollY, viewHeight := l.shownOffsetLocked(), l.viewHeight
	l.scrollMu.Unlock()

	// cursor Y relative to viewport (account for scroll)
	viewY := l.cursor.Y - scrollY

	// check if cursor is within visible viewport
	if viewY < 0 || viewY >= viewHeight {
		return 0, 0, false
	}

	// translate to screen coordinates
	x = l.screenX + l.cursor.X
	y = l.screenY + viewY
	return x, y, true
}

// Cursor represents a cursor position and style.
// Use this to read full cursor state. For setting, use the individual
// methods (SetCursor, SetCursorStyle, ShowCursor, HideCursor) which
// are optimized for their typical usage patterns.
type Cursor struct {
	X, Y    int
	Style   CursorShape
	Visible bool
}

// DefaultCursor returns a cursor with sensible defaults.
func DefaultCursor() Cursor {
	return Cursor{
		Style:   CursorBlock,
		Visible: true,
	}
}

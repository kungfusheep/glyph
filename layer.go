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

	// Bound scroll offset (ADR 38). When scrollTarget != nil it is the SINGLE source of
	// truth for the scroll position: every scroll method writes it (clamped), and blit
	// reads the eased displayed value moving toward it. This dissolves the stale-pending
	// vs manual-scroll race by construction (one value, last-write-wins). nil keeps the
	// legacy scrollY path. scrollEaseDur 0 = instant (no animation); >0 eases the
	// displayed offset toward the target over that duration with scrollEaseFn.
	scrollTarget    *int
	scrollEaseDur   time.Duration
	scrollEaseFn    func(float64) float64
	scrollShown     float64   // current displayed offset (eased); valid once scrollShownSet
	scrollShownSet  bool
	scrollAnimFrom  float64   // displayed value when the current ease began
	scrollAnimT0    time.Time // when the current ease began
	scrollAnimTo    int       // target the current ease heads to (detects retargets)
	scrollAnimating bool
	nowFn           func() time.Time // clock hook for tests; nil = time.Now
}

// now returns the layer's clock (real time, or a test hook).
func (l *Layer) now() time.Time {
	if l.nowFn != nil {
		return l.nowFn()
	}
	return time.Now()
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
	l.scrollY = 0
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
	if l.scrollTarget != nil {
		if *l.scrollTarget > l.maxScroll {
			*l.scrollTarget = l.maxScroll
		}
		if *l.scrollTarget < 0 {
			*l.scrollTarget = 0
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
	if l.scrollTarget != nil {
		y := *l.scrollTarget
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
	if l.scrollTarget != nil {
		*l.scrollTarget = y
		return
	}
	l.scrollY = y
}

// currentScrollLocked is the logical scroll position relative scrolls build on; caller
// holds scrollMu. Bound: the target (where we're headed), so HalfPageDown advances from
// the destination, not a stale scrollY. Unbound: scrollY.
func (l *Layer) currentScrollLocked() int {
	if l.scrollTarget != nil {
		return *l.scrollTarget
	}
	return l.scrollY
}

// displayedOffsetLocked returns the offset blit should draw at; caller holds scrollMu.
// Unbound: the legacy scrollY. Bound: the target clamped to [0,maxScroll] (written back
// so it never sits out of range and snaps after a content grow), eased toward over
// scrollEaseDur. Sets scrollAnimating while an ease is in flight so blit can request
// the next frame. Instant (dur 0) snaps to the target.
func (l *Layer) displayedOffsetLocked() int {
	if l.scrollTarget == nil {
		return l.scrollY
	}
	target := *l.scrollTarget
	if target < 0 {
		target = 0
	}
	if target > l.maxScroll {
		target = l.maxScroll
	}
	*l.scrollTarget = target // write back the clamp (grow-snap guard)

	if l.scrollEaseDur <= 0 {
		l.scrollShown = float64(target)
		l.scrollShownSet = true
		l.scrollAnimating = false
		return target
	}
	if !l.scrollShownSet {
		l.scrollShown = float64(target)
		l.scrollShownSet = true
		l.scrollAnimating = false
		return target
	}
	// (re)start an ease when the target moves away from where we're shown/heading.
	if int(math.Round(l.scrollShown)) == target {
		l.scrollAnimating = false
		l.scrollShown = float64(target)
		return target
	}
	if !l.scrollAnimating || l.scrollAnimTo != target {
		l.scrollAnimFrom = l.scrollShown
		l.scrollAnimT0 = l.now()
		l.scrollAnimTo = target
		l.scrollAnimating = true
	}
	p := float64(l.now().Sub(l.scrollAnimT0)) / float64(l.scrollEaseDur)
	if p >= 1 {
		l.scrollShown = float64(target)
		l.scrollAnimating = false
		return target
	}
	if l.scrollEaseFn != nil {
		p = l.scrollEaseFn(p)
	}
	l.scrollShown = l.scrollAnimFrom + p*(float64(target)-l.scrollAnimFrom)
	return int(math.Round(l.scrollShown))
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
	animating := l.scrollAnimating
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
	for r := 0; r < height; r++ {
		if r >= topN && r < height-botN {
			continue // covered by the bulk middle Blit
		}
		t := featherRowT(r, n, height, topActive, botActive)
		if t <= 0 {
			// not actually faded (e.g. a row only the inactive edge would reach) — plain copy.
			dst.Blit(l.buffer, 0, sy+r, dstX, dstY+r, width, 1)
			continue
		}
		y := dstY + r
		for x := 0; x < width; x++ {
			c := l.buffer.Get(x, sy+r)
			c.Style.FG = lerpIfRGB(c.Style.FG, target, t)
			if c.Style.BG.Mode != ColorDefault {
				c.Style.BG = lerpIfRGB(c.Style.BG, target, t)
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
	// goroutine (render()) while input handlers write scrollY via ScrollTo etc.
	l.scrollMu.Lock()
	scrollY, viewHeight := l.scrollY, l.viewHeight
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

package term

import (
	"strconv"
	"sync"
	"unicode/utf8"

	glyph "github.com/kungfusheep/glyph"
)

// screen is a minimal VT interpreter: it consumes the pty's byte stream and
// maintains a cell grid, a cursor, and the current pen. It is content-blind —
// it renders whatever escape sequences arrive, it does not understand programs.
//
// Scope is a shell session, line-oriented tools, and full-screen programs:
// printable text with deferred autowrap, the common C0 controls, CSI
// cursor/erase/scroll/SGR, OSC title, and the alternate screen. Mouse reporting,
// bracketed paste and focus reporting are consumed but not acted on, so a program
// that asks for them falls back to plain encodings.
type screen struct {
	mu sync.Mutex

	rows, cols int
	cells      []glyph.Cell // row-major, len == rows*cols; aliases primary or alt

	// the alternate screen. A full-screen program paints a blank alt grid and the
	// primary grid survives underneath it untouched, so leaving restores whatever
	// the shell had on screen. cells aliases whichever is active, so every write
	// path stays unaware of the swap.
	primary   []glyph.Cell
	alt       []glyph.Cell // nil until a program first asks for it
	altActive bool

	cx, cy int         // cursor, 0-based
	pen    glyph.Style // current graphic rendition

	savedCx, savedCy int
	savedPen         glyph.Style

	// 1049 saves the cursor separately from DECSC (ESC 7), so a program that uses
	// both does not have one clobber the other.
	altSavedCx, altSavedCy int
	altSavedPen            glyph.Style

	scrollTop, scrollBot int // scroll region, 0-based inclusive
	autowrap             bool
	wrapNext             bool // deferred wrap: cursor sits past the last column
	cursorVisible        bool

	// parser state
	st      parseState
	params  []int
	private bool   // CSI '?' private-parameter prefix
	prefix  byte   // CSI parameter-prefix byte, 0x3c-0x3f ('<' '=' '>' '?')
	interm  byte   // last intermediate byte (charset selector etc.)
	oscBuf  []byte // OSC string accumulator
	utf8Buf []byte // partial multi-byte sequence carried across write() calls

	g0Graphics bool // G0 holds the DEC special graphics set (ESC ( 0)

	onTitle func(string)

	// onReply sends bytes back to the pty as terminal input. Programs query the
	// terminal (cursor position, device attributes) and BLOCK waiting for the
	// answer — neovim reports "did not detect DSR response" and degrades when it
	// never arrives.
	onReply func([]byte)

	// staged here and delivered by write() once it drops the lock: a callback that
	// ran under s.mu would invert the lock order (see write).
	pendingTitle string
	titleChanged bool
	pendingReply []byte
}

type parseState uint8

const (
	stGround parseState = iota
	stEscape
	stCSI
	stOSC
	stCharset // consume one selector byte after ESC ( or ESC )
)

func newScreen(rows, cols int) *screen {
	s := &screen{
		rows:          rows,
		cols:          cols,
		primary:       make([]glyph.Cell, rows*cols),
		scrollTop:     0,
		scrollBot:     rows - 1,
		autowrap:      true,
		cursorVisible: true,
		params:        make([]int, 0, 8),
	}
	s.cells = s.primary
	s.clearAll()
	return s
}

// enterAlt makes the alternate grid active, blank. saveCursor distinguishes 1049
// (which saves and restores the cursor across the swap) from the older 47/1047.
func (s *screen) enterAlt(saveCursor bool) {
	if s.altActive {
		return
	}
	if saveCursor {
		s.altSavedCx, s.altSavedCy, s.altSavedPen = s.cx, s.cy, s.pen
	}
	if len(s.alt) != s.rows*s.cols {
		s.alt = make([]glyph.Cell, s.rows*s.cols)
	}
	s.cells = s.alt
	s.altActive = true
	s.clearAll()
	s.cx, s.cy = 0, 0
	s.wrapNext = false
}

// leaveAlt returns to the primary grid, which still holds whatever was on screen
// when the program took over.
func (s *screen) leaveAlt(restoreCursor bool) {
	if !s.altActive {
		return
	}
	s.cells = s.primary
	s.altActive = false
	if restoreCursor {
		s.cx = clamp(s.altSavedCx, 0, s.cols-1)
		s.cy = clamp(s.altSavedCy, 0, s.rows-1)
		s.pen = s.altSavedPen
	}
	s.wrapNext = false
}

// reshapeGrid returns a grid of the new geometry holding the top-left overlap of
// the old one.
func reshapeGrid(old []glyph.Cell, oldRows, oldCols, rows, cols int) []glyph.Cell {
	next := make([]glyph.Cell, rows*cols)
	blank := glyph.Cell{Rune: ' '}
	for i := range next {
		next[i] = blank
	}
	copyRows := min(rows, oldRows)
	copyCols := min(cols, oldCols)
	for y := 0; y < copyRows; y++ {
		for x := 0; x < copyCols; x++ {
			next[y*cols+x] = old[y*oldCols+x]
		}
	}
	return next
}

// cellAt returns a pointer to the cell at col x, row y. Caller holds s.mu.
func (s *screen) cellAt(x, y int) *glyph.Cell {
	return &s.cells[y*s.cols+x]
}

func (s *screen) clearAll() {
	blank := glyph.Cell{Rune: ' '}
	for i := range s.cells {
		s.cells[i] = blank
	}
}

// resize reshapes the grid to rows x cols, preserving the top-left overlap. The
// cursor and scroll region are clamped into the new bounds.
func (s *screen) resize(rows, cols int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rows == s.rows && cols == s.cols {
		return
	}
	// both grids reshape: the primary is the scrollback a program is sitting on
	// top of, so a resize taken while the alt screen is active must not leave it
	// the old size to be restored into.
	s.primary = reshapeGrid(s.primary, s.rows, s.cols, rows, cols)
	if s.alt != nil {
		s.alt = reshapeGrid(s.alt, s.rows, s.cols, rows, cols)
	}
	if s.altActive {
		s.cells = s.alt
	} else {
		s.cells = s.primary
	}
	s.rows, s.cols = rows, cols
	s.scrollTop = 0
	s.scrollBot = rows - 1
	s.cx = clamp(s.cx, 0, cols-1)
	s.cy = clamp(s.cy, 0, rows-1)
	s.wrapNext = false
}

// write feeds pty bytes through the parser, mutating the grid under the lock.
//
// Host callbacks fire AFTER the lock is released. Calling out while holding
// s.mu inverts the lock order: the host's own state mutex would be taken on the
// pty goroutine beneath the screen lock, while the render goroutine takes them
// the other way round in blitToLayer.
func (s *screen) write(p []byte) {
	s.mu.Lock()
	for _, b := range p {
		s.step(b)
	}
	title, changed := s.pendingTitle, s.titleChanged
	s.titleChanged = false
	titleFn := s.onTitle

	var reply []byte
	if len(s.pendingReply) > 0 {
		reply = append(reply, s.pendingReply...)
		s.pendingReply = s.pendingReply[:0]
	}
	replyFn := s.onReply
	s.mu.Unlock()

	if changed && titleFn != nil {
		titleFn(title)
	}
	if len(reply) > 0 && replyFn != nil {
		replyFn(reply)
	}
}

func (s *screen) step(b byte) {
	switch s.st {
	case stGround:
		s.ground(b)
	case stEscape:
		s.escape(b)
	case stCSI:
		s.csi(b)
	case stOSC:
		s.osc(b)
	case stCharset:
		// only G0 is consulted when printing; the other slots are tracked as
		// designated-and-ignored (SI/SO shifting is out of scope).
		if s.interm == '(' {
			s.g0Graphics = b == '0'
		}
		s.st = stGround
	}
}

func (s *screen) ground(b byte) {
	if b >= 0x80 {
		s.decodeUTF8(b)
		return
	}
	if len(s.utf8Buf) > 0 {
		s.utf8Buf = s.utf8Buf[:0] // an ASCII byte abandons a truncated sequence
	}
	switch {
	case b == 0x1b: // ESC
		s.st = stEscape
	case b == '\n', b == '\v', b == '\f':
		s.lineFeed()
	case b == '\r':
		s.cx = 0
		s.wrapNext = false
	case b == '\b':
		if s.cx > 0 {
			s.cx--
		}
		s.wrapNext = false
	case b == '\t':
		s.tab()
	case b == 0x07: // BEL
		// audible bell — no visual effect in scope
	case b < 0x20:
		// other C0: ignore
	default:
		s.put(rune(b))
	}
}

func (s *screen) escape(b byte) {
	switch b {
	case '[':
		s.st = stCSI
		s.params = s.params[:0]
		s.params = append(s.params, 0)
		s.private = false
		s.prefix = 0
		s.interm = 0
	case ']':
		s.st = stOSC
		s.oscBuf = s.oscBuf[:0]
	case '(', ')', '*', '+':
		s.interm = b
		s.st = stCharset
	case '7': // DECSC save cursor
		s.savedCx, s.savedCy, s.savedPen = s.cx, s.cy, s.pen
		s.st = stGround
	case '8': // DECRC restore cursor
		s.cx, s.cy, s.pen = s.savedCx, s.savedCy, s.savedPen
		s.st = stGround
	case 'M': // reverse index
		s.reverseIndex()
		s.st = stGround
	case 'D': // index
		s.lineFeed()
		s.st = stGround
	case 'E': // next line
		s.cx = 0
		s.lineFeed()
		s.st = stGround
	case 'c': // RIS full reset
		s.reset()
		s.st = stGround
	case '=', '>': // keypad modes — ignore
		s.st = stGround
	default:
		s.st = stGround
	}
}

func (s *screen) csi(b byte) {
	switch {
	case b >= '0' && b <= '9':
		n := len(s.params) - 1
		s.params[n] = s.params[n]*10 + int(b-'0')
	case b == ';':
		s.params = append(s.params, 0)
	case b >= 0x3c && b <= 0x3f: // parameter prefix: '<' '=' '>' '?'
		s.prefix = b
		s.private = b == '?'
	case b >= 0x20 && b <= 0x2f: // intermediate
		s.interm = b
	case b >= 0x40 && b <= 0x7e: // final byte
		s.dispatchCSI(b)
		s.st = stGround
	default:
		s.st = stGround
	}
}

func (s *screen) param(i, def int) int {
	if i >= len(s.params) || s.params[i] == 0 {
		return def
	}
	return s.params[i]
}

func (s *screen) dispatchCSI(final byte) {
	if s.prefix != 0 {
		if s.private {
			s.privateMode(final)
		}
		// '<' '=' '>' forms (kitty keyboard protocol, modifyOtherKeys) are
		// consumed and ignored. Without a prefix case they would abort the
		// sequence and the parameters would print into the grid as text.
		return
	}
	switch final {
	case 'H', 'f': // CUP / HVP
		s.cy = clamp(s.param(0, 1)-1, 0, s.rows-1)
		s.cx = clamp(s.param(1, 1)-1, 0, s.cols-1)
		s.wrapNext = false
	case 'A': // cursor up
		s.cy = max(0, s.cy-s.param(0, 1))
		s.wrapNext = false
	case 'B': // cursor down
		s.cy = min(s.rows-1, s.cy+s.param(0, 1))
		s.wrapNext = false
	case 'C': // cursor forward
		s.cx = min(s.cols-1, s.cx+s.param(0, 1))
		s.wrapNext = false
	case 'D': // cursor back
		s.cx = max(0, s.cx-s.param(0, 1))
		s.wrapNext = false
	case 'G', '`': // cursor horizontal absolute
		s.cx = clamp(s.param(0, 1)-1, 0, s.cols-1)
		s.wrapNext = false
	case 'd': // vertical position absolute
		s.cy = clamp(s.param(0, 1)-1, 0, s.rows-1)
		s.wrapNext = false
	case 'J': // erase in display
		s.eraseDisplay(s.param(0, 0))
	case 'K': // erase in line
		s.eraseLine(s.param(0, 0))
	case 'm': // SGR
		s.sgr()
	case 'r': // set scroll region (DECSTBM)
		top := clamp(s.param(0, 1)-1, 0, s.rows-1)
		bot := clamp(s.param(1, s.rows)-1, 0, s.rows-1)
		if top < bot {
			s.scrollTop, s.scrollBot = top, bot
			s.cx, s.cy = 0, top
		}
	case 'L': // insert lines
		s.insertLines(s.param(0, 1))
	case 'M': // delete lines
		s.deleteLines(s.param(0, 1))
	case '@': // insert chars
		s.insertChars(s.param(0, 1))
	case 'P': // delete chars
		s.deleteChars(s.param(0, 1))
	case 'X': // erase chars
		s.eraseChars(s.param(0, 1))
	case 'S': // scroll up
		s.scrollUp(s.scrollTop, s.scrollBot, s.param(0, 1))
	case 'T': // scroll down
		s.scrollDown(s.scrollTop, s.scrollBot, s.param(0, 1))
	case 'n': // DSR — device status report
		switch s.param(0, 0) {
		case 5: // "are you ok?"
			s.reply("\x1b[0n")
		case 6: // report cursor position, 1-based
			s.reply("\x1b[" + strconv.Itoa(s.cy+1) + ";" + strconv.Itoa(s.cx+1) + "R")
		}
	case 'c': // DA — primary device attributes. Answer as a VT100 with AVO, which
		// is what xterm reports and what TERM=xterm promises.
		if s.param(0, 0) == 0 {
			s.reply("\x1b[?1;2c")
		}
	case 's': // save cursor (ANSI.SYS)
		s.savedCx, s.savedCy = s.cx, s.cy
	case 'u': // restore cursor (ANSI.SYS)
		s.cx, s.cy = s.savedCx, s.savedCy
	}
}

func (s *screen) privateMode(final byte) {
	set := final == 'h'
	if final != 'h' && final != 'l' {
		return
	}
	switch s.param(0, 0) {
	case 25: // DECTCEM cursor visible
		s.cursorVisible = set
	case 7: // DECAWM autowrap
		s.autowrap = set
	case 1049: // alt screen, saving and restoring the cursor with the swap
		if set {
			s.enterAlt(true)
		} else {
			s.leaveAlt(true)
		}
	case 47, 1047: // the older alt-screen forms: swap only, cursor untouched
		if set {
			s.enterAlt(false)
		} else {
			s.leaveAlt(false)
		}
	}
}

func (s *screen) osc(b byte) {
	switch {
	case b == 0x07: // BEL terminates
		s.finishOSC()
		s.st = stGround
	case b == 0x1b: // possible ST (ESC \)
		s.st = stEscape // the following '\' falls through escape default → ground
		s.finishOSC()
	default:
		s.oscBuf = append(s.oscBuf, b)
	}
}

func (s *screen) finishOSC() {
	// forms: "0;title" (icon+title) or "2;title" (title)
	buf := s.oscBuf
	i := 0
	for i < len(buf) && buf[i] != ';' {
		i++
	}
	if i >= len(buf) {
		return
	}
	code := string(buf[:i])
	title := string(buf[i+1:])
	if code == "0" || code == "2" {
		s.pendingTitle, s.titleChanged = title, true
	}
}

// decodeUTF8 accumulates a multi-byte sequence and prints it once complete. The
// buffer persists across write() calls, so a rune split over two pty reads still
// lands as one glyph rather than a run of latin-1 garbage.
func (s *screen) decodeUTF8(b byte) {
	s.utf8Buf = append(s.utf8Buf, b)
	if utf8.FullRune(s.utf8Buf) {
		r, _ := utf8.DecodeRune(s.utf8Buf)
		s.utf8Buf = s.utf8Buf[:0]
		s.put(r)
		return
	}
	if len(s.utf8Buf) >= utf8.UTFMax {
		s.utf8Buf = s.utf8Buf[:0]
		s.put(utf8.RuneError)
	}
}

// decGraphics maps 0x5f..0x7e to the DEC special graphics set, which ncurses and
// friends select with ESC ( 0 to draw box lines. Without it, `qqq` prints as the
// letter q instead of a horizontal rule.
var decGraphics = [...]rune{
	' ', '◆', '▒', '␉', '␌', '␍', '␊', '°',
	'±', '␤', '␋', '┘', '┐', '┌', '└', '┼',
	'⎺', '⎻', '─', '⎼', '⎽', '├', '┤', '┴',
	'┬', '│', '≤', '≥', 'π', '≠', '£', '·',
}

// put writes a printable rune at the cursor, applying the active charset,
// deferred autowrap, and double-width layout.
func (s *screen) put(r rune) {
	if s.g0Graphics && r >= 0x5f && r <= 0x7e {
		r = decGraphics[r-0x5f]
	}
	// ASCII is width 1 without consulting the table — this is the hot path.
	w := 1
	if r >= 0x1100 {
		w = glyph.RuneWidth(r)
	}
	if s.wrapNext && s.autowrap {
		s.cx = 0
		s.lineFeed()
		s.wrapNext = false
	}
	// a double-width rune never straddles the right margin: wrap it whole.
	if w == 2 && s.cx == s.cols-1 {
		if !s.autowrap {
			return
		}
		s.cx = 0
		s.lineFeed()
	}

	c := s.cellAt(s.cx, s.cy)
	if isWideHalf(c.Rune) {
		s.breakWideAt(s.cx)
	}
	c.Rune = r
	c.Style = s.pen
	if w == 2 {
		if p := s.cellAt(s.cx+1, s.cy); isWideHalf(p.Rune) {
			s.breakWideAt(s.cx + 1)
		}
		p := s.cellAt(s.cx+1, s.cy)
		p.Rune = 0 // placeholder: the renderer skips the second half
		p.Style = s.pen
	}

	if nx := s.cx + w; nx >= s.cols {
		s.cx = s.cols - 1
		s.wrapNext = true // sit at the last column until the next glyph
	} else {
		s.cx = nx
	}
}

// isWideHalf reports whether a cell could be either half of a double-width pair:
// the placeholder (rune 0) or a rune wide enough to have one. It gates the
// bisect check so the ASCII path never pays for the table lookup.
func isWideHalf(r rune) bool {
	return r == 0 || (r >= 0x1100 && glyph.RuneWidth(r) == 2)
}

// breakWideAt blanks the remains of a double-width pair that a write at x is
// about to bisect, so no orphaned half survives to render as a hole.
func (s *screen) breakWideAt(x int) {
	if x < 0 || x >= s.cols {
		return
	}
	c := s.cellAt(x, s.cy)
	switch {
	case c.Rune == 0 && x > 0:
		if left := s.cellAt(x-1, s.cy); glyph.RuneWidth(left.Rune) == 2 {
			left.Rune = ' '
		}
		c.Rune = ' '
	case glyph.RuneWidth(c.Rune) == 2 && x+1 < s.cols:
		if right := s.cellAt(x+1, s.cy); right.Rune == 0 {
			right.Rune = ' '
		}
	}
}

// reply stages bytes for the pty. write() flushes them after releasing the lock.
func (s *screen) reply(seq string) {
	s.pendingReply = append(s.pendingReply, seq...)
}

func (s *screen) tab() {
	s.wrapNext = false
	next := ((s.cx / 8) + 1) * 8
	s.cx = min(next, s.cols-1)
}

// lineFeed moves down one row, scrolling the region when at the bottom margin.
func (s *screen) lineFeed() {
	if s.cy == s.scrollBot {
		s.scrollUp(s.scrollTop, s.scrollBot, 1)
	} else if s.cy < s.rows-1 {
		s.cy++
	}
	s.wrapNext = false
}

func (s *screen) reverseIndex() {
	if s.cy == s.scrollTop {
		s.scrollDown(s.scrollTop, s.scrollBot, 1)
	} else if s.cy > 0 {
		s.cy--
	}
}

// scrollUp shifts rows [top,bot] up by n, blanking the freed rows at the bottom.
func (s *screen) scrollUp(top, bot, n int) {
	if n <= 0 {
		return
	}
	n = min(n, bot-top+1)
	for y := top; y <= bot; y++ {
		src := y + n
		if src <= bot {
			copy(s.rowSlice(y), s.rowSlice(src))
		} else {
			s.blankRow(y)
		}
	}
}

// scrollDown shifts rows [top,bot] down by n, blanking the freed rows at the top.
func (s *screen) scrollDown(top, bot, n int) {
	if n <= 0 {
		return
	}
	n = min(n, bot-top+1)
	for y := bot; y >= top; y-- {
		src := y - n
		if src >= top {
			copy(s.rowSlice(y), s.rowSlice(src))
		} else {
			s.blankRow(y)
		}
	}
}

func (s *screen) rowSlice(y int) []glyph.Cell {
	return s.cells[y*s.cols : y*s.cols+s.cols]
}

func (s *screen) blankRow(y int) {
	blank := glyph.Cell{Rune: ' ', Style: s.pen}
	row := s.rowSlice(y)
	for i := range row {
		row[i] = blank
	}
}

func (s *screen) insertLines(n int) {
	if s.cy < s.scrollTop || s.cy > s.scrollBot {
		return
	}
	s.scrollDown(s.cy, s.scrollBot, n)
}

func (s *screen) deleteLines(n int) {
	if s.cy < s.scrollTop || s.cy > s.scrollBot {
		return
	}
	s.scrollUp(s.cy, s.scrollBot, n)
}

func (s *screen) insertChars(n int) {
	row := s.rowSlice(s.cy)
	n = min(n, s.cols-s.cx)
	copy(row[s.cx+n:], row[s.cx:s.cols-n])
	for x := s.cx; x < s.cx+n; x++ {
		row[x] = glyph.Cell{Rune: ' ', Style: s.pen}
	}
}

func (s *screen) deleteChars(n int) {
	row := s.rowSlice(s.cy)
	n = min(n, s.cols-s.cx)
	copy(row[s.cx:], row[s.cx+n:])
	for x := s.cols - n; x < s.cols; x++ {
		row[x] = glyph.Cell{Rune: ' ', Style: s.pen}
	}
}

func (s *screen) eraseChars(n int) {
	row := s.rowSlice(s.cy)
	end := min(s.cx+n, s.cols)
	for x := s.cx; x < end; x++ {
		row[x] = glyph.Cell{Rune: ' ', Style: s.pen}
	}
}

func (s *screen) eraseDisplay(mode int) {
	switch mode {
	case 0: // cursor to end
		s.eraseLine(0)
		for y := s.cy + 1; y < s.rows; y++ {
			s.blankRow(y)
		}
	case 1: // start to cursor
		for y := 0; y < s.cy; y++ {
			s.blankRow(y)
		}
		s.eraseLine(1)
	case 2, 3: // whole display
		for y := 0; y < s.rows; y++ {
			s.blankRow(y)
		}
	}
}

func (s *screen) eraseLine(mode int) {
	row := s.rowSlice(s.cy)
	blank := glyph.Cell{Rune: ' ', Style: s.pen}
	switch mode {
	case 0: // cursor to end
		for x := s.cx; x < s.cols; x++ {
			row[x] = blank
		}
	case 1: // start to cursor
		for x := 0; x <= s.cx && x < s.cols; x++ {
			row[x] = blank
		}
	case 2: // whole line
		for x := 0; x < s.cols; x++ {
			row[x] = blank
		}
	}
}

func (s *screen) reset() {
	s.pen = glyph.Style{}
	s.cx, s.cy = 0, 0
	s.scrollTop, s.scrollBot = 0, s.rows-1
	s.autowrap = true
	s.cursorVisible = true
	s.wrapNext = false
	s.g0Graphics = false
	s.utf8Buf = s.utf8Buf[:0]
	s.clearAll()
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

package term

import (
	"sync"

	glyph "github.com/kungfusheep/glyph"
)

// screen is a minimal VT interpreter: it consumes the pty's byte stream and
// maintains a cell grid, a cursor, and the current pen. It is content-blind —
// it renders whatever escape sequences arrive, it does not understand programs.
//
// Scope is a shell session and line-oriented tools: printable text with
// deferred autowrap, the common C0 controls, CSI cursor/erase/scroll/SGR, and
// OSC title. Alt-screen and truecolor-mouse programs render on a later slice;
// this is enough to run a shell and see its output correctly.
type screen struct {
	mu sync.Mutex

	rows, cols int
	cells      []glyph.Cell // row-major, len == rows*cols

	cx, cy int         // cursor, 0-based
	pen    glyph.Style // current graphic rendition

	savedCx, savedCy int
	savedPen         glyph.Style

	scrollTop, scrollBot int // scroll region, 0-based inclusive
	autowrap             bool
	wrapNext             bool // deferred wrap: cursor sits past the last column
	cursorVisible        bool

	// parser state
	st      parseState
	params  []int
	private bool   // CSI '?' private-parameter prefix
	interm  byte   // last intermediate byte (charset selector etc.)
	oscBuf  []byte // OSC string accumulator

	onTitle func(string)
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
		cells:         make([]glyph.Cell, rows*cols),
		scrollTop:     0,
		scrollBot:     rows - 1,
		autowrap:      true,
		cursorVisible: true,
		params:        make([]int, 0, 8),
	}
	s.clearAll()
	return s
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
	next := make([]glyph.Cell, rows*cols)
	blank := glyph.Cell{Rune: ' '}
	for i := range next {
		next[i] = blank
	}
	copyRows := min(rows, s.rows)
	copyCols := min(cols, s.cols)
	for y := 0; y < copyRows; y++ {
		for x := 0; x < copyCols; x++ {
			next[y*cols+x] = s.cells[y*s.cols+x]
		}
	}
	s.cells = next
	s.rows, s.cols = rows, cols
	s.scrollTop = 0
	s.scrollBot = rows - 1
	s.cx = clamp(s.cx, 0, cols-1)
	s.cy = clamp(s.cy, 0, rows-1)
	s.wrapNext = false
}

// write feeds pty bytes through the parser, mutating the grid under the lock.
func (s *screen) write(p []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range p {
		s.step(b)
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
		s.st = stGround // swallow the selector, we only run one charset
	}
}

func (s *screen) ground(b byte) {
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
		s.put(b)
	}
}

func (s *screen) escape(b byte) {
	switch b {
	case '[':
		s.st = stCSI
		s.params = s.params[:0]
		s.params = append(s.params, 0)
		s.private = false
		s.interm = 0
	case ']':
		s.st = stOSC
		s.oscBuf = s.oscBuf[:0]
	case '(', ')', '*', '+':
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
	case b == '?':
		s.private = true
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
	if s.private {
		s.privateMode(final)
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
	}
	// 1049/47/1047 alt-screen intentionally unhandled in this slice
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
	if (code == "0" || code == "2") && s.onTitle != nil {
		s.onTitle(title)
	}
}

// put writes a printable byte at the cursor, applying deferred autowrap.
func (s *screen) put(b byte) {
	if s.wrapNext && s.autowrap {
		s.cx = 0
		s.lineFeed()
		s.wrapNext = false
	}
	c := s.cellAt(s.cx, s.cy)
	c.Rune = rune(b)
	c.Style = s.pen
	if s.cx == s.cols-1 {
		s.wrapNext = true // sit at the last column until the next glyph
	} else {
		s.cx++
	}
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

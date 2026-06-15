package glyph

// JumpStyle configures the appearance of jump labels.
type JumpStyle struct {
	LabelStyle Style // Style for the not-yet-typed remainder of a label

	// MatchedStyle paints the already-typed prefix of a multi-char label
	// (and, when a label no longer matches the typed input, the whole label),
	// so progress is visible mid-sequence. A zero MatchedStyle derives a dim
	// from LabelStyle — feedback is on by default (ADR 11). At rest (nothing
	// typed) no prefix exists, so the render is identical to LabelStyle alone.
	MatchedStyle Style
}

// dimDerived returns a dimmed variant of a style for the default-on matched
// feedback: the typed prefix (and dead labels) recede so the next key to press
// stays prominent. The recede is CATEGORICAL — it drops the background band
// (the "chip"), greys the FG, and drops bold — not a mere attribute or FG nudge.
// Two earlier tries failed on real consumers: a bold->faint flip is invisible
// (SGR 2 is weak/terminal-dependent), and nudging only the FG is too subtle on a
// label that keeps a coloured chip (recap's diff labels are near-black on a
// muted-blue band, so FG near-black->grey on the SAME band barely reads).
// Removing the band is unmistakable on any theme: an active label is a coloured
// chip, a receded one is plain grey text.
func dimDerived(s Style) Style {
	s.BG = Color{} // drop the chip — the strongest theme-agnostic recede
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
type JumpMode struct {
	Active  bool
	Targets []JumpTarget
	Input   string // Accumulated input for multi-char labels
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

// ClearJumpTargets resets the jump targets slice for reuse.
func (jm *JumpMode) ClearJumpTargets() {
	jm.Targets = jm.Targets[:0]
	jm.Input = ""
}

func (jm *JumpMode) ClearTargets() {
	jm.Targets = jm.Targets[:0]
}

// AddTarget adds a jump target during render.
func (jm *JumpMode) AddTarget(x, y int16, onSelect func(), style Style) {
	jm.Targets = append(jm.Targets, JumpTarget{
		X:        x,
		Y:        y,
		OnSelect: onSelect,
		Style:    style,
	})
}

// AssignLabels assigns labels to all collected targets.
func (jm *JumpMode) AssignLabels() {
	labels := GenerateLabels(len(jm.Targets))
	for i := range jm.Targets {
		jm.Targets[i].Label = labels[i]
	}
}

// FindTarget finds a target by its label.
func (jm *JumpMode) FindTarget(label string) *JumpTarget {
	for i := range jm.Targets {
		if jm.Targets[i].Label == label {
			return &jm.Targets[i]
		}
	}
	return nil
}

// HasPartialMatch checks if any target label starts with the given prefix.
func (jm *JumpMode) HasPartialMatch(prefix string) bool {
	for _, t := range jm.Targets {
		if len(t.Label) > len(prefix) && t.Label[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

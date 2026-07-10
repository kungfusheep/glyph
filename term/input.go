package term

import (
	"github.com/kungfusheep/riffkey"
)

// HandleKey encodes a key event into the pty's input stream and reports it
// consumed. Its signature matches riffkey.Router.HandleUnmatched, so the host
// arms the focused pane with:
//
//	router.HandleUnmatched(term.HandleKey)
//
// It always returns true — a focused terminal swallows every key. Route the
// tmux-style prefix on a router ABOVE this so the host keeps its own bindings.
func (t *TermC) HandleKey(k riffkey.Key) bool {
	if b := encodeKey(k); len(b) > 0 {
		t.Write(b)
	}
	return true
}

// encodeKey turns a key event into the bytes a terminal application expects.
func encodeKey(k riffkey.Key) []byte {
	if k.Paste != "" {
		return []byte(k.Paste)
	}

	switch k.Special {
	case riffkey.SpecialEnter:
		return []byte{'\r'}
	case riffkey.SpecialTab:
		return []byte{'\t'}
	case riffkey.SpecialBackspace:
		return []byte{0x7f}
	case riffkey.SpecialEscape:
		return []byte{0x1b}
	case riffkey.SpecialSpace:
		return []byte{' '}
	case riffkey.SpecialUp:
		return []byte("\x1b[A")
	case riffkey.SpecialDown:
		return []byte("\x1b[B")
	case riffkey.SpecialRight:
		return []byte("\x1b[C")
	case riffkey.SpecialLeft:
		return []byte("\x1b[D")
	case riffkey.SpecialHome:
		return []byte("\x1b[H")
	case riffkey.SpecialEnd:
		return []byte("\x1b[F")
	case riffkey.SpecialPageUp:
		return []byte("\x1b[5~")
	case riffkey.SpecialPageDown:
		return []byte("\x1b[6~")
	case riffkey.SpecialInsert:
		return []byte("\x1b[2~")
	case riffkey.SpecialDelete:
		return []byte("\x1b[3~")
	}

	if k.Rune == 0 {
		return nil
	}

	// Ctrl-<letter> maps to the C0 control code (Ctrl-A = 0x01 ... Ctrl-Z =
	// 0x1a); Ctrl-Space and a few symbols fold into the same low range.
	if k.Mod&riffkey.ModCtrl != 0 {
		r := k.Rune
		switch {
		case r >= 'a' && r <= 'z':
			return []byte{byte(r-'a') + 1}
		case r >= 'A' && r <= 'Z':
			return []byte{byte(r-'A') + 1}
		case r == ' ' || r == '@':
			return []byte{0}
		case r >= '[' && r <= '_':
			return []byte{byte(r-'[') + 0x1b}
		}
	}

	// Alt-<key> is ESC then the key's bytes.
	if k.Mod&riffkey.ModAlt != 0 {
		return append([]byte{0x1b}, []byte(string(k.Rune))...)
	}

	return []byte(string(k.Rune))
}

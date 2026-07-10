package term

import (
	"bytes"
	"testing"

	"github.com/kungfusheep/riffkey"
)

func TestEncodeKey(t *testing.T) {
	cases := []struct {
		name string
		key  riffkey.Key
		want []byte
	}{
		{"enter", riffkey.Key{Special: riffkey.SpecialEnter}, []byte{'\r'}},
		{"tab", riffkey.Key{Special: riffkey.SpecialTab}, []byte{'\t'}},
		{"backspace", riffkey.Key{Special: riffkey.SpecialBackspace}, []byte{0x7f}},
		{"escape", riffkey.Key{Special: riffkey.SpecialEscape}, []byte{0x1b}},
		{"up", riffkey.Key{Special: riffkey.SpecialUp}, []byte("\x1b[A")},
		{"left", riffkey.Key{Special: riffkey.SpecialLeft}, []byte("\x1b[D")},
		{"delete", riffkey.Key{Special: riffkey.SpecialDelete}, []byte("\x1b[3~")},
		{"plain rune", riffkey.Key{Rune: 'a'}, []byte("a")},
		{"ctrl-c", riffkey.Key{Rune: 'c', Mod: riffkey.ModCtrl}, []byte{0x03}},
		{"ctrl-a", riffkey.Key{Rune: 'a', Mod: riffkey.ModCtrl}, []byte{0x01}},
		{"alt-x", riffkey.Key{Rune: 'x', Mod: riffkey.ModAlt}, []byte{0x1b, 'x'}},
		{"paste", riffkey.Key{Paste: "hello"}, []byte("hello")},
		{"unicode", riffkey.Key{Rune: '€'}, []byte("€")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := encodeKey(c.key); !bytes.Equal(got, c.want) {
				t.Fatalf("encodeKey(%+v) = %v, want %v", c.key, got, c.want)
			}
		})
	}
}

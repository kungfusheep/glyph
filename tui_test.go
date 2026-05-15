package glyph

import "testing"

func TestAttribute(t *testing.T) {
	t.Run("Has", func(t *testing.T) {
		attr := AttrBold | AttrItalic
		if !attr.Has(AttrBold) {
			t.Error("expected attr to have Bold")
		}
		if !attr.Has(AttrItalic) {
			t.Error("expected attr to have Italic")
		}
		if attr.Has(AttrUnderline) {
			t.Error("expected attr to not have Underline")
		}
	})

	t.Run("With", func(t *testing.T) {
		attr := AttrBold
		attr = attr.With(AttrItalic)
		if !attr.Has(AttrBold) || !attr.Has(AttrItalic) {
			t.Error("expected attr to have both Bold and Italic")
		}
	})

	t.Run("Without", func(t *testing.T) {
		attr := AttrBold | AttrItalic
		attr = attr.Without(AttrBold)
		if attr.Has(AttrBold) {
			t.Error("expected attr to not have Bold")
		}
		if !attr.Has(AttrItalic) {
			t.Error("expected attr to still have Italic")
		}
	})
}

func TestColor(t *testing.T) {
	t.Run("DefaultColor", func(t *testing.T) {
		c := DefaultColor()
		if c.Mode != ColorDefault {
			t.Errorf("expected ColorDefault, got %v", c.Mode)
		}
	})

	t.Run("Ansi16", func(t *testing.T) {
		c := Ansi16(5)
		if c.Mode != Color16 || c.Index != 5 {
			t.Errorf("expected Color16 with index 5, got %v/%d", c.Mode, c.Index)
		}
	})

	t.Run("Ansi256", func(t *testing.T) {
		c := Ansi256(200)
		if c.Mode != Color256 || c.Index != 200 {
			t.Errorf("expected Color256 with index 200, got %v/%d", c.Mode, c.Index)
		}
	})

	t.Run("RGB", func(t *testing.T) {
		c := RGB(255, 128, 64)
		if c.Mode != ColorRGB || c.R != 255 || c.G != 128 || c.B != 64 {
			t.Errorf("expected RGB(255,128,64), got %+v", c)
		}
	})

	t.Run("Hex", func(t *testing.T) {
		c := Hex(0xFF8040)
		if c.Mode != ColorRGB || c.R != 255 || c.G != 128 || c.B != 64 {
			t.Errorf("expected RGB(255,128,64), got %+v", c)
		}
	})

	t.Run("Equal", func(t *testing.T) {
		c1 := RGB(100, 100, 100)
		c2 := RGB(100, 100, 100)
		c3 := RGB(100, 100, 101)

		if !c1.Equal(c2) {
			t.Error("expected c1 and c2 to be equal")
		}
		if c1.Equal(c3) {
			t.Error("expected c1 and c3 to not be equal")
		}
	})

	t.Run("Lerp", func(t *testing.T) {
		black := RGB(0, 0, 0)
		white := RGB(255, 255, 255)

		// t=0 should return first color
		c := Lerp(black, white, 0)
		if c.R != 0 || c.G != 0 || c.B != 0 {
			t.Errorf("t=0: expected black, got %+v", c)
		}

		// t=1 should return second color
		c = Lerp(black, white, 1)
		if c.R != 255 || c.G != 255 || c.B != 255 {
			t.Errorf("t=1: expected white, got %+v", c)
		}

		// t=0.5 should return midpoint
		c = Lerp(black, white, 0.5)
		if c.R != 128 || c.G != 128 || c.B != 128 {
			t.Errorf("t=0.5: expected gray(128), got %+v", c)
		}

		// test clamping
		c = Lerp(black, white, -1)
		if c.R != 0 {
			t.Errorf("t=-1: should clamp to 0, got %+v", c)
		}
		c = Lerp(black, white, 2)
		if c.R != 255 {
			t.Errorf("t=2: should clamp to 1, got %+v", c)
		}
	})

	t.Run("ContrastRatio", func(t *testing.T) {
		if got := ContrastRatio(RGB(0, 0, 0), RGB(255, 255, 255)); got < 20.9 || got > 21.1 {
			t.Errorf("expected black/white contrast around 21, got %.2f", got)
		}
		if got := ContrastRatio(RGB(64, 64, 64), RGB(64, 64, 64)); got != 1 {
			t.Errorf("expected same-colour contrast of 1, got %.2f", got)
		}
	})

	t.Run("LerpToContrast", func(t *testing.T) {
		bg := White
		start := RGB(180, 180, 180)
		result := LerpToContrast(start, Black, bg, 4.5)
		if ContrastRatio(result, bg) < 4.5 {
			t.Errorf("expected AA contrast, got %.2f from %+v", ContrastRatio(result, bg), result)
		}
		if result.Equal(start) {
			t.Error("expected colour to move towards target")
		}
	})

	t.Run("LerpToContrast best effort", func(t *testing.T) {
		bg := RGB(255, 255, 255)
		start := RGB(230, 230, 230)
		target := RGB(180, 180, 180)
		result := LerpToContrast(start, target, bg, 4.5)
		if !result.Equal(target) {
			t.Errorf("expected closest reachable target, got %+v", result)
		}
	})

	t.Run("ReadableTint", func(t *testing.T) {
		bg := Hex(0x1c1c1c)
		fg := Hex(0xe8e6e3)
		result := ReadableTint(bg, Hex(0xd65f5f), fg, 4.5, 0.4)
		if result.Equal(bg) {
			t.Error("expected readable tint to move away from background")
		}
		if ContrastRatio(fg, result) < 4.5 {
			t.Errorf("expected readable tint contrast >= 4.5, got %.2f", ContrastRatio(fg, result))
		}
	})

	t.Run("ReadableTint caps at maximum", func(t *testing.T) {
		bg := Black
		fg := White
		tint := RGB(20, 0, 0)
		result := ReadableTint(bg, tint, fg, 4.5, 0.5)
		want := Lerp(bg, tint, 0.5)
		if !result.Equal(want) {
			t.Errorf("expected tint capped at max amount %+v, got %+v", want, result)
		}
	})
}

func TestStyle(t *testing.T) {
	t.Run("DefaultStyle", func(t *testing.T) {
		s := DefaultStyle()
		if s.FG.Mode != ColorDefault || s.BG.Mode != ColorDefault {
			t.Error("expected default colors")
		}
		if s.Attr != AttrNone {
			t.Error("expected no attributes")
		}
	})

	t.Run("Chaining", func(t *testing.T) {
		s := DefaultStyle().
			Foreground(Red).
			Background(Blue).
			Bold().
			Italic()

		if !s.FG.Equal(Red) {
			t.Error("expected Red foreground")
		}
		if !s.BG.Equal(Blue) {
			t.Error("expected Blue background")
		}
		if !s.Attr.Has(AttrBold) || !s.Attr.Has(AttrItalic) {
			t.Error("expected Bold and Italic attributes")
		}
	})

	t.Run("Equal", func(t *testing.T) {
		s1 := DefaultStyle().Foreground(Red).Bold()
		s2 := DefaultStyle().Foreground(Red).Bold()
		s3 := DefaultStyle().Foreground(Red)

		if !s1.Equal(s2) {
			t.Error("expected s1 and s2 to be equal")
		}
		if s1.Equal(s3) {
			t.Error("expected s1 and s3 to not be equal")
		}
	})
}

func TestCell(t *testing.T) {
	t.Run("EmptyCell", func(t *testing.T) {
		c := EmptyCell()
		if c.Rune != ' ' {
			t.Errorf("expected space, got %q", c.Rune)
		}
	})

	t.Run("NewCell", func(t *testing.T) {
		style := DefaultStyle().Foreground(Red)
		c := NewCell('X', style)
		if c.Rune != 'X' || !c.Style.Equal(style) {
			t.Error("cell not created correctly")
		}
	})

	t.Run("Equal", func(t *testing.T) {
		c1 := NewCell('A', DefaultStyle().Foreground(Red))
		c2 := NewCell('A', DefaultStyle().Foreground(Red))
		c3 := NewCell('B', DefaultStyle().Foreground(Red))

		if !c1.Equal(c2) {
			t.Error("expected c1 and c2 to be equal")
		}
		if c1.Equal(c3) {
			t.Error("expected c1 and c3 to not be equal")
		}
	})
}

func TestSetViewLimit(t *testing.T) {
	// helper to simulate SetView counter logic without needing a real screen
	checkLimit := func(app *App) {
		app.setViewCount++
		if app.setViewLimit > 0 && app.setViewCount > app.setViewLimit {
			panic("SetView limit exceeded")
		}
	}

	t.Run("unlimited by default", func(t *testing.T) {
		app := &App{}
		// Should not panic - unlimited by default
		checkLimit(app)
		checkLimit(app)
		checkLimit(app)
		if app.setViewCount != 3 {
			t.Errorf("expected setViewCount=3, got %d", app.setViewCount)
		}
	})

	t.Run("limit of 1 allows single call", func(t *testing.T) {
		app := &App{}
		app.SetViewLimit(1)
		checkLimit(app)
		if app.setViewCount != 1 {
			t.Errorf("expected setViewCount=1, got %d", app.setViewCount)
		}
	})

	t.Run("limit of 1 panics on second call", func(t *testing.T) {
		app := &App{}
		app.SetViewLimit(1)
		checkLimit(app)

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic on second SetView call")
			}
		}()
		checkLimit(app) // Should panic
	})

	t.Run("limit of 2 allows two calls", func(t *testing.T) {
		app := &App{}
		app.SetViewLimit(2)
		checkLimit(app)
		checkLimit(app)
		if app.setViewCount != 2 {
			t.Errorf("expected setViewCount=2, got %d", app.setViewCount)
		}
	})
}

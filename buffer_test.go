package glyph

import "testing"

func TestBuffer(t *testing.T) {
	t.Run("NewBuffer", func(t *testing.T) {
		buf := NewBuffer(80, 24)
		if buf.Width() != 80 || buf.Height() != 24 {
			t.Errorf("expected 80x24, got %dx%d", buf.Width(), buf.Height())
		}

		// All cells should be empty
		for y := 0; y < buf.Height(); y++ {
			for x := 0; x < buf.Width(); x++ {
				c := buf.Get(x, y)
				if c.Rune != ' ' {
					t.Errorf("expected space at (%d,%d), got %q", x, y, c.Rune)
				}
			}
		}
	})

	t.Run("InBounds", func(t *testing.T) {
		buf := NewBuffer(10, 10)

		tests := []struct {
			x, y   int
			expect bool
		}{
			{0, 0, true},
			{9, 9, true},
			{-1, 0, false},
			{0, -1, false},
			{10, 0, false},
			{0, 10, false},
		}

		for _, tt := range tests {
			got := buf.InBounds(tt.x, tt.y)
			if got != tt.expect {
				t.Errorf("InBounds(%d,%d) = %v, want %v", tt.x, tt.y, got, tt.expect)
			}
		}
	})

	t.Run("SetGet", func(t *testing.T) {
		buf := NewBuffer(10, 10)
		cell := NewCell('X', DefaultStyle().Foreground(Red))

		buf.Set(5, 5, cell)
		got := buf.Get(5, 5)

		if !got.Equal(cell) {
			t.Errorf("got %+v, want %+v", got, cell)
		}

		// Out of bounds should return empty cell
		oob := buf.Get(-1, -1)
		if oob.Rune != ' ' {
			t.Error("expected empty cell for out of bounds")
		}
	})

	t.Run("SetRune", func(t *testing.T) {
		buf := NewBuffer(10, 10)
		buf.Set(5, 5, NewCell('A', DefaultStyle().Foreground(Red)))
		buf.SetRune(5, 5, 'B')

		got := buf.Get(5, 5)
		if got.Rune != 'B' {
			t.Errorf("expected 'B', got %q", got.Rune)
		}
		// Style should be preserved
		if !got.Style.FG.Equal(Red) {
			t.Error("expected style to be preserved")
		}
	})

	t.Run("WriteString", func(t *testing.T) {
		buf := NewBuffer(20, 5)
		style := DefaultStyle().Foreground(Green)

		written := buf.WriteString(2, 2, "Hello", style)
		if written != 5 {
			t.Errorf("expected 5 written, got %d", written)
		}

		expected := "Hello"
		for i, ch := range expected {
			c := buf.Get(2+i, 2)
			if c.Rune != ch {
				t.Errorf("at %d: expected %q, got %q", i, ch, c.Rune)
			}
		}
	})

	t.Run("WriteStringClipped", func(t *testing.T) {
		buf := NewBuffer(20, 5)
		style := DefaultStyle()

		written := buf.WriteStringClipped(0, 0, "Hello World", style, 5)
		if written != 5 {
			t.Errorf("expected 5 written, got %d", written)
		}

		// Should only have "Hello"
		if buf.Get(4, 0).Rune != 'o' {
			t.Error("expected 'o' at position 4")
		}
		if buf.Get(5, 0).Rune != ' ' {
			t.Error("expected space at position 5")
		}
	})

	t.Run("FillRect", func(t *testing.T) {
		buf := NewBuffer(20, 10)
		cell := NewCell('#', DefaultStyle().Background(Blue))

		buf.FillRect(5, 5, 3, 2, cell)

		// Check filled area
		for y := 5; y < 7; y++ {
			for x := 5; x < 8; x++ {
				if buf.Get(x, y).Rune != '#' {
					t.Errorf("expected '#' at (%d,%d)", x, y)
				}
			}
		}

		// Check outside area
		if buf.Get(4, 5).Rune != ' ' {
			t.Error("expected space outside filled area")
		}
	})

	t.Run("DrawBorder", func(t *testing.T) {
		buf := NewBuffer(20, 10)
		style := DefaultStyle()

		buf.DrawBorder(0, 0, 5, 3, BorderSingle, style)

		// Check corners
		if buf.Get(0, 0).Rune != BoxTopLeft {
			t.Error("expected top-left corner")
		}
		if buf.Get(4, 0).Rune != BoxTopRight {
			t.Error("expected top-right corner")
		}
		if buf.Get(0, 2).Rune != BoxBottomLeft {
			t.Error("expected bottom-left corner")
		}
		if buf.Get(4, 2).Rune != BoxBottomRight {
			t.Error("expected bottom-right corner")
		}

		// Check horizontal lines
		for x := 1; x < 4; x++ {
			if buf.Get(x, 0).Rune != BoxHorizontal {
				t.Errorf("expected horizontal at (%d,0)", x)
			}
		}

		// Check vertical lines
		if buf.Get(0, 1).Rune != BoxVertical {
			t.Error("expected vertical at (0,1)")
		}
	})

	t.Run("Resize", func(t *testing.T) {
		buf := NewBuffer(10, 10)
		buf.WriteString(0, 0, "Test", DefaultStyle())

		buf.Resize(20, 5)

		if buf.Width() != 20 || buf.Height() != 5 {
			t.Errorf("expected 20x5, got %dx%d", buf.Width(), buf.Height())
		}

		// Content should be preserved
		if buf.Get(0, 0).Rune != 'T' {
			t.Error("expected content to be preserved")
		}
	})
}

func TestWriteSparklineScalesToVisibleWindow(t *testing.T) {
	buf := NewBuffer(3, 1)
	buf.WriteSparkline(0, 0, []float64{0, 100, 101, 102}, 3, 0, 0, DefaultStyle())

	got := []rune{
		buf.Get(0, 0).Rune,
		buf.Get(1, 0).Rune,
		buf.Get(2, 0).Rune,
	}
	want := []rune{'▁', '▄', '█'}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("column %d: got %q, want %q", i, string(got[i]), string(want[i]))
		}
	}
}

func TestWriteSparklineMultiScalesToVisibleWindow(t *testing.T) {
	buf := NewBuffer(3, 2)
	buf.WriteSparklineMulti(0, 0, []float64{0, 100, 101, 102}, 3, 2, 0, 0, DefaultStyle())

	if got := buf.Get(0, 0).Rune; got != ' ' {
		t.Fatalf("top-left: got %q, want space", string(got))
	}
	if got := buf.Get(0, 1).Rune; got != ' ' {
		t.Fatalf("bottom-left: got %q, want space", string(got))
	}
	if got := buf.Get(1, 0).Rune; got != ' ' {
		t.Fatalf("top-middle: got %q, want space", string(got))
	}
	if got := buf.Get(1, 1).Rune; got != '█' {
		t.Fatalf("bottom-middle: got %q, want full block", string(got))
	}
	if got := buf.Get(2, 0).Rune; got != '█' {
		t.Fatalf("top-right: got %q, want full block", string(got))
	}
	if got := buf.Get(2, 1).Rune; got != '█' {
		t.Fatalf("bottom-right: got %q, want full block", string(got))
	}
}

func TestRegion(t *testing.T) {
	t.Run("Coordinates", func(t *testing.T) {
		buf := NewBuffer(20, 20)
		region := buf.Region(5, 5, 10, 10)

		if region.Width() != 10 || region.Height() != 10 {
			t.Errorf("expected 10x10, got %dx%d", region.Width(), region.Height())
		}
	})

	t.Run("SetGet", func(t *testing.T) {
		buf := NewBuffer(20, 20)
		region := buf.Region(5, 5, 10, 10)

		cell := NewCell('R', DefaultStyle().Foreground(Red))
		region.Set(0, 0, cell)

		// Should be at (5,5) in parent buffer
		got := buf.Get(5, 5)
		if !got.Equal(cell) {
			t.Error("region write should affect parent buffer")
		}

		// And readable from region
		got = region.Get(0, 0)
		if !got.Equal(cell) {
			t.Error("region read should work")
		}
	})

	t.Run("InBounds", func(t *testing.T) {
		buf := NewBuffer(20, 20)
		region := buf.Region(5, 5, 10, 10)

		if !region.InBounds(0, 0) {
			t.Error("(0,0) should be in bounds")
		}
		if !region.InBounds(9, 9) {
			t.Error("(9,9) should be in bounds")
		}
		if region.InBounds(10, 0) {
			t.Error("(10,0) should be out of bounds")
		}
		if region.InBounds(-1, 0) {
			t.Error("(-1,0) should be out of bounds")
		}
	})

	t.Run("WriteString", func(t *testing.T) {
		buf := NewBuffer(20, 20)
		region := buf.Region(5, 5, 10, 10)

		region.WriteString(0, 0, "Hi", DefaultStyle())

		// Check in parent buffer
		if buf.Get(5, 5).Rune != 'H' {
			t.Error("expected 'H' at (5,5) in parent")
		}
		if buf.Get(6, 5).Rune != 'i' {
			t.Error("expected 'i' at (6,5) in parent")
		}
	})
}

func BenchmarkBufferSet(b *testing.B) {
	buf := NewBuffer(200, 50)
	cell := NewCell('X', DefaultStyle().Foreground(Red))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := i % 200
		y := (i / 200) % 50
		buf.Set(x, y, cell)
	}
}

func BenchmarkBufferFill(b *testing.B) {
	buf := NewBuffer(200, 50)
	cell := NewCell('X', DefaultStyle().Foreground(Red))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Fill(cell)
	}
}

func BenchmarkBufferWriteString(b *testing.B) {
	buf := NewBuffer(200, 50)
	style := DefaultStyle()
	text := "Hello, World!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.WriteString(0, i%50, text, style)
	}
}

// TestBufferClip: PushClip drops writes outside the rect, PopClip restores, and
// nested clips compose by intersection.
func TestBufferClip(t *testing.T) {
	b := NewBuffer(10, 10)
	cell := NewCell('X', DefaultStyle())

	b.PushClip(2, 2, 8, 8) // half-open [2,8)x[2,8)
	b.Set(0, 0, cell)      // outside — dropped
	b.Set(2, 2, cell)      // inside — kept
	b.Set(7, 7, cell)      // inside (last) — kept
	b.Set(8, 8, cell)      // outside (half-open) — dropped
	if b.Get(0, 0).Rune == 'X' {
		t.Error("write outside clip was not dropped")
	}
	if b.Get(2, 2).Rune != 'X' || b.Get(7, 7).Rune != 'X' {
		t.Error("write inside clip was dropped")
	}
	if b.Get(8, 8).Rune == 'X' {
		t.Error("half-open upper bound not respected (8 should be excluded)")
	}

	// nested clip intersects: [2,8) ∩ [0,4) = [2,4)
	b.PushClip(0, 0, 4, 4)
	b.Set(3, 3, cell) // inside intersection
	b.Set(5, 5, cell) // outside the narrower clip
	if b.Get(3, 3).Rune != 'X' {
		t.Error("write inside nested intersection was dropped")
	}
	if b.Get(5, 5).Rune == 'X' {
		t.Error("nested clip did not intersect (5,5 should be excluded)")
	}

	b.PopClip() // back to [2,8)
	b.Set(5, 5, cell)
	if b.Get(5, 5).Rune != 'X' {
		t.Error("PopClip did not restore the outer clip")
	}

	b.PopClip() // no clip
	b.Set(0, 0, cell)
	if b.Get(0, 0).Rune != 'X' {
		t.Error("write after all clips popped was dropped")
	}
}

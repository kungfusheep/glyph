package tui

import "testing"

func TestList(t *testing.T) {
	t.Run("NewList", func(t *testing.T) {
		list := NewList[*TextComponent]()
		if list.Len() != 0 {
			t.Errorf("expected len 0, got %d", list.Len())
		}
		if list.direction != Vertical {
			t.Errorf("expected Vertical direction")
		}
	})

	t.Run("NewHList", func(t *testing.T) {
		list := NewHList[*TextComponent]()
		if list.direction != Horizontal {
			t.Errorf("expected Horizontal direction")
		}
	})

	t.Run("Add", func(t *testing.T) {
		list := NewList[*TextComponent]()
		list.Add(Text("one"), Text("two"))

		if list.Len() != 2 {
			t.Errorf("expected len 2, got %d", list.Len())
		}
		if list.At(0).text != "one" {
			t.Errorf("expected 'one', got %q", list.At(0).text)
		}
		if list.At(1).text != "two" {
			t.Errorf("expected 'two', got %q", list.At(1).text)
		}
	})

	t.Run("Insert", func(t *testing.T) {
		list := NewList[*TextComponent]()
		list.Add(Text("a"), Text("c"))
		list.Insert(1, Text("b"))

		if list.Len() != 3 {
			t.Errorf("expected len 3, got %d", list.Len())
		}
		if list.At(1).text != "b" {
			t.Errorf("expected 'b' at index 1, got %q", list.At(1).text)
		}
	})

	t.Run("InsertBounds", func(t *testing.T) {
		list := NewList[*TextComponent]()
		list.Add(Text("middle"))

		// Negative index should clamp to 0
		list.Insert(-5, Text("first"))
		if list.At(0).text != "first" {
			t.Errorf("expected 'first' at index 0")
		}

		// Index beyond length should append
		list.Insert(999, Text("last"))
		if list.At(list.Len()-1).text != "last" {
			t.Errorf("expected 'last' at end")
		}
	})

	t.Run("RemoveAt", func(t *testing.T) {
		list := NewList[*TextComponent]()
		list.Add(Text("a"), Text("b"), Text("c"))
		list.RemoveAt(1)

		if list.Len() != 2 {
			t.Errorf("expected len 2, got %d", list.Len())
		}
		if list.At(0).text != "a" || list.At(1).text != "c" {
			t.Errorf("expected [a,c]")
		}
	})

	t.Run("RemoveAtOutOfBounds", func(t *testing.T) {
		list := NewList[*TextComponent]()
		list.Add(Text("only"))

		list.RemoveAt(-1)
		list.RemoveAt(999)

		if list.Len() != 1 {
			t.Errorf("expected len unchanged")
		}
	})

	t.Run("Remove", func(t *testing.T) {
		list := NewList[*TextComponent]()
		item := Text("target")
		list.Add(Text("a"), item, Text("b"))

		list.Remove(item)

		if list.Len() != 2 {
			t.Errorf("expected len 2, got %d", list.Len())
		}
	})

	t.Run("Clear", func(t *testing.T) {
		list := NewList[*TextComponent]()
		list.Add(Text("a"), Text("b"))
		list.Clear()

		if list.Len() != 0 {
			t.Errorf("expected len 0, got %d", list.Len())
		}
	})

	t.Run("At OutOfBounds", func(t *testing.T) {
		list := NewList[*TextComponent]()
		list.Add(Text("only"))

		if list.At(-1) != nil {
			t.Errorf("expected nil for negative index")
		}
		if list.At(999) != nil {
			t.Errorf("expected nil for large index")
		}
	})

	t.Run("Items", func(t *testing.T) {
		list := NewList[*TextComponent]()
		list.Add(Text("a"), Text("b"))

		items := list.Items()
		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}
	})

	t.Run("FluentAPI", func(t *testing.T) {
		list := NewList[*TextComponent]().
			Gap(5).
			Padding(2).
			Border(BorderRounded).
			Background(Red)

		if list.gap != 5 {
			t.Errorf("expected gap 5, got %d", list.gap)
		}
		if list.padding != 2 {
			t.Errorf("expected padding 2, got %d", list.padding)
		}
		if list.border == nil {
			t.Errorf("expected border to be set")
		}
		if list.background == nil {
			t.Errorf("expected background to be set")
		}
	})

	t.Run("Children Sync", func(t *testing.T) {
		list := NewList[*TextComponent]()
		list.Add(Text("a"), Text("b"))

		children := list.Children()
		if len(children) != 2 {
			t.Errorf("expected 2 children, got %d", len(children))
		}

		list.RemoveAt(0)
		children = list.Children()
		if len(children) != 1 {
			t.Errorf("expected 1 child after remove, got %d", len(children))
		}
	})
}

func TestBoundList(t *testing.T) {
	t.Run("Bind creates components", func(t *testing.T) {
		data := NewObservable[string]()
		data.Add("hello")
		data.Add("world")

		bound := Bind(data, func(s string, idx int) *TextComponent {
			return Text(s)
		})

		if bound.List().Len() != 2 {
			t.Errorf("expected 2 items, got %d", bound.List().Len())
		}
	})

	t.Run("Add updates list", func(t *testing.T) {
		data := NewObservable[string]()
		bound := Bind(data, func(s string, idx int) *TextComponent {
			return Text(s)
		})

		data.Add("new")

		if bound.List().Len() != 1 {
			t.Errorf("expected 1 item, got %d", bound.List().Len())
		}
		if bound.List().At(0).text != "new" {
			t.Errorf("expected 'new', got %q", bound.List().At(0).text)
		}
	})

	t.Run("Remove updates list", func(t *testing.T) {
		data := NewObservable[string]()
		data.Add("a")
		data.Add("b")

		bound := Bind(data, func(s string, idx int) *TextComponent {
			return Text(s)
		})

		data.RemoveAt(0)

		if bound.List().Len() != 1 {
			t.Errorf("expected 1 item, got %d", bound.List().Len())
		}
		if bound.List().At(0).text != "b" {
			t.Errorf("expected 'b', got %q", bound.List().At(0).text)
		}
	})

	t.Run("Update with dispatcher", func(t *testing.T) {
		type Item struct {
			Text string
		}

		data := NewObservable[Item]()
		data.Add(Item{Text: "initial"})

		updateCalled := false
		bound := BindWith(data, Dispatcher[Item, *TextComponent]{
			Create: func(i Item, idx int) *TextComponent {
				return Text(i.Text)
			},
			Update: func(c *TextComponent, i Item, idx int) {
				updateCalled = true
				c.SetText(i.Text)
			},
		})

		data.Update(0, func(i *Item) {
			i.Text = "updated"
		})

		if !updateCalled {
			t.Errorf("expected Update to be called")
		}
		if bound.List().At(0).text != "updated" {
			t.Errorf("expected 'updated', got %q", bound.List().At(0).text)
		}
	})

	t.Run("Update without dispatcher recreates", func(t *testing.T) {
		data := NewObservable[string]()
		data.Add("original")

		createCount := 0
		bound := Bind(data, func(s string, idx int) *TextComponent {
			createCount++
			return Text(s)
		})

		data.Update(0, func(s *string) {
			*s = "modified"
		})

		// Should be called twice: initial create + recreate on update
		if createCount != 2 {
			t.Errorf("expected create called 2 times, got %d", createCount)
		}
		if bound.List().At(0).text != "modified" {
			t.Errorf("expected 'modified', got %q", bound.List().At(0).text)
		}
	})

	t.Run("FluentAPI", func(t *testing.T) {
		data := NewObservable[string]()
		bound := Bind(data, func(s string, idx int) *TextComponent {
			return Text(s)
		}).Gap(5).Padding(2).Border(BorderRounded).Background(Blue)

		if bound.List().gap != 5 {
			t.Errorf("expected gap 5")
		}
	})

	t.Run("Dispose", func(t *testing.T) {
		data := NewObservable[string]()
		callCount := 0

		bound := Bind(data, func(s string, idx int) *TextComponent {
			callCount++
			return Text(s)
		})

		data.Add("one")
		bound.Dispose()
		data.Add("two") // Should not trigger create

		if callCount != 1 {
			t.Errorf("expected 1 create call, got %d", callCount)
		}
	})
}

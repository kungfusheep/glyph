package tui

import "testing"

func TestObservable(t *testing.T) {
	t.Run("Add", func(t *testing.T) {
		obs := NewObservable[string]()
		obs.Add("one")
		obs.Add("two")

		if obs.Len() != 2 {
			t.Errorf("expected len 2, got %d", obs.Len())
		}
		if obs.At(0) != "one" {
			t.Errorf("expected 'one', got %q", obs.At(0))
		}
		if obs.At(1) != "two" {
			t.Errorf("expected 'two', got %q", obs.At(1))
		}
	})

	t.Run("Insert", func(t *testing.T) {
		obs := NewObservable[int]()
		obs.Add(1)
		obs.Add(3)
		obs.Insert(1, 2)

		if obs.Len() != 3 {
			t.Errorf("expected len 3, got %d", obs.Len())
		}
		items := obs.Items()
		if items[0] != 1 || items[1] != 2 || items[2] != 3 {
			t.Errorf("expected [1,2,3], got %v", items)
		}
	})

	t.Run("RemoveAt", func(t *testing.T) {
		obs := NewObservable[string]()
		obs.Add("a")
		obs.Add("b")
		obs.Add("c")
		obs.RemoveAt(1)

		if obs.Len() != 2 {
			t.Errorf("expected len 2, got %d", obs.Len())
		}
		if obs.At(0) != "a" || obs.At(1) != "c" {
			t.Errorf("expected [a,c], got %v", obs.Items())
		}
	})

	t.Run("Update", func(t *testing.T) {
		type Item struct {
			Value int
		}
		obs := NewObservable[Item]()
		obs.Add(Item{Value: 10})

		obs.Update(0, func(i *Item) {
			i.Value = 20
		})

		if obs.At(0).Value != 20 {
			t.Errorf("expected 20, got %d", obs.At(0).Value)
		}
	})

	t.Run("Clear", func(t *testing.T) {
		obs := NewObservable[int]()
		obs.Add(1)
		obs.Add(2)
		obs.Clear()

		if obs.Len() != 0 {
			t.Errorf("expected len 0, got %d", obs.Len())
		}
	})

	t.Run("Subscribe", func(t *testing.T) {
		obs := NewObservable[string]()
		var changes []Change[string]

		obs.Subscribe(func(c Change[string]) {
			changes = append(changes, c)
		})

		obs.Add("hello")
		obs.Update(0, func(s *string) { *s = "world" })
		obs.RemoveAt(0)

		if len(changes) != 3 {
			t.Fatalf("expected 3 changes, got %d", len(changes))
		}
		if changes[0].Type != ChangeAdd {
			t.Errorf("expected ChangeAdd, got %v", changes[0].Type)
		}
		if changes[1].Type != ChangeUpdate {
			t.Errorf("expected ChangeUpdate, got %v", changes[1].Type)
		}
		if changes[2].Type != ChangeRemove {
			t.Errorf("expected ChangeRemove, got %v", changes[2].Type)
		}
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		obs := NewObservable[int]()
		callCount := 0

		unsub := obs.Subscribe(func(c Change[int]) {
			callCount++
		})

		obs.Add(1)
		unsub()
		obs.Add(2)

		if callCount != 1 {
			t.Errorf("expected 1 call, got %d", callCount)
		}
	})

	t.Run("OutOfBounds", func(t *testing.T) {
		obs := NewObservable[int]()
		obs.Add(42)

		// At out of bounds returns zero value
		if obs.At(-1) != 0 {
			t.Errorf("expected 0 for negative index")
		}
		if obs.At(999) != 0 {
			t.Errorf("expected 0 for large index")
		}

		// Update out of bounds is no-op
		obs.Update(999, func(i *int) { *i = 100 })
		if obs.At(0) != 42 {
			t.Errorf("value should be unchanged")
		}

		// RemoveAt out of bounds is no-op
		obs.RemoveAt(-1)
		obs.RemoveAt(999)
		if obs.Len() != 1 {
			t.Errorf("len should be unchanged")
		}
	})
}

package ringbuf

import "testing"

func TestRingBuffer_Basic(t *testing.T) {
	rb := New[int](5)

	if rb.Len() != 0 {
		t.Errorf("Len = %d, want 0", rb.Len())
	}
	if rb.Cap() != 5 {
		t.Errorf("Cap = %d, want 5", rb.Cap())
	}

	for i := 0; i < 3; i++ {
		rb.Push(i)
	}
	if rb.Len() != 3 {
		t.Errorf("Len = %d, want 3", rb.Len())
	}

	for i := 0; i < 3; i++ {
		if got := rb.Get(i); got != i {
			t.Errorf("Get(%d) = %d, want %d", i, got, i)
		}
	}
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := New[int](3)
	for i := 0; i < 5; i++ {
		rb.Push(i)
	}

	if rb.Len() != 3 {
		t.Errorf("Len = %d, want 3", rb.Len())
	}

	// Should contain [2, 3, 4] (oldest items dropped)
	want := []int{2, 3, 4}
	got := rb.All()
	if len(got) != len(want) {
		t.Fatalf("All() len = %d, want %d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("All()[%d] = %d, want %d", i, got[i], v)
		}
	}
}

func TestRingBuffer_Slice(t *testing.T) {
	rb := New[int](5)
	for i := 0; i < 5; i++ {
		rb.Push(i * 10)
	}

	got := rb.Slice(1, 4)
	want := []int{10, 20, 30}
	if len(got) != len(want) {
		t.Fatalf("Slice len = %d, want %d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("Slice[%d] = %d, want %d", i, got[i], v)
		}
	}
}

func TestRingBuffer_Clear(t *testing.T) {
	rb := New[int](5)
	for i := 0; i < 3; i++ {
		rb.Push(i)
	}
	rb.Clear()
	if rb.Len() != 0 {
		t.Errorf("Len after clear = %d, want 0", rb.Len())
	}
}

func TestRingBuffer_OutOfBounds(t *testing.T) {
	rb := New[int](3)
	rb.Push(42)
	if got := rb.Get(-1); got != 0 {
		t.Errorf("Get(-1) = %d, want 0", got)
	}
	if got := rb.Get(5); got != 0 {
		t.Errorf("Get(5) = %d, want 0", got)
	}
}

func TestRingBuffer_ForEachAndLast(t *testing.T) {
	rb := New[int](3)
	for i := 0; i < 5; i++ {
		rb.Push(i)
	}

	var got []int
	rb.ForEach(func(item int) bool {
		got = append(got, item)
		return true
	})

	want := []int{2, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("ForEach len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ForEach[%d] = %d, want %d", i, got[i], want[i])
		}
	}

	last, ok := rb.Last()
	if !ok {
		t.Fatal("Last() should report ok")
	}
	if last != 4 {
		t.Fatalf("Last() = %d, want 4", last)
	}
}

// internal/client/history/history_test.go
package history_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/client/history"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHistory_DefaultCap(t *testing.T) {
	h := history.New(0)
	// Push 101 entries — oldest must be evicted
	for i := 0; i < 101; i++ {
		h.Push(string(rune('a' + i%26)))
	}
	// Navigate all the way back: should reach no further than cap=100 entries
	count := 0
	for h.Up() != "" {
		count++
	}
	assert.Equal(t, 100, count)
}

func TestHistory_UpDown_Empty(t *testing.T) {
	h := history.New(5)
	assert.Equal(t, "", h.Up())
	assert.Equal(t, "", h.Down())
}

func TestHistory_UpDown_Order(t *testing.T) {
	h := history.New(5)
	h.Push("first")
	h.Push("second")
	h.Push("third")

	assert.Equal(t, "third", h.Up())
	assert.Equal(t, "second", h.Up())
	assert.Equal(t, "first", h.Up())
	assert.Equal(t, "", h.Up()) // at oldest
	assert.Equal(t, "first", h.Down())
	assert.Equal(t, "second", h.Down())
	assert.Equal(t, "third", h.Down())
	assert.Equal(t, "", h.Down()) // past newest (live position)
}

func TestHistory_PushResetsCorsor(t *testing.T) {
	h := history.New(5)
	h.Push("a")
	h.Push("b")
	h.Up() // move cursor back
	h.Push("c")
	// After Push, cursor resets: Up() returns "c" (most recent)
	assert.Equal(t, "c", h.Up())
}

func TestHistory_Reset(t *testing.T) {
	h := history.New(5)
	h.Push("x")
	h.Push("y")
	h.Up()
	h.Up()
	h.Reset()
	assert.Equal(t, "y", h.Up()) // cursor back at live position
}

func TestHistory_Property_CapEnforced(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cap := rapid.IntRange(1, 50).Draw(rt, "cap")
		n := rapid.IntRange(cap, cap*3).Draw(rt, "n")
		h := history.New(cap)
		for i := 0; i < n; i++ {
			h.Push("entry")
		}
		count := 0
		for h.Up() != "" {
			count++
			if count > cap {
				rt.Fatalf("navigated more than cap=%d entries", cap)
			}
		}
		if count != cap {
			rt.Fatalf("expected %d entries, got %d", cap, count)
		}
	})
}

func TestHistory_Property_UpDownSymmetry(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cap := rapid.IntRange(2, 20).Draw(rt, "cap")
		n := rapid.IntRange(1, cap).Draw(rt, "n")
		h := history.New(cap)
		entries := make([]string, n)
		for i := 0; i < n; i++ {
			entries[i] = rapid.StringMatching(`[a-z]+`).Draw(rt, "entry")
			h.Push(entries[i])
		}
		// Navigate all the way back
		collected := []string{}
		for {
			v := h.Up()
			if v == "" {
				break
			}
			collected = append(collected, v)
		}
		// Navigate all the way forward
		rebuilt := []string{}
		for {
			v := h.Down()
			if v == "" {
				break
			}
			rebuilt = append(rebuilt, v)
		}
		// collected is newest→oldest; rebuilt is oldest→newest
		// rebuilt reversed must equal collected
		for i, j := 0, len(rebuilt)-1; i < j; i, j = i+1, j-1 {
			rebuilt[i], rebuilt[j] = rebuilt[j], rebuilt[i]
		}
		if len(collected) != len(rebuilt) {
			rt.Fatalf("up count %d != down count %d", len(collected), len(rebuilt))
		}
		for i := range collected {
			if collected[i] != rebuilt[i] {
				rt.Fatalf("mismatch at %d: up=%q down=%q", i, collected[i], rebuilt[i])
			}
		}
	})
}

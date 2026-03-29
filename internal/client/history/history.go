// internal/client/history/history.go
package history

const defaultCap = 100

// History is a command ring buffer with ↑/↓ cursor navigation.
// Not goroutine-safe — designed for single-input-goroutine use.
type History struct {
	buf    []string
	cap    int
	head   int // index of oldest entry
	count  int // number of valid entries
	cursor int // offset from newest: 0 = live, 1 = newest, count = oldest
}

// New creates a History with the given capacity. cap=0 uses the default (100).
func New(cap int) *History {
	if cap <= 0 {
		cap = defaultCap
	}
	return &History{buf: make([]string, cap), cap: cap}
}

// Push adds a command to the history and resets the cursor to the live position.
func (h *History) Push(cmd string) {
	if cmd == "" {
		return
	}
	idx := (h.head + h.count) % h.cap
	if h.count < h.cap {
		h.count++
	} else {
		// Overwrite oldest — advance head
		h.head = (h.head + 1) % h.cap
	}
	h.buf[idx] = cmd
	h.cursor = 0
}

// Up navigates toward older entries. Returns "" when already at the oldest entry.
func (h *History) Up() string {
	if h.cursor >= h.count {
		return ""
	}
	h.cursor++
	return h.buf[(h.head+h.count-h.cursor+h.cap)%h.cap]
}

// Down navigates toward newer entries. Returns "" when at the live position.
func (h *History) Down() string {
	if h.cursor <= 0 {
		return ""
	}
	val := h.buf[(h.head+h.count-h.cursor+h.cap)%h.cap]
	h.cursor--
	return val
}

// Reset returns the cursor to the live position (call after each command submission).
func (h *History) Reset() {
	h.cursor = 0
}

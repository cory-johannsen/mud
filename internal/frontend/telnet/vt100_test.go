package telnet

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// vt100Screen is a minimal VT100 terminal emulator for layout testing.
// It interprets ANSI CSI escape sequences and maintains a virtual screen
// buffer, cursor position, scroll region, and cursor-save stack.
//
// Supported sequences (sufficient for the split-screen protocol):
//   \033[2J       – erase screen
//   \033[H        – cursor home (1,1)
//   \033[r;cH     – cursor position
//   \033[N;Mr     – set scroll region (DECSTBM), resets cursor to (1,1)
//   \033[2K       – erase entire line
//   \033[s        – save cursor
//   \033[u        – restore cursor
//   \033[?25l/h   – hide/show cursor (ignored)
//   \033[…m       – SGR color codes (ignored)
//   \r            – carriage return
//   \n            – line feed (scroll-region aware)
//   printable     – written at cursor; cursor advances
type vt100Screen struct {
	width, height      int
	cells              [][]rune
	curRow, curCol     int // 1-based
	scrollTop          int // 1-based, inclusive
	scrollBottom       int // 1-based, inclusive
	savedRow, savedCol int
}

func newVT100Screen(width, height int) *vt100Screen {
	cells := make([][]rune, height)
	for i := range cells {
		cells[i] = make([]rune, width)
		for j := range cells[i] {
			cells[i][j] = ' '
		}
	}
	return &vt100Screen{
		width:  width,
		height: height,
		cells:  cells,
		curRow: 1, curCol: 1,
		scrollTop:    1,
		scrollBottom: height,
	}
}

// Feed processes a raw byte string as if it were terminal output.
// It correctly decodes UTF-8 multi-byte sequences (e.g. ═ U+2550) so that
// each Unicode code point occupies exactly one cell in the screen buffer.
func (s *vt100Screen) Feed(data string) {
	i := 0
	for i < len(data) {
		if data[i] == '\033' && i+1 < len(data) && data[i+1] == '[' {
			// CSI sequence: collect params until final byte [0x40–0x7E]
			j := i + 2
			for j < len(data) && !isCSIFinal(data[j]) {
				j++
			}
			if j < len(data) {
				s.handleCSI(data[i+2:j], data[j])
				i = j + 1
				continue
			}
		}
		switch data[i] {
		case '\r':
			s.curCol = 1
			i++
		case '\n':
			s.lineFeed()
			i++
		default:
			// Decode a full UTF-8 rune so multi-byte characters (e.g. ═ U+2550)
			// occupy exactly one screen cell instead of three separate bytes.
			r, size := utf8.DecodeRuneInString(data[i:])
			if r >= 32 && r != utf8.RuneError && s.curCol >= 1 && s.curCol <= s.width {
				s.cells[s.curRow-1][s.curCol-1] = r
				s.curCol++
			}
			i += size
		}
	}
}

// isCSIFinal reports whether c is a valid CSI final byte (0x40–0x7E).
func isCSIFinal(c byte) bool {
	return c >= 0x40 && c <= 0x7E
}

// handleCSI dispatches a parsed CSI sequence.
func (s *vt100Screen) handleCSI(params string, final byte) {
	switch final {
	case 'H': // cursor position: \033[r;cH  (default 1,1)
		r, c := parseTwo(params, 1, 1)
		s.curRow = clampInt(r, 1, s.height)
		s.curCol = clampInt(c, 1, s.width)

	case 'r': // DECSTBM – set scroll region: \033[top;bottomr
		top, bottom := parseTwo(params, 1, s.height)
		s.scrollTop = clampInt(top, 1, s.height)
		s.scrollBottom = clampInt(bottom, 1, s.height)
		// DECSTBM resets cursor to (1,1) (absolute home, DECOM=off).
		s.curRow, s.curCol = 1, 1

	case 'J': // erase in display
		n := parseOne(params, 0)
		if n == 2 { // erase entire screen
			for r := 0; r < s.height; r++ {
				for c := 0; c < s.width; c++ {
					s.cells[r][c] = ' '
				}
			}
		}

	case 'K': // erase in line
		n := parseOne(params, 0)
		switch n {
		case 0: // cursor to end of line
			for c := s.curCol - 1; c < s.width; c++ {
				s.cells[s.curRow-1][c] = ' '
			}
		case 1: // start of line to cursor
			for c := 0; c < s.curCol; c++ {
				s.cells[s.curRow-1][c] = ' '
			}
		case 2: // entire line
			for c := 0; c < s.width; c++ {
				s.cells[s.curRow-1][c] = ' '
			}
		}

	case 's': // save cursor position
		s.savedRow, s.savedCol = s.curRow, s.curCol

	case 'u': // restore cursor position
		s.curRow = clampInt(s.savedRow, 1, s.height)
		s.curCol = clampInt(s.savedCol, 1, s.width)

	case 'm': // SGR – color/style (ignore; no effect on screen layout)
	case 'l', 'h': // cursor visibility (?25l/h) and other modes – ignore
	}
}

// lineFeed advances the cursor down one row, scrolling the scroll region if needed.
func (s *vt100Screen) lineFeed() {
	if s.curRow == s.scrollBottom {
		// Scroll region up by one line.
		copy(s.cells[s.scrollTop-1:s.scrollBottom-1], s.cells[s.scrollTop:s.scrollBottom])
		for c := 0; c < s.width; c++ {
			s.cells[s.scrollBottom-1][c] = ' '
		}
	} else if s.curRow < s.height {
		s.curRow++
	}
}

// RowText returns the visible (non-trailing-space) content of row r (1-based).
func (s *vt100Screen) RowText(row int) string {
	if row < 1 || row > s.height {
		return ""
	}
	return strings.TrimRight(string(s.cells[row-1]), " ")
}

// RowFull returns the full content of row r (1-based) including trailing spaces.
func (s *vt100Screen) RowFull(row int) string {
	if row < 1 || row > s.height {
		return ""
	}
	return string(s.cells[row-1])
}

// CursorRow returns the current 1-based cursor row.
func (s *vt100Screen) CursorRow() int { return s.curRow }

// CursorCol returns the current 1-based cursor column.
func (s *vt100Screen) CursorCol() int { return s.curCol }

// parseOne parses a single integer parameter (e.g. "2"), returning def if empty.
func parseOne(params string, def int) int {
	if params == "" {
		return def
	}
	n, err := strconv.Atoi(params)
	if err != nil {
		return def
	}
	return n
}

// parseTwo parses "a;b" params returning (a or defA, b or defB).
func parseTwo(params string, defA, defB int) (int, int) {
	if params == "" {
		return defA, defB
	}
	parts := strings.SplitN(params, ";", 2)
	a := parseOne(parts[0], defA)
	b := defB
	if len(parts) == 2 {
		b = parseOne(parts[1], defB)
	}
	return a, b
}

// clampInt clamps v to [lo, hi].
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// tHelper is the minimal interface needed by our test helpers.
// Both *testing.T and *rapid.T satisfy this.
type tHelper interface {
	Helper()
	Logf(string, ...interface{})
	Cleanup(func())
}

// feedScreen drains bytes from the client net.Conn into the VT100 screen emulator.
func feedScreen(t tHelper, screen *vt100Screen, client net.Conn, d time.Duration) {
	t.Helper()
	raw := readAllConn(client, d)
	screen.Feed(raw)
}

// readAllConn drains bytes from a net.Conn until the read deadline expires.
func readAllConn(client net.Conn, d time.Duration) string {
	_ = client.SetReadDeadline(time.Now().Add(d))
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, err := client.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(buf)
}

// newSplitConnTB creates a split-screen Conn pair for testing.
// Accepts any tHelper (works with *testing.T and *rapid.T).
func newSplitConnTB(t tHelper, w, h int) (*Conn, net.Conn) {
	t.Helper()
	client, server := net.Pipe()
	conn := NewConn(server, 2*time.Second, 2*time.Second)
	conn.mu.Lock()
	conn.width = w
	conn.height = h
	conn.splitScreen = true
	conn.mu.Unlock()
	t.Cleanup(func() {
		client.Close()
		conn.Close()
	})
	return conn, client
}

// ─── InitScreen layout tests ──────────────────────────────────────────────────

func TestIntegration_InitScreen_ScrollRegionAndDividers(t *testing.T) {
	const W, H = 80, 24
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	go func() { _ = conn.InitScreen() }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	// Scroll region must be rows 8..(H-2)=22
	assert.Equal(t, 8, screen.scrollTop, "scroll region top must be row 8")
	assert.Equal(t, H-2, screen.scrollBottom, "scroll region bottom must be row H-2")

	// Row 7 must contain the top divider (═ repeated)
	assert.Contains(t, screen.RowText(7), "═", "row 7 must have top divider")

	// Row H-1 must contain the bottom divider
	assert.Contains(t, screen.RowText(H-1), "═", "row H-1 must have bottom divider")

	// Rows 1-6 must be blank after InitScreen (no room content written yet)
	for r := 1; r <= 6; r++ {
		assert.Empty(t, screen.RowText(r), fmt.Sprintf("row %d must be blank after InitScreen", r))
	}
}

// ─── WriteRoom layout tests ───────────────────────────────────────────────────

func TestIntegration_WriteRoom_PlacesLinesInRows1to6(t *testing.T) {
	const W, H = 80, 24
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	content := "Title\nDescription text.\nExits:\n  north\n  south"
	go func() { _ = conn.WriteRoom(content) }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	assert.Equal(t, "Title", screen.RowText(1), "row 1 must contain title")
	assert.Equal(t, "Description text.", screen.RowText(2), "row 2 must contain description")
	assert.Equal(t, "Exits:", screen.RowText(3), "row 3 must contain 'Exits:'")
	assert.Equal(t, "  north", screen.RowText(4), "row 4 must contain north exit")
	assert.Equal(t, "  south", screen.RowText(5), "row 5 must contain south exit")
	assert.Empty(t, screen.RowText(6), "row 6 must be blank (no 6th content line)")
}

func TestIntegration_WriteRoom_ExcessLinesAreTruncated(t *testing.T) {
	const W, H = 80, 24
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	// 8 lines: only first 6 should appear
	lines := []string{"L1", "L2", "L3", "L4", "L5", "L6", "L7", "L8"}
	go func() { _ = conn.WriteRoom(strings.Join(lines, "\n")) }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	for i := 1; i <= 6; i++ {
		assert.Equal(t, fmt.Sprintf("L%d", i), screen.RowText(i))
	}
	// Rows 7+ must not be overwritten (row 7 is the divider region, not a room row)
}

func TestIntegration_WriteRoom_EmptyContent_ClearsAll6Rows(t *testing.T) {
	const W, H = 80, 24
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	// Pre-fill rows 1-6 with garbage to ensure WriteRoom clears them.
	for r := 0; r < 6; r++ {
		for c := 0; c < W; c++ {
			screen.cells[r][c] = 'X'
		}
	}

	go func() { _ = conn.WriteRoom("") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	for r := 1; r <= 6; r++ {
		assert.Empty(t, screen.RowText(r), fmt.Sprintf("row %d must be blank after WriteRoom('')", r))
	}
}

func TestIntegration_WriteRoom_RestoresCursorToSavedPosition(t *testing.T) {
	const W, H = 80, 24
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	// Position cursor at row H (input row) before WriteRoom, as gameBridge does.
	screen.curRow, screen.curCol = H, 1
	screen.savedRow, screen.savedCol = H, 1

	go func() { _ = conn.WriteRoom("Hello\nWorld") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	// Cursor must be restored to saved position (row H).
	assert.Equal(t, H, screen.CursorRow(), "cursor must be restored to row H after WriteRoom")
}

func TestIntegration_WriteRoom_DoesNotCorruptScrollRegionRows(t *testing.T) {
	const W, H = 80, 24
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	// Simulate InitScreen first: set scroll region so the emulator knows the layout.
	screen.scrollTop, screen.scrollBottom = 8, H-2

	// Pre-fill scroll region rows with sentinel text.
	for r := 8; r <= H-2; r++ {
		copy(screen.cells[r-1], []rune("SCROLL_SENTINEL                                                                 "))
	}

	go func() { _ = conn.WriteRoom("Title\nDesc") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	// Scroll region rows must be untouched (WriteRoom only touches rows 1-6).
	for r := 8; r <= H-2; r++ {
		assert.Contains(t, screen.RowFull(r), "SCROLL",
			fmt.Sprintf("scroll region row %d must not be overwritten by WriteRoom", r))
	}
}

// ─── WriteConsole layout tests ────────────────────────────────────────────────

func TestIntegration_WriteConsole_AppendsInScrollRegion(t *testing.T) {
	const W, H = 80, 24
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	// Simulate the scroll region established by InitScreen.
	screen.scrollTop, screen.scrollBottom = 8, H-2

	go func() { _ = conn.WriteConsole("A goblin attacks!") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	// Message must appear somewhere in the scroll region (rows 8..H-2).
	found := false
	for r := 8; r <= H-2; r++ {
		if strings.Contains(screen.RowText(r), "A goblin attacks!") {
			found = true
			break
		}
	}
	assert.True(t, found, "console message must appear in the scroll region rows 8..H-2")
}

func TestIntegration_WriteConsole_RedrawsBottomDividerAtHMinus1(t *testing.T) {
	const W, H = 80, 24
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	screen.scrollTop, screen.scrollBottom = 8, H-2

	go func() { _ = conn.WriteConsole("test") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	assert.Contains(t, screen.RowText(H-1), "═", "bottom divider must be redrawn at row H-1")
}

func TestIntegration_WriteConsole_RedrawsPromptAtRowH(t *testing.T) {
	const W, H = 80, 24
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	conn.mu.Lock()
	conn.inputBuf = "partial"
	conn.mu.Unlock()

	screen.scrollTop, screen.scrollBottom = 8, H-2

	go func() { _ = conn.WriteConsole("message") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	assert.Contains(t, screen.RowText(H), "partial",
		"prompt row H must show preserved partial input after WriteConsole")
}

func TestIntegration_WriteConsole_RedrawsRoomRegion(t *testing.T) {
	const W, H = 80, 24
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	conn.mu.Lock()
	conn.roomBuf = "MyRoom\nA dark chamber.\nExits:\n  east"
	conn.mu.Unlock()

	screen.scrollTop, screen.scrollBottom = 8, H-2

	go func() { _ = conn.WriteConsole("NPC says hello.") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	// Room region must be redrawn.
	assert.Equal(t, "MyRoom", screen.RowText(1), "room title must be redrawn at row 1 after WriteConsole")
	assert.Equal(t, "A dark chamber.", screen.RowText(2), "room desc must be redrawn at row 2 after WriteConsole")
}

// ─── WritePromptSplit tests ───────────────────────────────────────────────────

func TestIntegration_WritePromptSplit_PlacesPromptAtRowH(t *testing.T) {
	const W, H = 80, 24
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	go func() { _ = conn.WritePromptSplit("[Newb]> ") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	assert.Contains(t, screen.RowFull(H), "[Newb]> ",
		"prompt must be placed at row H (the input row)")
}

// ─── Full layout integration: InitScreen + WriteRoom + WriteConsole ───────────

func TestIntegration_FullLayout_InitThenWriteRoomThenConsole(t *testing.T) {
	const W, H = 80, 24
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	go func() {
		_ = conn.InitScreen()
		conn.mu.Lock()
		conn.roomBuf = ""
		conn.mu.Unlock()
		_ = conn.WriteRoom("Rotgut Alley\nA grungy street.\nExits:\n  south\n  east")
		conn.mu.Lock()
		conn.roomBuf = "Rotgut Alley\nA grungy street.\nExits:\n  south\n  east"
		conn.mu.Unlock()
		_ = conn.WriteConsole("Ganger says: keep walking.")
		// In the real game, WritePromptSplit is called after every WriteConsole
		// to restore the prompt at row H.
		_ = conn.WritePromptSplit("[Newb]> ")
	}()
	feedScreen(t, screen, client, 500*time.Millisecond)

	// Room region rows 1-5 must have room content.
	assert.Equal(t, "Rotgut Alley", screen.RowText(1), "row 1: room title")
	assert.Equal(t, "A grungy street.", screen.RowText(2), "row 2: room description")
	assert.Equal(t, "Exits:", screen.RowText(3), "row 3: exits header")
	assert.Equal(t, "  south", screen.RowText(4), "row 4: south exit")
	assert.Equal(t, "  east", screen.RowText(5), "row 5: east exit")
	assert.Empty(t, screen.RowText(6), "row 6: blank")

	// Top divider at row 7.
	assert.Contains(t, screen.RowText(7), "═", "row 7: top divider")

	// Console message in scroll region.
	found := false
	for r := 8; r <= H-2; r++ {
		if strings.Contains(screen.RowText(r), "Ganger says") {
			found = true
			break
		}
	}
	assert.True(t, found, "console message must appear in scroll region")

	// Bottom divider at row H-1.
	assert.Contains(t, screen.RowText(H-1), "═", "row H-1: bottom divider")

	// Prompt at row H.
	assert.Contains(t, screen.RowFull(H), "[Newb]> ", "row H: prompt")
}

// ─── Property-based tests ─────────────────────────────────────────────────────

// TestProperty_WriteRoom_RoomRegionNeverOverflowsIntoScrollRegion verifies that
// WriteRoom never writes to rows 7+ regardless of content size or terminal size.
func TestProperty_WriteRoom_RoomRegionNeverOverflowsIntoScrollRegion(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		W := rapid.IntRange(40, 200).Draw(rt, "width")
		H := rapid.IntRange(12, 50).Draw(rt, "height")
		lineCount := rapid.IntRange(0, 20).Draw(rt, "lines")

		lines := make([]string, lineCount)
		for i := range lines {
			lines[i] = rapid.StringMatching(`[A-Za-z0-9 ]{1,30}`).Draw(rt, fmt.Sprintf("line%d", i))
		}

		screen := newVT100Screen(W, H)
		// Simulate scroll region from InitScreen.
		screen.scrollTop, screen.scrollBottom = 8, H-2

		// Pre-fill rows 7+ with sentinel so any overwrite is detectable.
		for r := 7; r <= H; r++ {
			for c := 0; c < W; c++ {
				screen.cells[r-1][c] = '.'
			}
		}

		conn, client := newSplitConnTB(rt, W, H)
		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = conn.WriteRoom(strings.Join(lines, "\n"))
			conn.Close()
		}()
		raw := readAllConn(client, 2*time.Second)
		<-done
		screen.Feed(raw)

		// WriteRoom now intentionally redraws the top divider at row 7.
		// Row 7 should contain "═" (divider), not "." (sentinel).
		require.Contains(rt, screen.RowFull(7), "═",
			fmt.Sprintf("WriteRoom must redraw top divider at row 7 (W=%d H=%d lines=%d)", W, H, lineCount))
		// Rows 8+ (scroll region and below) must not be touched.
		for r := 8; r <= H; r++ {
			rowContent := screen.RowFull(r)
			require.Contains(rt, rowContent, ".",
				fmt.Sprintf("WriteRoom overflowed into row %d (W=%d H=%d lines=%d)", r, W, H, lineCount))
		}
	})
}

// TestProperty_WriteRoom_AlwaysShowsFirstNLinesInOrder verifies that the first
// min(len(lines), roomRegionRows) lines of content appear in rows 1..N in order.
func TestProperty_WriteRoom_AlwaysShowsFirstNLinesInOrder(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		W := rapid.IntRange(40, 200).Draw(rt, "width")
		H := rapid.IntRange(12, 50).Draw(rt, "height")
		lineCount := rapid.IntRange(1, 15).Draw(rt, "lines")

		lines := make([]string, lineCount)
		for i := range lines {
			// Short, simple ASCII lines — no ANSI codes in this property test.
			lines[i] = fmt.Sprintf("L%03d", i+1)
		}

		screen := newVT100Screen(W, H)
		screen.scrollTop, screen.scrollBottom = 8, H-2

		conn, client := newSplitConnTB(rt, W, H)
		go func() {
			_ = conn.WriteRoom(strings.Join(lines, "\n"))
			conn.Close()
		}()
		raw := readAllConn(client, 2*time.Second)
		screen.Feed(raw)

		expected := lineCount
		if expected > roomRegionRows {
			expected = roomRegionRows
		}
		for i := 0; i < expected; i++ {
			row := i + 1
			got := screen.RowText(row)
			require.Equal(rt, lines[i], got,
				fmt.Sprintf("row %d must contain line %d content (W=%d H=%d)", row, i+1, W, H))
		}
	})
}

// TestProperty_WriteConsole_NeverOverwritesRoomRegion verifies that WriteConsole
// does not corrupt rows 1-6 with console text (it may redraw them via appendRoomRedraw,
// but the room content must remain correct).
func TestProperty_WriteConsole_NeverOverwritesRoomRegion(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		W := rapid.IntRange(40, 200).Draw(rt, "width")
		H := rapid.IntRange(12, 50).Draw(rt, "height")

		roomContent := "RoomTitle\nRoom description.\nExits:\n  north"
		consoleMsg := rapid.StringMatching(`[A-Za-z0-9 !?,]{5,40}`).Draw(rt, "msg")

		screen := newVT100Screen(W, H)
		screen.scrollTop, screen.scrollBottom = 8, H-2

		conn, client := newSplitConnTB(rt, W, H)
		conn.mu.Lock()
		conn.roomBuf = roomContent
		conn.mu.Unlock()

		go func() {
			_ = conn.WriteConsole(consoleMsg)
			conn.Close()
		}()
		raw := readAllConn(client, 2*time.Second)
		screen.Feed(raw)

		// The console message must NOT appear in rows 1-6.
		for r := 1; r <= 6; r++ {
			require.NotContains(rt, screen.RowText(r), consoleMsg,
				fmt.Sprintf("row %d must not contain console text (W=%d H=%d)", r, W, H))
		}

		// Room title must still be in row 1.
		require.Equal(rt, "RoomTitle", screen.RowText(1),
			fmt.Sprintf("room title must remain in row 1 after WriteConsole (W=%d H=%d)", W, H))
	})
}

// TestProperty_WriteConsole_MessageAlwaysInScrollRegion verifies that the console
// message always ends up somewhere in the scroll region rows 8..H-2.
func TestProperty_WriteConsole_MessageAlwaysInScrollRegion(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		W := rapid.IntRange(40, 200).Draw(rt, "width")
		H := rapid.IntRange(12, 50).Draw(rt, "height")

		// Simple short message that fits in one line.
		msg := "SentinelMsg"

		screen := newVT100Screen(W, H)
		screen.scrollTop, screen.scrollBottom = 8, H-2

		conn, client := newSplitConnTB(rt, W, H)

		go func() {
			_ = conn.WriteConsole(msg)
			conn.Close()
		}()
		raw := readAllConn(client, 2*time.Second)
		screen.Feed(raw)

		found := false
		for r := 8; r <= H-2; r++ {
			if strings.Contains(screen.RowText(r), msg) {
				found = true
				break
			}
		}
		require.True(rt, found,
			fmt.Sprintf("console message must appear in scroll region 8..%d (W=%d H=%d)", H-2, W, H))
	})
}

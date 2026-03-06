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

	case 'A': // CUU – cursor up N rows (no scroll)
		n := parseOne(params, 1)
		s.curRow = clampInt(s.curRow-n, 1, s.height)

	case 'B': // CUD – cursor down N rows (no scroll)
		n := parseOne(params, 1)
		s.curRow = clampInt(s.curRow+n, 1, s.height)

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

// TestIntegration_InitScreen_ScrollRegionAndDivider verifies InitScreen configures
// the new bottom-room layout: scroll region 1..scrollBottom, divider at dividerRow,
// blank room rows, cursor at promptRow.
func TestIntegration_InitScreen_ScrollRegionAndDivider(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H) // scrollBottom=16, dividerRow=17, firstRow=18, lastRow=23, promptRow=24
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	go func() { _ = conn.InitScreen() }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	// Scroll region must be rows 1..scrollBottom.
	assert.Equal(t, 1, screen.scrollTop, "scroll region top must be row 1")
	assert.Equal(t, lo.scrollBottom, screen.scrollBottom, "scroll region bottom must be scrollBottom")

	// InitScreen does NOT draw the divider — WriteRoom is the sole divider drawer.
	assert.NotContains(t, screen.RowText(lo.dividerRow), "═",
		fmt.Sprintf("row %d (dividerRow) must NOT have divider after InitScreen alone", lo.dividerRow))

	// Room content rows must be blank after InitScreen.
	for r := lo.firstRow; r <= lo.lastRow; r++ {
		assert.Empty(t, screen.RowText(r),
			fmt.Sprintf("room row %d must be blank after InitScreen", r))
	}
}

// ─── WriteRoom layout tests ───────────────────────────────────────────────────

func TestIntegration_WriteRoom_PlacesLinesInRoomRows(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	content := "Title\nDescription text.\nExits:\n  north\n  south"
	go func() { _ = conn.WriteRoom(content) }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	assert.Equal(t, "Title", screen.RowText(lo.firstRow+0), "firstRow must contain title")
	assert.Equal(t, "Description text.", screen.RowText(lo.firstRow+1), "firstRow+1 must contain description")
	assert.Equal(t, "Exits:", screen.RowText(lo.firstRow+2), "firstRow+2 must contain 'Exits:'")
	assert.Equal(t, "  north", screen.RowText(lo.firstRow+3), "firstRow+3 must contain north exit")
	assert.Equal(t, "  south", screen.RowText(lo.firstRow+4), "firstRow+4 must contain south exit")
	assert.Empty(t, screen.RowText(lo.firstRow+5), "firstRow+5 must be blank (no 6th content line)")
}

func TestIntegration_WriteRoom_ExcessLinesAreTruncated(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	// 8 lines: only first 6 should appear in room rows.
	lines := []string{"L1", "L2", "L3", "L4", "L5", "L6", "L7", "L8"}
	go func() { _ = conn.WriteRoom(strings.Join(lines, "\n")) }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	for i := 0; i < roomRegionRows; i++ {
		assert.Equal(t, fmt.Sprintf("L%d", i+1), screen.RowText(lo.firstRow+i),
			fmt.Sprintf("room row %d must contain L%d", lo.firstRow+i, i+1))
	}
}

func TestIntegration_WriteRoom_EmptyContent_ClearsAllRoomRows(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	// Pre-fill room rows with garbage to ensure WriteRoom clears them.
	for r := lo.firstRow; r <= lo.lastRow; r++ {
		for c := 0; c < W; c++ {
			screen.cells[r-1][c] = 'X'
		}
	}

	go func() { _ = conn.WriteRoom("") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	for r := lo.firstRow; r <= lo.lastRow; r++ {
		assert.Empty(t, screen.RowText(r),
			fmt.Sprintf("room row %d must be blank after WriteRoom('')", r))
	}
}

func TestIntegration_WriteRoom_LeavesCursorAtPromptRow(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	go func() { _ = conn.WriteRoom("Hello\nWorld") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	// Cursor must be at promptRow after WriteRoom.
	assert.Equal(t, lo.promptRow, screen.CursorRow(),
		"cursor must be at promptRow after WriteRoom")
}

func TestIntegration_WriteRoom_DoesNotCorruptScrollRegionRows(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	// Simulate InitScreen: scroll region 1..scrollBottom.
	screen.scrollTop, screen.scrollBottom = 1, lo.scrollBottom

	// Pre-fill scroll region rows with sentinel text.
	for r := 1; r <= lo.scrollBottom; r++ {
		copy(screen.cells[r-1], []rune("SCROLL_SENTINEL                                                                 "))
	}

	go func() { _ = conn.WriteRoom("Title\nDesc") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	// Scroll region rows must be untouched (WriteRoom only touches dividerRow..promptRow).
	for r := 1; r <= lo.scrollBottom; r++ {
		assert.Contains(t, screen.RowFull(r), "SCROLL",
			fmt.Sprintf("scroll region row %d must not be overwritten by WriteRoom", r))
	}
}

// ─── WriteConsole layout tests ────────────────────────────────────────────────

func TestIntegration_WriteConsole_AppendsInScrollRegion(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	// Simulate the scroll region established by InitScreen.
	screen.scrollTop, screen.scrollBottom = 1, lo.scrollBottom

	go func() { _ = conn.WriteConsole("A goblin attacks!") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	// Message must appear somewhere in the scroll region (rows 1..scrollBottom).
	found := false
	for r := 1; r <= lo.scrollBottom; r++ {
		if strings.Contains(screen.RowText(r), "A goblin attacks!") {
			found = true
			break
		}
	}
	assert.True(t, found,
		fmt.Sprintf("console message must appear in scroll region rows 1..%d", lo.scrollBottom))
}

func TestIntegration_WriteConsole_RedrawsDividerAtDividerRow(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	screen.scrollTop, screen.scrollBottom = 1, lo.scrollBottom

	go func() { _ = conn.WriteConsole("test") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	assert.Contains(t, screen.RowText(lo.dividerRow), "═",
		fmt.Sprintf("room divider must be redrawn at row %d (dividerRow)", lo.dividerRow))
}

func TestIntegration_WriteConsole_RedrawsPromptAtRowH(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	conn.mu.Lock()
	conn.inputBuf = "partial"
	conn.mu.Unlock()

	screen.scrollTop, screen.scrollBottom = 1, lo.scrollBottom

	go func() { _ = conn.WriteConsole("message") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	assert.Contains(t, screen.RowText(lo.promptRow), "partial",
		"prompt row must show preserved partial input after WriteConsole")
}

func TestIntegration_WriteConsole_RedrawsRoomRegion(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	conn.mu.Lock()
	conn.roomBuf = "MyRoom\nA dark chamber.\nExits:\n  east"
	conn.mu.Unlock()

	screen.scrollTop, screen.scrollBottom = 1, lo.scrollBottom

	go func() { _ = conn.WriteConsole("NPC says hello.") }()
	feedScreen(t, screen, client, 500*time.Millisecond)

	// Room region must be redrawn.
	assert.Equal(t, "MyRoom", screen.RowText(lo.firstRow),
		fmt.Sprintf("room title must be redrawn at row %d (firstRow) after WriteConsole", lo.firstRow))
	assert.Equal(t, "A dark chamber.", screen.RowText(lo.firstRow+1),
		"room description must be redrawn after WriteConsole")
}

// ─── WritePromptSplit tests ───────────────────────────────────────────────────

func TestIntegration_WritePromptSplit_PlacesPromptAtRowH(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	go func() {
		_ = conn.WriteRoom("Nexus Hub")
		_ = conn.WritePromptSplit("[Newb]> ")
	}()
	feedScreen(t, screen, client, 500*time.Millisecond)

	assert.Contains(t, screen.RowFull(lo.promptRow), "[Newb]> ",
		"prompt must be placed at promptRow (row H)")
}

// ─── Full layout integration: InitScreen + WriteRoom + WriteConsole ───────────

func TestIntegration_FullLayout_InitThenWriteRoomThenConsole(t *testing.T) {
	const W, H = 80, 24
	lo := newRoomLayout(H)
	screen := newVT100Screen(W, H)
	conn, client := newSplitConnTB(t, W, H)

	roomContent := "Rotgut Alley\nA grungy street.\nExits:\n  south\n  east"
	go func() {
		_ = conn.InitScreen()
		conn.mu.Lock()
		conn.roomBuf = ""
		conn.mu.Unlock()
		_ = conn.WriteRoom(roomContent)
		conn.mu.Lock()
		conn.roomBuf = roomContent
		conn.mu.Unlock()
		_ = conn.WriteConsole("Ganger says: keep walking.")
		// In the real game, WritePromptSplit is called after every WriteConsole.
		_ = conn.WritePromptSplit("[Newb]> ")
	}()
	feedScreen(t, screen, client, 500*time.Millisecond)

	// Room content rows.
	assert.Equal(t, "Rotgut Alley", screen.RowText(lo.firstRow+0), "firstRow: room title")
	assert.Equal(t, "A grungy street.", screen.RowText(lo.firstRow+1), "firstRow+1: room description")
	assert.Equal(t, "Exits:", screen.RowText(lo.firstRow+2), "firstRow+2: exits header")
	assert.Equal(t, "  south", screen.RowText(lo.firstRow+3), "firstRow+3: south exit")
	assert.Equal(t, "  east", screen.RowText(lo.firstRow+4), "firstRow+4: east exit")
	assert.Empty(t, screen.RowText(lo.firstRow+5), "firstRow+5: blank")

	// Room divider.
	assert.Contains(t, screen.RowText(lo.dividerRow), "═",
		fmt.Sprintf("row %d: room divider", lo.dividerRow))

	// Console message in scroll region (rows 1..scrollBottom).
	found := false
	for r := 1; r <= lo.scrollBottom; r++ {
		if strings.Contains(screen.RowText(r), "Ganger says") {
			found = true
			break
		}
	}
	assert.True(t, found, "console message must appear in scroll region")

	// Prompt at row H.
	assert.Contains(t, screen.RowFull(lo.promptRow), "[Newb]> ", "promptRow: prompt")
}

// ─── Property-based tests ─────────────────────────────────────────────────────

// TestProperty_WriteRoom_NeverOverwritesScrollRegion verifies that WriteRoom
// never writes to the scroll region rows (1..scrollBottom) regardless of
// content size or terminal dimensions.
func TestProperty_WriteRoom_NeverOverwritesScrollRegion(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		W := rapid.IntRange(40, 200).Draw(rt, "width")
		H := rapid.IntRange(12, 50).Draw(rt, "height")
		lineCount := rapid.IntRange(0, 20).Draw(rt, "lines")
		lo := newRoomLayout(H)

		lines := make([]string, lineCount)
		for i := range lines {
			lines[i] = rapid.StringMatching(`[A-Za-z0-9 ]{1,30}`).Draw(rt, fmt.Sprintf("line%d", i))
		}

		screen := newVT100Screen(W, H)
		screen.scrollTop, screen.scrollBottom = 1, lo.scrollBottom

		// Pre-fill scroll region rows with sentinel so any overwrite is detectable.
		for r := 1; r <= lo.scrollBottom; r++ {
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

		// Scroll region rows must be untouched.
		for r := 1; r <= lo.scrollBottom; r++ {
			require.Contains(rt, screen.RowFull(r), ".",
				fmt.Sprintf("WriteRoom must not overwrite scroll region row %d (W=%d H=%d lines=%d)",
					r, W, H, lineCount))
		}

		// Room divider must be present.
		require.Contains(rt, screen.RowFull(lo.dividerRow), "═",
			fmt.Sprintf("WriteRoom must draw divider at row %d (W=%d H=%d lines=%d)",
				lo.dividerRow, W, H, lineCount))
	})
}

// TestProperty_WriteRoom_AlwaysShowsFirstNLinesInOrder verifies that the first
// min(len(lines), roomRegionRows) lines of content appear in room rows in order.
func TestProperty_WriteRoom_AlwaysShowsFirstNLinesInOrder(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		W := rapid.IntRange(40, 200).Draw(rt, "width")
		H := rapid.IntRange(12, 50).Draw(rt, "height")
		lineCount := rapid.IntRange(1, 15).Draw(rt, "lines")
		lo := newRoomLayout(H)

		lines := make([]string, lineCount)
		for i := range lines {
			lines[i] = fmt.Sprintf("L%03d", i+1)
		}

		screen := newVT100Screen(W, H)
		screen.scrollTop, screen.scrollBottom = 1, lo.scrollBottom

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
			row := lo.firstRow + i
			got := screen.RowText(row)
			require.Equal(rt, lines[i], got,
				fmt.Sprintf("room row %d must contain line %d (W=%d H=%d)", row, i+1, W, H))
		}
	})
}

// TestProperty_WriteConsole_NeverOverwritesRoomRegion verifies that WriteConsole
// does not corrupt room rows with console text (it redraws them via appendRoomRedraw
// with the cached room content).
func TestProperty_WriteConsole_NeverOverwritesRoomRegion(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		W := rapid.IntRange(40, 200).Draw(rt, "width")
		H := rapid.IntRange(12, 50).Draw(rt, "height")
		lo := newRoomLayout(H)

		roomContent := "RoomTitle\nRoom description.\nExits:\n  north"
		consoleMsg := rapid.StringMatching(`[A-Za-z0-9 !?,]{5,40}`).Draw(rt, "msg")

		screen := newVT100Screen(W, H)
		screen.scrollTop, screen.scrollBottom = 1, lo.scrollBottom

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

		// The console message must NOT appear in room rows.
		for r := lo.firstRow; r <= lo.lastRow; r++ {
			require.NotContains(rt, screen.RowText(r), consoleMsg,
				fmt.Sprintf("room row %d must not contain console text (W=%d H=%d)", r, W, H))
		}

		// Room title must be redrawn in firstRow.
		require.Equal(rt, "RoomTitle", screen.RowText(lo.firstRow),
			fmt.Sprintf("room title must remain at firstRow=%d after WriteConsole (W=%d H=%d)",
				lo.firstRow, W, H))
	})
}

// TestProperty_WriteConsole_MessageAlwaysInScrollRegion verifies that console
// messages always appear in the scroll region rows 1..scrollBottom.
func TestProperty_WriteConsole_MessageAlwaysInScrollRegion(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		W := rapid.IntRange(40, 200).Draw(rt, "width")
		H := rapid.IntRange(12, 50).Draw(rt, "height")
		lo := newRoomLayout(H)

		msg := "SentinelMsg"

		screen := newVT100Screen(W, H)
		screen.scrollTop, screen.scrollBottom = 1, lo.scrollBottom

		conn, client := newSplitConnTB(rt, W, H)

		go func() {
			_ = conn.WriteConsole(msg)
			conn.Close()
		}()
		raw := readAllConn(client, 2*time.Second)
		screen.Feed(raw)

		found := false
		for r := 1; r <= lo.scrollBottom; r++ {
			if strings.Contains(screen.RowText(r), msg) {
				found = true
				break
			}
		}
		require.True(rt, found,
			fmt.Sprintf("console message must appear in scroll region 1..%d (W=%d H=%d)",
				lo.scrollBottom, W, H))
	})
}

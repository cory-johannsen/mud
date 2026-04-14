package handlers

// REQ-70-1: RenderCharacterSheet MUST display the Crypto balance directly beneath the XP line.
// REQ-70-2: The Crypto line MUST use the format "Crypto: N Crypto".

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// TestRenderCharacterSheet_ShowsCryptoUnderXP verifies that the Crypto balance
// appears directly beneath the XP line in the character sheet.
//
// REQ-70-1: Crypto MUST appear directly beneath XP.
//
// Precondition: CharacterSheetView.Currency = "340 Crypto", XpToNext = 2000, Experience = 1240.
// Postcondition: output contains a Crypto line immediately after the XP line.
func TestRenderCharacterSheet_ShowsCryptoUnderXP(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:       "Alice",
		Level:      1,
		Currency:   "340 Crypto",
		Experience: 1240,
		XpToNext:   2000,
	}
	result := RenderCharacterSheet(csv, 80)
	stripped := telnet.StripANSI(result)

	assert.Contains(t, stripped, "340 Crypto", "character sheet must show the currency amount (REQ-70-1, REQ-70-2)")

	// Verify Crypto appears directly after the XP line.
	lines := strings.Split(stripped, "\n")
	xpIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "XP:") {
			xpIdx = i
			break
		}
	}
	require.True(t, xpIdx >= 0, "XP line must be present in character sheet")

	// Search the two lines immediately following XP for the currency line.
	found := false
	for _, offset := range []int{1, 2} {
		if xpIdx+offset < len(lines) && strings.Contains(lines[xpIdx+offset], "Crypto") {
			found = true
			break
		}
	}
	assert.True(t, found, "Crypto line must appear directly beneath XP line (REQ-70-1); XP at line %d, lines after: %v",
		xpIdx, lines[xpIdx+1:min70(xpIdx+4, len(lines))])
}

// TestRenderCharacterSheet_ShowsCryptoWhenZero verifies that zero Crypto is
// shown on the character sheet (not hidden).
//
// REQ-70-1: Crypto MUST appear even when balance is 0.
func TestRenderCharacterSheet_ShowsCryptoWhenZero(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:       "Bob",
		Level:      1,
		Currency:   "0 Crypto",
		Experience: 0,
		XpToNext:   1000,
	}
	result := RenderCharacterSheet(csv, 80)
	stripped := telnet.StripANSI(result)
	assert.Contains(t, stripped, "0 Crypto", "Crypto line must appear even when balance is 0 (REQ-70-1)")
}

// TestRenderCharacterSheet_NoCryptoWhenCurrencyEmpty verifies that when
// Currency field is empty the Crypto line is omitted.
func TestRenderCharacterSheet_NoCryptoWhenCurrencyEmpty(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:     "Eve",
		Level:    1,
		Currency: "",
	}
	result := RenderCharacterSheet(csv, 80)
	stripped := telnet.StripANSI(result)
	assert.NotContains(t, stripped, "Crypto", "no Crypto line when Currency field is empty")
}

// TestProperty_RenderCharacterSheet_CryptoAlwaysAfterXP is a property test
// verifying that for any non-empty Currency value, Crypto appears after XP.
//
// REQ-70-1 (property): ordering is invariant over experience values.
func TestProperty_RenderCharacterSheet_CryptoAlwaysAfterXP(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		xp := rapid.IntRange(0, 100000).Draw(rt, "xp")
		xpToNext := rapid.IntRange(0, 100000).Draw(rt, "xpToNext")
		crypto := rapid.IntRange(0, 999999).Draw(rt, "crypto")

		csv := &gamev1.CharacterSheetView{
			Name:       "Test",
			Level:      1,
			Currency:   strings.Join([]string{itoa70(crypto), " Crypto"}, ""),
			Experience: int32(xp),
			XpToNext:   int32(xpToNext),
		}
		result := telnet.StripANSI(RenderCharacterSheet(csv, 80))
		lines := strings.Split(result, "\n")

		xpIdx := -1
		for i, line := range lines {
			if strings.Contains(line, "XP:") {
				xpIdx = i
				break
			}
		}
		if xpIdx < 0 {
			rt.Fatal("XP line not found in character sheet")
		}

		found := false
		for _, offset := range []int{1, 2} {
			if xpIdx+offset < len(lines) && strings.Contains(lines[xpIdx+offset], "Crypto") {
				found = true
				break
			}
		}
		if !found {
			rt.Fatalf("Crypto not found directly after XP (xpIdx=%d); lines=%v", xpIdx, lines[xpIdx:min70(xpIdx+5, len(lines))])
		}
	})
}

func min70(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func itoa70(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

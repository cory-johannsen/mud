package telnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestColorize(t *testing.T) {
	result := Colorize(Red, "danger")
	assert.Equal(t, "\033[31mdanger\033[0m", result)
}

func TestColorf(t *testing.T) {
	result := Colorf(Green, "health: %d", 42)
	assert.Equal(t, "\033[32mhealth: 42\033[0m", result)
}

func TestStripANSI(t *testing.T) {
	input := "\033[31mred\033[0m normal \033[1m\033[32mbold green\033[0m"
	result := StripANSI(input)
	assert.Equal(t, "red normal bold green", result)
}

func TestStripANSI_NoEscapes(t *testing.T) {
	input := "plain text"
	assert.Equal(t, input, StripANSI(input))
}

func TestStripANSI_EmptyString(t *testing.T) {
	assert.Equal(t, "", StripANSI(""))
}

// Property: StripANSI(Colorize(color, text)) == text for any ASCII text.
func TestPropertyStripANSIInversesColorize(t *testing.T) {
	colors := []string{Red, Green, Blue, Yellow, Cyan, Magenta, White, Bold, Dim}
	rapid.Check(t, func(t *rapid.T) {
		text := rapid.StringMatching(`[a-zA-Z0-9 ]{0,50}`).Draw(t, "text")
		colorIdx := rapid.IntRange(0, len(colors)-1).Draw(t, "color")
		colored := Colorize(colors[colorIdx], text)
		stripped := StripANSI(colored)
		assert.Equal(t, text, stripped, "stripping ANSI from colorized text should yield original")
	})
}

// Property: StripANSI output never contains ESC character.
func TestPropertyStripANSINoEscapeInOutput(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		text := rapid.StringMatching(`[a-zA-Z0-9 ]{0,30}`).Draw(t, "text")
		styled := Bold + Red + text + Reset
		stripped := StripANSI(styled)
		for _, c := range stripped {
			assert.NotEqual(t, '\033', c, "output should not contain ESC character")
		}
	})
}

// Property: StripANSI output length <= input length.
func TestPropertyStripANSIOutputShorterOrEqual(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		text := rapid.String().Draw(t, "text")
		result := StripANSI(text)
		assert.LessOrEqual(t, len(result), len(text))
	})
}

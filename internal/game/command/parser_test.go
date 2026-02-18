package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestParse_Empty(t *testing.T) {
	result := Parse("")
	assert.Equal(t, "", result.Command)
	assert.Nil(t, result.Args)
}

func TestParse_SingleWord(t *testing.T) {
	result := Parse("look")
	assert.Equal(t, "look", result.Command)
	assert.Nil(t, result.Args)
	assert.Equal(t, "", result.RawArgs)
}

func TestParse_Lowercase(t *testing.T) {
	result := Parse("NORTH")
	assert.Equal(t, "north", result.Command)
}

func TestParse_WithArgs(t *testing.T) {
	result := Parse("say hello world")
	assert.Equal(t, "say", result.Command)
	assert.Equal(t, []string{"hello", "world"}, result.Args)
	assert.Equal(t, "hello world", result.RawArgs)
}

func TestParse_ExtraWhitespace(t *testing.T) {
	result := Parse("  say   hello   world  ")
	assert.Equal(t, "say", result.Command)
	assert.Equal(t, []string{"hello", "world"}, result.Args)
	assert.Equal(t, "hello   world", result.RawArgs)
}

func TestParse_DirectionAlias(t *testing.T) {
	result := Parse("n")
	assert.Equal(t, "n", result.Command)
}

func TestParse_EmoteWithAction(t *testing.T) {
	result := Parse("emote waves enthusiastically")
	assert.Equal(t, "emote", result.Command)
	assert.Equal(t, []string{"waves", "enthusiastically"}, result.Args)
	assert.Equal(t, "waves enthusiastically", result.RawArgs)
}

func TestPropertyParseAlwaysLowercasesCommand(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		word := rapid.StringMatching(`[A-Za-z]{1,20}`).Draw(t, "word")
		result := Parse(word)
		for _, c := range result.Command {
			if c >= 'A' && c <= 'Z' {
				t.Fatalf("command %q contains uppercase char in Parse result %q", word, result.Command)
			}
		}
	})
}

func TestPropertyParseNonEmptyInputHasCommand(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		word := rapid.StringMatching(`[a-z]{1,10}`).Draw(t, "word")
		result := Parse(word)
		if result.Command == "" {
			t.Fatalf("non-empty input %q produced empty command", word)
		}
	})
}

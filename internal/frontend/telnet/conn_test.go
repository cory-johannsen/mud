package telnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestFilterIAC_NoIAC(t *testing.T) {
	input := []byte("hello world")
	result := FilterIAC(input)
	assert.Equal(t, input, result)
}

func TestFilterIAC_WillCommand(t *testing.T) {
	input := []byte{IAC, WILL, OptEcho, 'h', 'i'}
	result := FilterIAC(input)
	assert.Equal(t, []byte("hi"), result)
}

func TestFilterIAC_WontCommand(t *testing.T) {
	input := []byte{IAC, WONT, OptSuppressGoAhead, 'o', 'k'}
	result := FilterIAC(input)
	assert.Equal(t, []byte("ok"), result)
}

func TestFilterIAC_DoCommand(t *testing.T) {
	input := []byte{'a', IAC, DO, OptLinemode, 'b'}
	result := FilterIAC(input)
	assert.Equal(t, []byte("ab"), result)
}

func TestFilterIAC_DontCommand(t *testing.T) {
	input := []byte{IAC, DONT, OptEcho}
	result := FilterIAC(input)
	assert.Empty(t, result)
}

func TestFilterIAC_SubNegotiation(t *testing.T) {
	input := []byte{IAC, SB, 24, 0, 'x', 't', 'e', 'r', 'm', IAC, SE, 'z'}
	result := FilterIAC(input)
	assert.Equal(t, []byte("z"), result)
}

func TestFilterIAC_EscapedIAC(t *testing.T) {
	input := []byte{'a', IAC, IAC, 'b'}
	result := FilterIAC(input)
	assert.Equal(t, []byte{byte('a'), IAC, byte('b')}, result)
}

func TestFilterIAC_NOP(t *testing.T) {
	input := []byte{'x', IAC, NOP, 'y'}
	result := FilterIAC(input)
	assert.Equal(t, []byte("xy"), result)
}

func TestFilterIAC_MultipleCommands(t *testing.T) {
	input := []byte{
		IAC, WILL, OptSuppressGoAhead,
		IAC, WILL, OptEcho,
		'h', 'e', 'l', 'l', 'o',
	}
	result := FilterIAC(input)
	assert.Equal(t, []byte("hello"), result)
}

// Property: FilterIAC on input without any IAC bytes returns the input unchanged.
func TestPropertyFilterIAC_NoIACBytesPassThrough(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate bytes that don't contain IAC (0xFF)
		length := rapid.IntRange(0, 200).Draw(t, "length")
		input := make([]byte, length)
		for i := range input {
			input[i] = byte(rapid.IntRange(0, 254).Draw(t, "byte"))
		}
		result := FilterIAC(input)
		assert.Equal(t, input, result, "input without IAC bytes should pass through unchanged")
	})
}

// Property: FilterIAC output never contains unescaped IAC command sequences.
func TestPropertyFilterIAC_OutputHasNoIACCommands(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		length := rapid.IntRange(0, 100).Draw(t, "length")
		input := make([]byte, length)
		for i := range input {
			input[i] = byte(rapid.IntRange(0, 255).Draw(t, "byte"))
		}
		result := FilterIAC(input)
		// Check that result contains no IAC followed by a command byte
		for i := 0; i < len(result)-1; i++ {
			if result[i] == IAC {
				next := result[i+1]
				// After filtering, only escaped IAC (IAC IAC -> IAC) should remain
				assert.Equal(t, IAC, next,
					"IAC in output should only appear as escaped IAC (0xFF 0xFF -> 0xFF)")
			}
		}
	})
}

// Property: FilterIAC output length is always <= input length.
func TestPropertyFilterIAC_OutputNeverLongerThanInput(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		length := rapid.IntRange(0, 200).Draw(t, "length")
		input := make([]byte, length)
		for i := range input {
			input[i] = byte(rapid.IntRange(0, 255).Draw(t, "byte"))
		}
		result := FilterIAC(input)
		assert.LessOrEqual(t, len(result), len(input),
			"filtered output should never be longer than input")
	})
}

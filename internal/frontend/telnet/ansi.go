// Package telnet provides a Telnet server with ANSI color support for the MUD.
package telnet

import "fmt"

// ANSI escape code constants for terminal styling.
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Underline = "\033[4m"

	// Foreground colors
	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	// Bright foreground colors
	BrightBlack   = "\033[90m"
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"

	// Background colors
	BgBlack   = "\033[40m"
	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
	BgWhite   = "\033[47m"
)

// Colorize wraps text with the given ANSI color code and a reset suffix.
//
// Precondition: color must be a valid ANSI escape sequence.
// Postcondition: Returns text wrapped with the color code and Reset.
func Colorize(color, text string) string {
	return color + text + Reset
}

// Colorf wraps a formatted string with the given ANSI color code.
//
// Precondition: color must be a valid ANSI escape sequence.
// Postcondition: Returns the formatted text wrapped with color and Reset.
func Colorf(color, format string, args ...interface{}) string {
	return color + fmt.Sprintf(format, args...) + Reset
}

// StripANSI removes all ANSI escape sequences from a string.
// This is useful for measuring the printable width of styled text.
//
// Postcondition: Returns text with all \033[...m sequences removed.
func StripANSI(s string) string {
	result := make([]byte, 0, len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// Skip past the 'm' terminator
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				i = j + 1
				continue
			}
		}
		result = append(result, s[i])
		i++
	}
	return string(result)
}

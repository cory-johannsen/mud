package command

import "strings"

// ParseResult holds the parsed command name and arguments from a text line.
type ParseResult struct {
	// Command is the first word of the input, lowercased.
	Command string
	// Args are the remaining words after the command.
	Args []string
	// RawArgs is the raw text after the command (preserving spacing for say/emote).
	RawArgs string
}

// Parse splits a text line into a command and arguments.
//
// Precondition: line should be trimmed of leading/trailing whitespace.
// Postcondition: Returns a ParseResult. If line is empty, Command is empty.
func Parse(line string) ParseResult {
	line = strings.TrimSpace(line)
	if line == "" {
		return ParseResult{}
	}

	// Split at first space for the command word
	spaceIdx := strings.IndexByte(line, ' ')
	if spaceIdx < 0 {
		return ParseResult{
			Command: strings.ToLower(line),
		}
	}

	cmd := strings.ToLower(line[:spaceIdx])
	rest := line[spaceIdx+1:]
	rest = strings.TrimSpace(rest)

	var args []string
	if rest != "" {
		args = strings.Fields(rest)
	}

	return ParseResult{
		Command: cmd,
		Args:    args,
		RawArgs: rest,
	}
}

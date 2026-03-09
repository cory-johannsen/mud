package command

// EscapeRequest is the parsed form of the escape command.
//
// Precondition: none.
type EscapeRequest struct{}

// HandleEscape parses the arguments for the "escape" command.
// Arguments are ignored — escape takes no parameters.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *EscapeRequest and nil error always.
func HandleEscape(args []string) (*EscapeRequest, error) {
	return &EscapeRequest{}, nil
}

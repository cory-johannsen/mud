package command

// DivertRequest is the parsed form of the divert command.
//
// Precondition: none.
type DivertRequest struct{}

// HandleDivert parses the arguments for the "divert" command.
// Arguments are ignored — divert takes no parameters.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *DivertRequest and nil error always.
func HandleDivert(args []string) (*DivertRequest, error) {
	return &DivertRequest{}, nil
}

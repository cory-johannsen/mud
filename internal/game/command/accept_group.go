package command

// AcceptGroupRequest is the parsed form of the accept command.
type AcceptGroupRequest struct{}

// HandleAcceptGroup parses the accept command. Arguments are ignored.
//
// Postcondition: Always returns a non-nil *AcceptGroupRequest and nil error.
func HandleAcceptGroup(_ []string) (*AcceptGroupRequest, error) {
	return &AcceptGroupRequest{}, nil
}

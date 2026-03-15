package command

// DeclineGroupRequest is the parsed form of the gdecline command.
type DeclineGroupRequest struct{}

// HandleDeclineGroup parses the gdecline command. Arguments are ignored.
//
// Postcondition: Always returns a non-nil *DeclineGroupRequest and nil error.
func HandleDeclineGroup(_ []string) (*DeclineGroupRequest, error) {
	return &DeclineGroupRequest{}, nil
}

package command

// DeclineRequest is the parsed form of the decline command.
type DeclineRequest struct{}

// HandleDecline parses the decline command. Arguments are ignored.
//
// Postcondition: Always returns a non-nil *DeclineRequest and nil error.
func HandleDecline(_ []string) (*DeclineRequest, error) {
	return &DeclineRequest{}, nil
}

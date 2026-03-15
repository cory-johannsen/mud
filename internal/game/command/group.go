package command

// GroupRequest is the parsed form of the group command.
type GroupRequest struct {
	Args []string
}

// HandleGroup parses the group command.
//
// Postcondition: Always returns a non-nil *GroupRequest and nil error.
func HandleGroup(args []string) (*GroupRequest, error) {
	return &GroupRequest{Args: args}, nil
}

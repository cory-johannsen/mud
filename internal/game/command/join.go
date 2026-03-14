package command

// JoinRequest is the parsed form of the join command.
type JoinRequest struct{}

// HandleJoin parses the join command. Arguments are ignored.
//
// Postcondition: Always returns a non-nil *JoinRequest and nil error.
func HandleJoin(_ []string) (*JoinRequest, error) {
	return &JoinRequest{}, nil
}

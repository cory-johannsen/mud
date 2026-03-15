package command

// InviteRequest is the parsed form of the invite command.
type InviteRequest struct {
	Player string
}

// HandleInvite parses the invite command.
//
// Postcondition: Always returns a non-nil *InviteRequest and nil error.
// Player is the first argument if provided, otherwise empty string.
func HandleInvite(args []string) (*InviteRequest, error) {
	player := ""
	if len(args) > 0 {
		player = args[0]
	}
	return &InviteRequest{Player: player}, nil
}

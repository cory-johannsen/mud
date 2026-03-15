package command

// KickRequest is the parsed form of the kick command.
type KickRequest struct {
	Player string
}

// HandleKick parses the kick command.
//
// Postcondition: Always returns a non-nil *KickRequest and nil error.
// Player is the first argument if provided, otherwise empty string.
func HandleKick(args []string) (*KickRequest, error) {
	player := ""
	if len(args) > 0 {
		player = args[0]
	}
	return &KickRequest{Player: player}, nil
}

package command

// TumbleRequest is the parsed form of the tumble command.
//
// Precondition: Target may be empty (handler will return error in that case).
type TumbleRequest struct {
	Target string
}

// HandleTumble parses the arguments for the "tumble" command.
//
// Precondition: args is the slice of words following "tumble" (may be empty).
// Postcondition: Returns a non-nil *TumbleRequest and nil error always.
func HandleTumble(args []string) (*TumbleRequest, error) {
	req := &TumbleRequest{}
	if len(args) >= 1 {
		req.Target = args[0]
	}
	return req, nil
}

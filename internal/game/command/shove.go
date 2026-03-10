package command

// ShoveRequest is the parsed form of the shove command.
//
// Precondition: Target may be empty (handler will return error in that case).
type ShoveRequest struct {
	Target string
}

// HandleShove parses the arguments for the "shove" command.
//
// Precondition: args is the slice of words following "shove" (may be empty).
// Postcondition: Returns a non-nil *ShoveRequest and nil error always.
func HandleShove(args []string) (*ShoveRequest, error) {
	req := &ShoveRequest{}
	if len(args) >= 1 {
		req.Target = args[0]
	}
	return req, nil
}

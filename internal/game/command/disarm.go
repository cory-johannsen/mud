package command

// DisarmRequest is the parsed form of the disarm command.
//
// Precondition: Target may be empty (handler will return error in that case).
type DisarmRequest struct {
	Target string
}

// HandleDisarm parses the arguments for the "disarm" command.
//
// Precondition: args is the slice of words following "disarm" (may be empty).
// Postcondition: Returns a non-nil *DisarmRequest and nil error always.
func HandleDisarm(args []string) (*DisarmRequest, error) {
	req := &DisarmRequest{}
	if len(args) >= 1 {
		req.Target = args[0]
	}
	return req, nil
}

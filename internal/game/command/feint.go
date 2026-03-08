package command

// FeintRequest is the parsed form of the feint command.
//
// Precondition: Target may be empty (handler will return error in that case).
type FeintRequest struct {
	Target string
}

// HandleFeint parses the arguments for the "feint" command.
//
// Precondition: args is the slice of words following "feint" (may be empty).
// Postcondition: Returns a non-nil *FeintRequest and nil error always.
func HandleFeint(args []string) (*FeintRequest, error) {
	req := &FeintRequest{}
	if len(args) >= 1 {
		req.Target = args[0]
	}
	return req, nil
}

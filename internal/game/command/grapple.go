package command

// GrappleRequest is the parsed form of the grapple command.
//
// Precondition: Target may be empty (handler will return error in that case).
type GrappleRequest struct {
	Target string
}

// HandleGrapple parses the arguments for the "grapple" command.
//
// Precondition: args is the slice of words following "grapple" (may be empty).
// Postcondition: Returns a non-nil *GrappleRequest and nil error always.
func HandleGrapple(args []string) (*GrappleRequest, error) {
	req := &GrappleRequest{}
	if len(args) >= 1 {
		req.Target = args[0]
	}
	return req, nil
}

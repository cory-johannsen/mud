package command

// DemoralizeRequest is the parsed form of the demoralize command.
//
// Precondition: Target may be empty (handler will return error in that case).
type DemoralizeRequest struct {
	Target string
}

// HandleDemoralize parses the arguments for the "demoralize" command.
//
// Precondition: args is the slice of words following "demoralize" (may be empty).
// Postcondition: Returns a non-nil *DemoralizeRequest and nil error always.
func HandleDemoralize(args []string) (*DemoralizeRequest, error) {
	req := &DemoralizeRequest{}
	if len(args) >= 1 {
		req.Target = args[0]
	}
	return req, nil
}

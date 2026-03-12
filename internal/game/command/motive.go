package command

// MotiveRequest is the parsed form of the motive command.
//
// Precondition: Target may be empty (handler will return error in that case).
type MotiveRequest struct {
	Target string
}

// HandleMotive parses the arguments for the "motive" command.
//
// Precondition: args is the slice of words following "motive" (may be empty).
// Postcondition: Returns a non-nil *MotiveRequest and nil error always.
func HandleMotive(args []string) (*MotiveRequest, error) {
	req := &MotiveRequest{}
	if len(args) >= 1 {
		req.Target = args[0]
	}
	return req, nil
}

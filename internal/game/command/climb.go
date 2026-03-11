package command

// ClimbRequest is the parsed form of the climb command.
// Precondition: none — no arguments are required.
// Postcondition: Always returns a non-nil *ClimbRequest with nil error.
type ClimbRequest struct{}

// HandleClimb parses the arguments for the "climb" command.
// Precondition: args is the slice of words following "climb" (may be empty).
// Postcondition: Returns a non-nil *ClimbRequest and nil error always.
func HandleClimb(args []string) (*ClimbRequest, error) {
	return &ClimbRequest{}, nil
}

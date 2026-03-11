package command

// SwimRequest is the parsed form of the swim command.
// Precondition: none — no arguments are required.
// Postcondition: Always returns a non-nil *SwimRequest with nil error.
type SwimRequest struct{}

// HandleSwim parses the arguments for the "swim" command.
// Precondition: args is the slice of words following "swim" (may be empty).
// Postcondition: Returns a non-nil *SwimRequest and nil error always.
func HandleSwim(args []string) (*SwimRequest, error) {
	return &SwimRequest{}, nil
}

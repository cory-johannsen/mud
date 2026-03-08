package command

// FirstAidRequest is the parsed form of the aid command.
//
// Precondition: none.
type FirstAidRequest struct{}

// HandleFirstAid parses the arguments for the "aid" command.
// Arguments are ignored — aid takes no parameters.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *FirstAidRequest and nil error always.
func HandleFirstAid(args []string) (*FirstAidRequest, error) {
	return &FirstAidRequest{}, nil
}

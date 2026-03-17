package command

// RestRequest is the parsed form of the rest command.
//
// Precondition: none.
type RestRequest struct{}

// HandleRest parses the arguments for the "rest" command.
// Arguments are ignored — rest takes no parameters.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *RestRequest and nil error always.
func HandleRest(args []string) (*RestRequest, error) {
	return &RestRequest{}, nil
}

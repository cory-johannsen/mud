package command

// SelectTechRequest is the parsed form of the selecttech command.
//
// Precondition: none.
type SelectTechRequest struct{}

// HandleSelectTech parses the selecttech command. Arguments are ignored.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *SelectTechRequest and nil error always.
func HandleSelectTech(_ []string) (*SelectTechRequest, error) {
	return &SelectTechRequest{}, nil
}

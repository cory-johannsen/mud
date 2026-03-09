package command

// HideRequest is the parsed form of the hide command.
//
// Precondition: none.
type HideRequest struct{}

// HandleHide parses the arguments for the "hide" command.
// Arguments are ignored — hide takes no parameters.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *HideRequest and nil error always.
func HandleHide(args []string) (*HideRequest, error) {
	return &HideRequest{}, nil
}

package command

// RaiseShieldRequest is the parsed form of the raise command.
//
// Precondition: none.
type RaiseShieldRequest struct{}

// HandleRaiseShield parses the arguments for the "raise" command.
// Arguments are ignored — raise takes no parameters.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *RaiseShieldRequest and nil error always.
func HandleRaiseShield(args []string) (*RaiseShieldRequest, error) {
	return &RaiseShieldRequest{}, nil
}

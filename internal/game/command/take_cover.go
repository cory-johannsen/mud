package command

// TakeCoverRequest is the parsed form of the cover command.
//
// Precondition: none.
type TakeCoverRequest struct{}

// HandleTakeCover parses the arguments for the "cover" command.
// Arguments are ignored — cover takes no parameters.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *TakeCoverRequest and nil error always.
func HandleTakeCover(args []string) (*TakeCoverRequest, error) {
	return &TakeCoverRequest{}, nil
}

// UncoverRequest is the parsed form of the uncover command.
//
// Precondition: none.
type UncoverRequest struct{}

// HandleUncover parses the arguments for the "uncover" command.
// Arguments are ignored — uncover takes no parameters.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *UncoverRequest and nil error always.
func HandleUncover(args []string) (*UncoverRequest, error) {
	return &UncoverRequest{}, nil
}

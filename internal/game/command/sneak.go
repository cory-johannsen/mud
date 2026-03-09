package command

// SneakRequest is the parsed form of the sneak command.
//
// Precondition: none.
type SneakRequest struct{}

// HandleSneak parses the arguments for the "sneak" command.
// Arguments are ignored — sneak takes no parameters.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *SneakRequest and nil error always.
func HandleSneak(args []string) (*SneakRequest, error) {
	return &SneakRequest{}, nil
}

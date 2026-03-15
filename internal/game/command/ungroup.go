package command

// UngroupRequest is the parsed form of the ungroup command.
type UngroupRequest struct{}

// HandleUngroup parses the ungroup command. Arguments are ignored.
//
// Postcondition: Always returns a non-nil *UngroupRequest and nil error.
func HandleUngroup(_ []string) (*UngroupRequest, error) {
	return &UngroupRequest{}, nil
}

package command

// StrideRequest is the parsed form of the stride command.
//
// Precondition: Direction is always "toward" or "away".
type StrideRequest struct {
	Direction string // "toward" or "away"
}

// HandleStride parses the direction argument for the "stride" command.
//
// Precondition: args is the slice of words following "stride" (may be empty).
// Postcondition: Returns a non-nil *StrideRequest with Direction "toward" or "away".
func HandleStride(args []string) (*StrideRequest, error) {
	dir := "toward"
	if len(args) >= 1 && args[0] == "away" {
		dir = "away"
	}
	return &StrideRequest{Direction: dir}, nil
}

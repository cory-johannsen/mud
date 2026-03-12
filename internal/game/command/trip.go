package command

// TripRequest is the parsed form of the trip command.
//
// Precondition: Target may be empty (handler will return error in that case).
type TripRequest struct {
	Target string
}

// HandleTrip parses the arguments for the "trip" command.
//
// Precondition: args is the slice of words following "trip" (may be empty).
// Postcondition: Returns a non-nil *TripRequest and nil error always.
func HandleTrip(args []string) (*TripRequest, error) {
	req := &TripRequest{}
	if len(args) >= 1 {
		req.Target = args[0]
	}
	return req, nil
}

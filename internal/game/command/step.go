package command

// StepRequest is the parsed form of the step command.
//
// Precondition: Direction is always "toward" or "away".
type StepRequest struct {
	Direction string // "toward" or "away"
}

// HandleStep parses the direction argument for the "step" command.
//
// Precondition: args is the slice of words following "step" (may be empty).
// Postcondition: Returns a non-nil *StepRequest with Direction "toward" or "away".
func HandleStep(args []string) (*StepRequest, error) {
	dir := "toward"
	if len(args) >= 1 && args[0] == "away" {
		dir = "away"
	}
	return &StepRequest{Direction: dir}, nil
}

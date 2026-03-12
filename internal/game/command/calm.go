package command

// CalmRequest is the parsed form of the calm command.
// calm takes no arguments — it always targets the player's own worst active mental state.
type CalmRequest struct{}

// HandleCalm parses the calm command. Arguments are ignored.
//
// Postcondition: Always returns a non-nil *CalmRequest and nil error.
func HandleCalm(_ []string) (*CalmRequest, error) {
	return &CalmRequest{}, nil
}

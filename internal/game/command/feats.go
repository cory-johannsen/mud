package command

// HandleFeats returns the feats command client acknowledgement.
// The actual feat data is returned by the server in a FeatsResponse.
//
// Postcondition: Returns a non-empty string.
func HandleFeats() string {
	return "Reviewing your feats..."
}

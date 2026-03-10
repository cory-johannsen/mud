package command

// SeekRequest is the parsed form of the seek command.
//
// Precondition: none.
type SeekRequest struct{}

// HandleSeek parses the seek command. No arguments are required.
//
// Precondition: args may be empty or non-empty; all arguments are ignored.
// Postcondition: Returns a non-nil SeekRequest and nil error.
func HandleSeek(_ []string) (*SeekRequest, error) {
	return &SeekRequest{}, nil
}

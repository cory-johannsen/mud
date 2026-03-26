package command

import "strings"

// ExploreRequest is the parsed form of the explore command.
//
// Precondition: none.
// Postcondition: Mode is lower-cased; ShadowTarget is the raw second token when Mode == "shadow".
type ExploreRequest struct {
	// Mode is the requested exploration mode ID, "off" to clear, or "" to query.
	Mode string
	// ShadowTarget is the ally player name when Mode == "shadow". Empty otherwise.
	ShadowTarget string
}

// HandleExplore parses the arguments for the "explore" command.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns a non-nil *ExploreRequest; Mode is lower-cased.
func HandleExplore(args []string) (*ExploreRequest, error) {
	if len(args) == 0 {
		return &ExploreRequest{}, nil
	}
	mode := strings.ToLower(args[0])
	req := &ExploreRequest{Mode: mode}
	if mode == "shadow" && len(args) >= 2 {
		req.ShadowTarget = args[1]
	}
	return req, nil
}

package command

// ActionRequest is the parsed form of the action command.
//
// Precondition: Name may be empty (list mode); Target is empty when not required.
type ActionRequest struct {
	Name   string // action ID or shortcut; empty means list available actions
	Target string // target NPC name; empty if not required
}

// HandleAction parses the arguments for the "action" command.
//
// Precondition: args is the slice of words following "action" (may be empty).
// Postcondition: Returns a non-nil *ActionRequest and nil error in all valid cases.
func HandleAction(args []string) (*ActionRequest, error) {
	req := &ActionRequest{}
	if len(args) >= 1 {
		req.Name = args[0]
	}
	if len(args) >= 2 {
		req.Target = args[1]
	}
	return req, nil
}

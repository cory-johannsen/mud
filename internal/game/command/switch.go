package command

// HandleSwitch handles the switch command.
// Switching is a frontend-driven operation; the server receives a SwitchCharacterRequest.
// This function exists to satisfy the command registry; it returns an empty string.
//
// Precondition: args may be nil or empty.
// Postcondition: Returns an empty string.
func HandleSwitch(args []string) string {
	return ""
}

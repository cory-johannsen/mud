package command

import "fmt"

// HandleUseEquipment returns a plain-text acknowledgment for the use command.
//
// Precondition: instanceID may be any string.
// Postcondition: Returns a non-empty human-readable string.
func HandleUseEquipment(instanceID string) string {
	if instanceID == "" {
		return "Use what? Specify an item instance ID."
	}
	return fmt.Sprintf("You use %s.", instanceID)
}

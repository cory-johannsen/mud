package command

import "fmt"

// HandleArchetypeSelection returns a plain-text acknowledgment of an archetype selection.
// Used as a fallback for telnet/dev-server clients; gRPC clients receive a structured response.
//
// Precondition: archetypeID may be any string (non-empty is expected during character creation).
// Postcondition: Returns a non-empty human-readable string.
func HandleArchetypeSelection(archetypeID string) string {
	return fmt.Sprintf("Archetype selected: %s", archetypeID)
}

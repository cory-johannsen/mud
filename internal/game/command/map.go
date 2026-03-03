package command

// HandleMap returns the map command client acknowledgement string.
// The actual map data is returned by the server in a MapResponse.
//
// Postcondition: Returns a non-empty string.
func HandleMap() string {
	return "Consulting your map..."
}

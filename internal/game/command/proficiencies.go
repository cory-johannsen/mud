package command

// HandleProficiencies is a client-side no-op; the bridge sends a ProficienciesRequest
// and the server returns a ProficienciesResponse rendered by the frontend.
//
// Postcondition: Returns an empty string.
func HandleProficiencies() string { return "" }

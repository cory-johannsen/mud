package handlers

// noOpSessionManager is an in-process stub for when no live session registry is wired.
// Postcondition: AllSessions always returns empty; GetSession always returns not found.
type noOpSessionManager struct{}

// NewNoOpSessionManager returns a SessionManager that reports no live sessions.
func NewNoOpSessionManager() SessionManager { return &noOpSessionManager{} }

func (n *noOpSessionManager) AllSessions() ([]ManagedSession, error) { return nil, nil }
func (n *noOpSessionManager) GetSession(_ int64) (ManagedSession, bool) { return nil, false }
func (n *noOpSessionManager) TeleportPlayer(_ int64, _ string) error    { return nil }

// noOpWorldEditor is an in-process stub for when no world manager is wired.
type noOpWorldEditor struct{}

// NewNoOpWorldEditor returns a WorldEditor that reports no zones, rooms, or NPC templates.
func NewNoOpWorldEditor() WorldEditor { return &noOpWorldEditor{} }

func (n *noOpWorldEditor) AllZones() []ZoneSummary { return []ZoneSummary{} }
func (n *noOpWorldEditor) RoomsInZone(zoneID string) ([]RoomSummary, error) {
	return nil, errZoneNotFound(zoneID)
}
func (n *noOpWorldEditor) UpdateRoom(_ string, _ RoomPatch) error { return nil }
func (n *noOpWorldEditor) AllNPCTemplates() []NPCTemplate                              { return []NPCTemplate{} }
func (n *noOpWorldEditor) SpawnNPC(_ string, _ string, _ int) (int, error) { return 0, nil }

// errZoneNotFound returns a sentinel error for a missing zone ID.
func errZoneNotFound(id string) error {
	return &zoneNotFoundError{id: id}
}

type zoneNotFoundError struct{ id string }

func (e *zoneNotFoundError) Error() string { return "zone not found: " + e.id }

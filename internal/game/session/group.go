package session

// Group represents a player party. Groups are session-only and not persisted to the database.
// All access is mediated through Manager methods which hold mu.
type Group struct {
	ID         string   // UUID assigned at creation
	LeaderUID  string
	MemberUIDs []string // All members including the leader; no duplicates; max 8
}

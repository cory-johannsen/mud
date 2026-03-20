package reaction

// PlayerReaction associates a registered reaction with the player who owns it.
type PlayerReaction struct {
	// UID is the player's session UID.
	UID string
	// Feat is the feat or tech ID that declared this reaction (matches spec REQ-RXN12).
	Feat string
	// FeatName is the human-readable display name of the feat/tech (e.g. "Chrome Reflex").
	// Used in the prompt message per REQ-RXN23.
	FeatName string
	// Def is the full reaction declaration.
	Def ReactionDef
}

// ReactionRegistry maps trigger types to registered player reactions.
//
// Precondition: created via NewReactionRegistry.
// Postcondition: Get returns the first registered reaction for (uid, trigger) or nil.
type ReactionRegistry struct {
	byTrigger map[ReactionTriggerType][]PlayerReaction
}

// NewReactionRegistry returns an empty ReactionRegistry.
func NewReactionRegistry() *ReactionRegistry {
	return &ReactionRegistry{
		byTrigger: make(map[ReactionTriggerType][]PlayerReaction),
	}
}

// Register adds a reaction for the given player and feat.
// featName is the human-readable display name used in prompts.
func (r *ReactionRegistry) Register(uid, featID, featName string, def ReactionDef) {
	r.byTrigger[def.Trigger] = append(r.byTrigger[def.Trigger], PlayerReaction{
		UID:      uid,
		Feat:     featID,
		FeatName: featName,
		Def:      def,
	})
}

// Get returns the first registered reaction for uid and trigger, or nil if none.
func (r *ReactionRegistry) Get(uid string, trigger ReactionTriggerType) *PlayerReaction {
	for i := range r.byTrigger[trigger] {
		pr := &r.byTrigger[trigger][i]
		if pr.UID == uid {
			return pr
		}
	}
	return nil
}

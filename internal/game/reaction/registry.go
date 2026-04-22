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
// One PlayerReaction entry is created per trigger in def.Triggers.
// If an entry with the same UID and trigger already exists, it is updated in-place.
// If def.Triggers is empty, Register is a no-op (REQ-CRX2).
func (r *ReactionRegistry) Register(uid, featID, featName string, def ReactionDef) {
	for _, trigger := range def.Triggers {
		entry := PlayerReaction{
			UID:      uid,
			Feat:     featID,
			FeatName: featName,
			Def:      def,
		}
		found := false
		for i := range r.byTrigger[trigger] {
			if r.byTrigger[trigger][i].UID == uid {
				r.byTrigger[trigger][i] = entry
				found = true
				break
			}
		}
		if !found {
			r.byTrigger[trigger] = append(r.byTrigger[trigger], entry)
		}
	}
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

// Filter returns all PlayerReactions registered for uid and trigger whose
// requirement is satisfied by requirementChecker.
// requirementChecker receives the Requirement string and returns true when met.
// A nil requirementChecker accepts all requirements.
// Returns an empty (non-nil) slice when no matching reactions are found.
func (r *ReactionRegistry) Filter(
	uid string,
	trigger ReactionTriggerType,
	requirementChecker func(req string) bool,
) []PlayerReaction {
	var result []PlayerReaction
	for _, pr := range r.byTrigger[trigger] {
		if pr.UID != uid {
			continue
		}
		if requirementChecker != nil && pr.Def.Requirement != "" {
			if !requirementChecker(pr.Def.Requirement) {
				continue
			}
		}
		result = append(result, pr)
	}
	if result == nil {
		result = []PlayerReaction{}
	}
	return result
}

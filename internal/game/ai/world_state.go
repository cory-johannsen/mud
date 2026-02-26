package ai

// CombatantState captures an entity's combat-relevant state at planning time.
//
// Kind is either "player" or "npc".
type CombatantState struct {
	UID   string
	Name  string
	Kind  string // "player" or "npc"
	HP    int
	MaxHP int
	AC    int
	Dead  bool
}

// HPPercent returns current HP as a percentage of MaxHP; 0 if MaxHP == 0.
func (c *CombatantState) HPPercent() float64 {
	if c.MaxHP <= 0 {
		return 0
	}
	return float64(c.HP) / float64(c.MaxHP) * 100
}

// NPCState captures the planning NPC's own state.
type NPCState struct {
	UID        string
	Name       string
	Kind       string // always "npc"
	HP         int
	MaxHP      int
	Perception int
	ZoneID     string
	RoomID     string
}

// RoomState captures room context at planning time.
type RoomState struct {
	ID     string
	ZoneID string
	Title  string
}

// WorldState is the snapshot passed to the HTN planner for one NPC.
//
// Invariant: NPC must not be nil.
type WorldState struct {
	NPC        *NPCState
	Room       *RoomState
	Combatants []*CombatantState // all combatants in the encounter
}

// EnemiesOf returns all living combatants of the opposite kind from uid.
//
// Precondition: uid must be the NPC's UID; ws.NPC must not be nil.
// Postcondition: returned slice contains no dead combatants and no same-kind combatants.
func (ws *WorldState) EnemiesOf(uid string) []*CombatantState {
	var out []*CombatantState
	for _, c := range ws.Combatants {
		if !c.Dead && c.UID != uid && c.Kind != ws.NPC.Kind {
			out = append(out, c)
		}
	}
	return out
}

// HasLivingEnemies returns true when at least one living enemy exists.
//
// Postcondition: equivalent to len(EnemiesOf(uid)) > 0.
func (ws *WorldState) HasLivingEnemies(uid string) bool {
	return len(ws.EnemiesOf(uid)) > 0
}

// NearestEnemy returns the first living enemy (by Combatants order), or nil.
//
// Postcondition: nil if no living enemies exist.
func (ws *WorldState) NearestEnemy(uid string) *CombatantState {
	enemies := ws.EnemiesOf(uid)
	if len(enemies) == 0 {
		return nil
	}
	return enemies[0]
}

// WeakestEnemy returns the living enemy with the lowest HP percentage, or nil.
//
// Postcondition: nil if no living enemies exist; ties broken by order in Combatants.
func (ws *WorldState) WeakestEnemy(uid string) *CombatantState {
	enemies := ws.EnemiesOf(uid)
	if len(enemies) == 0 {
		return nil
	}
	weakest := enemies[0]
	for _, e := range enemies[1:] {
		if e.HPPercent() < weakest.HPPercent() {
			weakest = e
		}
	}
	return weakest
}

// AlliesOf returns all living combatants of the same kind as the NPC (excluding self).
//
// Postcondition: returned slice excludes the NPC itself and dead combatants.
func (ws *WorldState) AlliesOf(uid string) []*CombatantState {
	var out []*CombatantState
	for _, c := range ws.Combatants {
		if !c.Dead && c.UID != uid && c.Kind == ws.NPC.Kind {
			out = append(out, c)
		}
	}
	return out
}

// ResolveTarget maps a target token to a combatant name/UID.
//
// Precondition: ws.NPC must not be nil.
// Postcondition: tokens "nearest_enemy"/"weakest_enemy"/"self" are resolved to names;
// unknown tokens are returned as-is; empty string returned if target is nil.
func (ws *WorldState) ResolveTarget(token string) string {
	switch token {
	case "nearest_enemy":
		if e := ws.NearestEnemy(ws.NPC.UID); e != nil {
			return e.Name
		}
		return ""
	case "weakest_enemy":
		if e := ws.WeakestEnemy(ws.NPC.UID); e != nil {
			return e.Name
		}
		return ""
	case "self":
		return ws.NPC.Name
	default:
		return token
	}
}

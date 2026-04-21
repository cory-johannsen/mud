package gameserver

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// Context string constants for action availability filtering.
const (
	// combatContext is active when the player is in combat.
	combatContext = "combat"
	// explorationContext is active when the player is idle or exploring.
	explorationContext = "exploration"
	// downtimeContext is active when the player is in downtime.
	downtimeContext = "downtime"
)

// statusInCombat is the Status value from gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT.
const statusInCombat = int32(2)

// statusUnconscious is the Status value for an unconscious player.
const statusUnconscious = int32(3)

// ContextForSession derives the current context string from a player session.
//
// Precondition: sess must not be nil.
// Postcondition: Returns "combat" if sess.Status == 2 (IN_COMBAT), "exploration" otherwise.
func ContextForSession(sess *session.PlayerSession) string {
	if sess.Status == statusInCombat {
		return combatContext
	}
	return explorationContext
}

// AvailableActions returns all active class features from reg that:
//  1. Are owned by the player (sess.PassiveFeats[f.ID] == true).
//  2. Have Active == true (player-activated, not passive).
//  3. Have ctx listed in their Contexts slice.
//
// Precondition: sess, reg must not be nil; ctx must be a non-empty string.
// Postcondition: Returns a non-nil slice (may be empty).
func AvailableActions(sess *session.PlayerSession, reg *ruleset.ClassFeatureRegistry, ctx string) []*ruleset.ClassFeature {
	out := make([]*ruleset.ClassFeature, 0)
	for _, f := range reg.ActiveFeatures() {
		if !sess.PassiveFeats[f.ID] {
			continue
		}
		if !featureInContext(f, ctx) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// featureInContext reports whether f is valid in the given context string.
//
// Precondition: f must not be nil.
// Postcondition: Returns true if ctx appears in f.Contexts; false if Contexts is empty or ctx not found.
func featureInContext(f *ruleset.ClassFeature, ctx string) bool {
	for _, c := range f.Contexts {
		if c == ctx {
			return true
		}
	}
	return false
}

// ActionHandler resolves player-initiated ability/action activations, applying
// their effects (conditions, heals, damage, skill checks) and queuing combat actions.
//
// Precondition: sessions, registry, condReg, npcMgr, charSaver must be non-nil at construction.
type ActionHandler struct {
	sessions  *session.Manager
	registry  *ruleset.ClassFeatureRegistry
	condReg   *condition.Registry
	npcMgr    *npc.Manager
	combatH   *CombatHandler
	charSaver CharacterSaver
	diceRoller *dice.Roller
	logger    *zap.Logger
}

// NewActionHandler constructs an ActionHandler.
//
// Precondition: sessions, registry, condReg, npcMgr, charSaver must be non-nil.
// Postcondition: Returns a non-nil *ActionHandler.
func NewActionHandler(
	sessions *session.Manager,
	registry *ruleset.ClassFeatureRegistry,
	condReg *condition.Registry,
	npcMgr *npc.Manager,
	combatH *CombatHandler,
	charSaver CharacterSaver,
	diceRoller *dice.Roller,
	logger *zap.Logger,
) *ActionHandler {
	return &ActionHandler{
		sessions:   sessions,
		registry:   registry,
		condReg:    condReg,
		npcMgr:     npcMgr,
		combatH:    combatH,
		charSaver:  charSaver,
		diceRoller: diceRoller,
		logger:     logger,
	}
}

// Handle activates the named ability for the player identified by sess.
// name is the feature ID; target is the optional target name (may be empty).
//
// Precondition: ctx must not be nil; sess must not be nil; name must be non-empty.
// Postcondition: Returns nil on success or a descriptive error.
func (h *ActionHandler) Handle(ctx context.Context, sess *session.PlayerSession, name, target string) error {
	feature, err := h.resolveFeature(sess, name)
	if err != nil {
		return err
	}

	actionCtx := ContextForSession(sess)

	if !featureInContext(feature, actionCtx) {
		return fmt.Errorf("ability %q is not available in the current context (%s)", name, actionCtx)
	}

	if actionCtx == combatContext {
		return h.queueCombatAction(ctx, sess, feature, target)
	}

	return h.resolveEffect(ctx, sess, feature, target)
}

// resolveFeature looks up a feature by name in the registry and verifies the player owns it.
//
// Precondition: sess and name must be non-nil/non-empty.
// Postcondition: Returns the feature or an error if not found/not owned.
func (h *ActionHandler) resolveFeature(sess *session.PlayerSession, name string) (*ruleset.ClassFeature, error) {
	feature, ok := h.registry.ClassFeature(name)
	if !ok {
		return nil, fmt.Errorf("unknown ability: %q", name)
	}
	if !feature.Active {
		return nil, fmt.Errorf("ability %q is passive and cannot be activated directly", name)
	}
	if !sess.PassiveFeats[name] {
		return nil, fmt.Errorf("you do not have the ability %q", name)
	}
	return feature, nil
}

// listActions returns a formatted list of available actions for the session's current context.
func (h *ActionHandler) listActions(sess *session.PlayerSession) string {
	ctx := ContextForSession(sess)
	actions := AvailableActions(sess, h.registry, ctx)
	if len(actions) == 0 {
		return "No actions available in the current context."
	}
	msg := "Available actions:\n"
	for _, a := range actions {
		msg += fmt.Sprintf("  %s - %s\n", a.ID, a.Name)
	}
	return msg
}

// queueCombatAction queues the feature as a combat action via the combat handler.
//
// Precondition: sess and feature must be non-nil.
// Postcondition: Returns nil on success or an error if the action cannot be queued.
func (h *ActionHandler) queueCombatAction(ctx context.Context, sess *session.PlayerSession, feature *ruleset.ClassFeature, target string) error {
	ap := h.combatH.RemainingAP(sess.UID)
	cost := feature.ActionCost
	if cost <= 0 {
		cost = 1
	}
	if ap < cost {
		return fmt.Errorf("not enough action points: need %d, have %d", cost, ap)
	}

	qa := combat.QueuedAction{
		Type:        combat.ActionUseAbility,
		AbilityID:   feature.ID,
		Target:      target,
		AbilityCost: cost,
	}

	if err := h.combatH.ActivateAbility(sess.UID, qa); err != nil {
		return fmt.Errorf("queuing combat ability %q: %w", feature.ID, err)
	}

	msg := fmt.Sprintf("Ability %q queued for combat (cost: %d AP).", feature.Name, cost)
	h.sendMessage(sess, msg)
	return nil
}

// resolveEffect applies the feature's ActionEffect to the player and/or target.
//
// Precondition: sess and feature must be non-nil.
// Postcondition: Returns nil on success or a descriptive error.
func (h *ActionHandler) resolveEffect(ctx context.Context, sess *session.PlayerSession, feature *ruleset.ClassFeature, target string) error {
	if feature.Effect == nil {
		// No mechanical effect; just send the activation text.
		text := firstOf(feature.ActivateText, fmt.Sprintf("You use %s.", feature.Name))
		h.sendMessage(sess, text)
		return nil
	}

	switch feature.Effect.Type {
	case "condition":
		return h.resolveConditionEffect(ctx, sess, feature, target)
	case "heal":
		return h.resolveHealEffect(ctx, sess, feature)
	case "damage":
		return h.resolveDamageEffect(ctx, sess, feature, target)
	case "skill_check":
		return h.resolveSkillCheckEffect(ctx, sess, feature)
	default:
		return fmt.Errorf("unknown effect type %q for ability %q", feature.Effect.Type, feature.ID)
	}
}

// resolveConditionEffect applies a condition to the appropriate target.
//
// Precondition: feature.Effect must not be nil and must have Type == "condition".
// Postcondition: Returns nil on success or an error if the condition is unknown or application fails.
func (h *ActionHandler) resolveConditionEffect(ctx context.Context, sess *session.PlayerSession, feature *ruleset.ClassFeature, target string) error {
	effect := feature.Effect
	def, ok := h.condReg.Get(effect.ConditionID)
	if !ok {
		return fmt.Errorf("unknown condition %q in ability %q", effect.ConditionID, feature.ID)
	}

	switch effect.Target {
	case "self":
		if sess.Conditions == nil {
			return fmt.Errorf("player condition set not initialised")
		}
		if err := sess.Conditions.Apply(sess.UID, def, 1, -1); err != nil {
			return fmt.Errorf("applying condition %q: %w", effect.ConditionID, err)
		}
	case "target":
		npcInst := h.combatH.FindNPCInCombat(sess.RoomID, target)
		if npcInst == nil {
			return fmt.Errorf("no NPC target %q found in active combat for ability %q", target, feature.ID)
		}
		if err := h.combatH.ApplyConditionToNPC(sess.RoomID, npcInst.ID, effect.ConditionID, 1, -1); err != nil {
			return fmt.Errorf("applying condition %q to target %q: %w", effect.ConditionID, target, err)
		}
	default:
		return fmt.Errorf("unsupported condition target %q for ability %q", effect.Target, feature.ID)
	}

	text := firstOf(feature.ActivateText, fmt.Sprintf("You use %s.", feature.Name))
	h.sendMessage(sess, text)
	return nil
}

// resolveHealEffect heals the player by rolling the effect's Amount dice expression.
//
// Precondition: feature.Effect must not be nil and must have Type == "heal".
// Postcondition: Returns nil on success or an error if the dice expression is invalid.
func (h *ActionHandler) resolveHealEffect(ctx context.Context, sess *session.PlayerSession, feature *ruleset.ClassFeature) error {
	effect := feature.Effect
	result, err := h.diceRoller.RollExpr(effect.Amount)
	if err != nil {
		return fmt.Errorf("rolling heal for %q: %w", feature.ID, err)
	}

	healed := result.Total()
	newHP := sess.CurrentHP + healed
	if newHP > sess.MaxHP {
		newHP = sess.MaxHP
	}
	sess.CurrentHP = newHP

	if h.charSaver != nil {
		if saveErr := h.charSaver.SaveState(ctx, sess.CharacterID, sess.RoomID, newHP); saveErr != nil {
			if h.logger != nil {
				h.logger.Warn("action_handler: saving heal HP",
					zap.String("uid", sess.UID),
					zap.Error(saveErr),
				)
			}
		}
	}

	msg := fmt.Sprintf("You use %s and recover %d HP. (%d/%d)", feature.Name, healed, newHP, sess.MaxHP)
	h.sendMessage(sess, msg)
	h.sendHPUpdate(sess)
	return nil
}

// resolveDamageEffect applies damage to the target NPC.
//
// Precondition: feature.Effect must not be nil and must have Type == "damage".
// Postcondition: Returns nil on success or an error if the dice expression or target is invalid.
func (h *ActionHandler) resolveDamageEffect(ctx context.Context, sess *session.PlayerSession, feature *ruleset.ClassFeature, target string) error {
	effect := feature.Effect
	result, err := h.diceRoller.RollExpr(effect.Amount)
	if err != nil {
		return fmt.Errorf("rolling damage for %q: %w", feature.ID, err)
	}

	dmg := result.Total()

	inst := h.npcMgr.FindInRoom(sess.RoomID, target)
	if inst == nil {
		return fmt.Errorf("target %q not found in current room", target)
	}

	inst.CurrentHP -= dmg
	if inst.CurrentHP < 0 {
		inst.CurrentHP = 0
	}

	msg := fmt.Sprintf("You use %s on %s for %d %s damage.", feature.Name, target, dmg, effect.DamageType)
	h.sendMessage(sess, msg)
	return nil
}

// resolveSkillCheckEffect performs a skill check for the ability.
//
// Precondition: feature.Effect must not be nil and must have Type == "skill_check".
// Postcondition: Returns nil on success or an error if the dice roll fails.
func (h *ActionHandler) resolveSkillCheckEffect(ctx context.Context, sess *session.PlayerSession, feature *ruleset.ClassFeature) error {
	effect := feature.Effect

	result, err := h.diceRoller.RollExpr("1d20")
	if err != nil {
		return fmt.Errorf("rolling skill check for %q: %w", feature.ID, err)
	}

	roll := result.Total()
	rank := sess.Skills[effect.Skill]
	bonus := skillRankBonus(rank)
	penalty := condition.SkillPenalty(sess.Conditions)
	total := roll + bonus - penalty

	var outcome string
	switch {
	case roll == 20 || total >= effect.DC+10:
		outcome = "critical success"
	case total >= effect.DC:
		outcome = "success"
	case roll == 1 || total <= effect.DC-10:
		outcome = "critical failure"
	default:
		outcome = "failure"
	}

	var msg string
	if penalty > 0 {
		msg = fmt.Sprintf("You use %s — %s skill check: rolled %d+%d-%d=%d vs DC %d — %s.",
			feature.Name, effect.Skill, roll, bonus, penalty, total, effect.DC, outcome)
	} else {
		msg = fmt.Sprintf("You use %s — %s skill check: rolled %d+%d=%d vs DC %d — %s.",
			feature.Name, effect.Skill, roll, bonus, total, effect.DC, outcome)
	}
	h.sendMessage(sess, msg)
	return nil
}

// skillRankBonus converts a skill proficiency rank string to a flat bonus.
func skillRankBonus(rank string) int {
	switch rank {
	case "trained":
		return 2
	case "expert":
		return 4
	case "master":
		return 6
	case "legendary":
		return 8
	default:
		return 0
	}
}

// sendMessage pushes a console text message to the player's stream.
//
// Precondition: sess must not be nil.
func (h *ActionHandler) sendMessage(sess *session.PlayerSession, msg string) {
	evt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: msg,
				Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
			},
		},
	}
	if data, err := proto.Marshal(evt); err == nil {
		_ = sess.Entity.Push(data)
	}
}

// sendHPUpdate pushes an HP update event so the prompt bar refreshes.
//
// Precondition: sess must not be nil.
func (h *ActionHandler) sendHPUpdate(sess *session.PlayerSession) {
	evt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_HpUpdate{
			HpUpdate: &gamev1.HpUpdateEvent{
				CurrentHp: int32(sess.CurrentHP),
				MaxHp:     int32(sess.MaxHP),
			},
		},
	}
	if data, err := proto.Marshal(evt); err == nil {
		_ = sess.Entity.Push(data)
	}
}

// firstOf returns the first non-empty string from the arguments.
//
// Postcondition: Returns "" only if all arguments are empty.
func firstOf(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

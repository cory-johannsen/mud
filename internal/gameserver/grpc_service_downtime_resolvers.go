package gameserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/crafting"
	"github.com/cory-johannsen/mud/internal/game/drawback"
	"github.com/cory-johannsen/mud/internal/game/focuspoints"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/skillcheck"
)

// defaultDC is the fallback DC for skill checks when no zone or metadata DC is provided.
const defaultDC = 15

// earnCredsBasePay is the base currency awarded on a Failure in the Earn Creds activity.
const earnCredsBasePay = 10

// patchUpCritSuccessMultiplier is the HP heal multiplier applied on a Critical Success in Patch Up.
const patchUpCritSuccessMultiplier = 4

// patchUpSuccessMultiplier is the HP heal multiplier applied on a Success in Patch Up.
const patchUpSuccessMultiplier = 2

// patchUpFailureMultiplier is the HP heal multiplier applied on a Failure in Patch Up.
const patchUpFailureMultiplier = 1

// runCoverCircumstanceBonus is the circumstance bonus granted to hustle checks in a zone after Run Cover succeeds.
const runCoverCircumstanceBonus = 1

// resolveDowntimeActivityDispatch dispatches per-activity resolution logic after
// the generic state-clearing block in resolveDowntimeActivity.
//
// Precondition: actID is the activity ID that was active; sess is non-nil; state already cleared.
// Postcondition: Per-activity effects applied; console message delivered to uid.
func (s *GameServiceServer) resolveDowntimeActivityDispatch(uid, actID string, sess *session.PlayerSession) {
	switch actID {
	case "earn_creds":
		s.resolveEarnCreds(uid, sess)
	case "patch_up":
		s.resolvePatchUp(uid, sess)
	case "recalibrate":
		s.resolveRecalibrate(uid, sess)
	case "retrain":
		s.resolveRetrain(uid, sess)
	case "forge_papers":
		s.resolveForgePapers(uid, sess)
	case "subsist":
		s.resolveSubsist(uid, sess)
	case "fight_sickness":
		s.resolveFightSickness(uid, sess)
	case "flush_it":
		s.resolveFlushIt(uid, sess)
	case "run_intel":
		s.resolveRunIntel(uid, sess)
	case "analyze_tech":
		s.resolveAnalyzeTech(uid, sess)
	case "field_repair":
		s.resolveFieldRepair(uid, sess)
	case "crack_code":
		s.resolveCrackCode(uid, sess)
	case "run_cover":
		s.resolveRunCover(uid, sess)
	case "apply_pressure":
		s.resolveApplyPressure(uid, sess)
	case "craft":
		s.resolveDowntimeCraft(uid, sess)
	default:
		s.pushMessageToUID(uid, fmt.Sprintf("Downtime activity %q completed.", actID))
	}
}

// rollD20 rolls a d20 using the server's dice roller.
// Returns 10 when s.dice is nil (neutral result).
//
// Precondition: none.
// Postcondition: Returns a value in [1, 20].
func (s *GameServiceServer) rollD20() int {
	if s.dice == nil {
		return 10
	}
	return s.dice.Src().Intn(20) + 1
}

// bestSkillForEarning returns the skill ID and proficiency rank with the highest
// ProficiencyBonus among "rigging", "intel", and "rep" in the session's skills map.
// Returns ("rep", "") if none are found (rep is the default tie-break skill).
//
// Precondition: sess is non-nil.
// Postcondition: Returns the skillID and rank string with the highest ProficiencyBonus.
func bestSkillForEarning(sess *session.PlayerSession) (skillID, rank string) {
	candidates := []string{"rigging", "intel", "rep"}
	bestID := "rep"
	bestRank := ""
	bestBonus := -1
	for _, id := range candidates {
		r := ""
		if sess.Skills != nil {
			r = sess.Skills[id]
		}
		bonus := skillcheck.ProficiencyBonus(r)
		if bonus > bestBonus {
			bestBonus = bonus
			bestID = id
			bestRank = r
		}
	}
	return bestID, bestRank
}

// skillCheckOutcome performs a skill check for the given skill against dc and
// returns the resulting CheckOutcome.
//
// Precondition: skillID and sess are non-nil.
// Postcondition: Returns one of CritSuccess, Success, Failure, CritFailure.
func (s *GameServiceServer) skillCheckOutcome(sess *session.PlayerSession, skillID string, dc int) skillcheck.CheckOutcome {
	abilityScore := s.abilityScoreForSkill(sess, skillID)
	amod := abilityModFrom(abilityScore)
	rank := ""
	if sess.Skills != nil {
		rank = sess.Skills[skillID]
	}
	roll := s.rollD20()
	total := roll + amod + skillcheck.ProficiencyBonus(rank)
	outcome := skillcheck.OutcomeFor(total, dc)
	// REQ-JD-10: Fire on_fail_skill_check drawback trigger on Failure or CritFailure.
	if outcome == skillcheck.Failure || outcome == skillcheck.CritFailure {
		if s.drawbackEngine != nil && s.jobRegistry != nil && sess.Conditions != nil {
			heldJobs := s.resolveHeldJobs(sess)
			s.drawbackEngine.FireTrigger(sess.UID, drawback.TriggerOnFailSkillCheck, heldJobs, sess.Conditions, time.Now())
		}
	}
	return outcome
}

// resolveEarnCreds resolves the "Earn Creds" downtime activity.
//
// Skill: best of rigging/intel/rep (highest proficiency rank).
// DC: zone.SettlementDC (default 15).
// Pay: CritSuccess=30, Success=20, Failure=10, CritFail=0.
//
// Precondition: sess is non-nil; state already cleared by resolveDowntimeActivity.
// Postcondition: sess.Currency incremented; persisted via charSaver if available.
func (s *GameServiceServer) resolveEarnCreds(uid string, sess *session.PlayerSession) {
	skillID, _ := bestSkillForEarning(sess)

	dc := defaultDC
	if s.world != nil {
		if room, ok := s.world.GetRoom(sess.RoomID); ok {
			if zone, ok := s.world.GetZone(room.ZoneID); ok && zone.SettlementDC > 0 {
				dc = zone.SettlementDC
			}
		}
	}

	outcome := s.skillCheckOutcome(sess, skillID, dc)

	var earned int
	var outcomeMsg string
	switch outcome {
	case skillcheck.CritSuccess:
		earned = earnCredsBasePay * 3
		outcomeMsg = fmt.Sprintf("Earn Creds complete. Critical success! You netted %d credits.", earned)
	case skillcheck.Success:
		earned = earnCredsBasePay * 2
		outcomeMsg = fmt.Sprintf("Earn Creds complete. Success. You earned %d credits.", earned)
	case skillcheck.Failure:
		earned = earnCredsBasePay
		outcomeMsg = fmt.Sprintf("Earn Creds complete. Partial success. You scraped together %d credits.", earned)
	default: // CritFailure
		earned = 0
		outcomeMsg = "Earn Creds complete. Critical failure — nothing to show for your efforts."
	}

	sess.Currency += earned

	if s.charSaver != nil && sess.CharacterID > 0 {
		_ = s.charSaver.SaveCurrency(context.Background(), sess.CharacterID, sess.Currency)
	}

	s.pushMessageToUID(uid, outcomeMsg)
}

// resolvePatchUp resolves the "Patch Up" downtime activity.
//
// Skill: patch_job (Savvy). DC: 15.
// Heal: CritSuccess=level×4, Success=level×2, Failure=level×1, CritFail=0.
//
// Precondition: sess is non-nil; state already cleared.
// Postcondition: sess.CurrentHP increased (capped at MaxHP); HP update pushed if Entity non-nil.
func (s *GameServiceServer) resolvePatchUp(uid string, sess *session.PlayerSession) {
	outcome := s.skillCheckOutcome(sess, "patch_job", defaultDC)

	var heal int
	var outcomeMsg string
	switch outcome {
	case skillcheck.CritSuccess:
		heal = sess.Level * patchUpCritSuccessMultiplier
		outcomeMsg = fmt.Sprintf("Patch Up complete. Critical success! You healed %d HP.", heal)
	case skillcheck.Success:
		heal = sess.Level * patchUpSuccessMultiplier
		outcomeMsg = fmt.Sprintf("Patch Up complete. Success. You healed %d HP.", heal)
	case skillcheck.Failure:
		heal = sess.Level * patchUpFailureMultiplier
		outcomeMsg = fmt.Sprintf("Patch Up complete. Partial success. You healed %d HP.", heal)
	default: // CritFailure
		heal = 0
		outcomeMsg = "Patch Up complete. Critical failure — the procedure made things worse. No healing."
	}

	if heal > 0 {
		sess.CurrentHP += heal
		if sess.CurrentHP > sess.MaxHP {
			sess.CurrentHP = sess.MaxHP
		}
		if s.charSaver != nil && sess.CharacterID > 0 {
			_ = s.charSaver.SaveState(context.Background(), sess.CharacterID, sess.RoomID, sess.CurrentHP)
		}
		s.pushHPUpdate(uid, sess)
	}

	s.pushMessageToUID(uid, outcomeMsg)
}

// resolveRecalibrate resolves the "Recalibrate" downtime activity.
//
// Stub: The full focus-points restoration logic is deferred until the focus-points
// recharge-on-rest system (REQ-FP-*) is implemented. This stub sends a console message
// describing the intended outcome.
//
// Precondition: sess is non-nil; state already cleared.
// Postcondition: Console message delivered; FocusPoints restored to MaxFocusPoints via
//
//	focuspoints.Restore when Entity is non-nil.
func (s *GameServiceServer) resolveRecalibrate(uid string, sess *session.PlayerSession) {
	// Restore focus points to max using focuspoints package.
	// Future: link to REQ-FP-* recharge-on-rest rules.
	sess.FocusPoints = focuspoints.Clamp(sess.MaxFocusPoints, sess.MaxFocusPoints)
	if s.charSaver != nil && sess.CharacterID > 0 {
		_ = s.charSaver.SaveFocusPoints(context.Background(), sess.CharacterID, sess.FocusPoints)
	}
	s.pushMessageToUID(uid, "Recalibrate complete. Focus Points restored.")
}

// resolveRetrain resolves the "Retrain" downtime activity.
//
// sess.DowntimeMetadata must hold "<old_feat_id> <new_feat_id>" set at start time.
// Removes the old feat from sess.PassiveFeats, adds the new feat, persists via characterFeatsRepo
// when it implements SetAll (REQ-RETRAIN-DT-4).
//
// Precondition: sess is non-nil; state already cleared by caller.
// Postcondition: Old feat removed and new feat added to sess.PassiveFeats;
//
//	changes persisted via characterFeatsRepo if it supports SetAll; console message delivered.
func (s *GameServiceServer) resolveRetrain(uid string, sess *session.PlayerSession) {
	parts := strings.Fields(sess.DowntimeMetadata)
	if len(parts) < 2 {
		s.pushMessageToUID(uid, "Retrain complete. (No feat change recorded.)")
		return
	}
	oldID, newID := parts[0], parts[1]

	if s.featRegistry == nil {
		s.pushMessageToUID(uid, "Retrain complete. (Feat system unavailable.)")
		return
	}
	oldFeat, oldOK := s.featRegistry.Feat(oldID)
	newFeat, newOK := s.featRegistry.Feat(newID)
	if !oldOK || !newOK {
		s.pushMessageToUID(uid, "Retrain complete. (Feat definition missing.)")
		return
	}

	// Apply the swap in session state.
	if sess.PassiveFeats == nil {
		sess.PassiveFeats = make(map[string]bool)
	}
	delete(sess.PassiveFeats, oldID)
	sess.PassiveFeats[newID] = true

	// Persist the updated feat list (REQ-RETRAIN-DT-4).
	// characterFeatsRepo is typed as CharacterFeatsGetter; use a type assertion to access SetAll
	// when the concrete implementation supports it (e.g. *postgres.CharacterFeatsRepository).
	type featsSetter interface {
		SetAll(ctx context.Context, characterID int64, feats []string) error
	}
	if setter, ok := s.characterFeatsRepo.(featsSetter); ok && sess.CharacterID > 0 {
		var featIDs []string
		for id := range sess.PassiveFeats {
			featIDs = append(featIDs, id)
		}
		if err := setter.SetAll(context.Background(), sess.CharacterID, featIDs); err != nil && s.logger != nil {
			s.logger.Error("resolveRetrain: failed to persist feat changes",
				zap.String("uid", uid), zap.Error(err))
		}
	}

	s.pushMessageToUID(uid, fmt.Sprintf(
		"Retrain complete. You replaced %s with %s.", oldFeat.Name, newFeat.Name,
	))
}

// resolveForgePapers resolves the "Forge Papers" downtime activity.
//
// Skill: hustle (Flair). DC: 15.
// CritSuccess: deliver "undetectable_forgery" + refund 1 "forgery_supplies" to backpack.
// Success: deliver "convincing_forgery" to backpack.
// Failure/CritFail: message only; supplies already consumed at activity start.
//
// Precondition: sess is non-nil; state already cleared; forgery_supplies consumed at activity start.
// Postcondition: Items added to backpack per outcome; console message delivered.
func (s *GameServiceServer) resolveForgePapers(uid string, sess *session.PlayerSession) {
	outcome := s.skillCheckOutcome(sess, "hustle", defaultDC)

	switch outcome {
	case skillcheck.CritSuccess:
		if sess.Backpack != nil {
			_ = sess.Backpack.AddInstance(&inventory.ItemInstance{
				InstanceID: uuid.New().String(),
				ItemDefID:  "undetectable_forgery",
				Quantity:   1,
			})
			// Refund one forgery_supplies on critical success. (REQ-DA-FORGE-2)
			_ = sess.Backpack.AddInstance(&inventory.ItemInstance{
				InstanceID: uuid.New().String(),
				ItemDefID:  "forgery_supplies",
				Quantity:   1,
			})
		}
		s.pushMessageToUID(uid, "Forge Papers complete. Critical success — you produced an undetectable forgery and recovered your supplies.")
	case skillcheck.Success:
		if sess.Backpack != nil {
			_ = sess.Backpack.AddInstance(&inventory.ItemInstance{
				InstanceID: uuid.New().String(),
				ItemDefID:  "convincing_forgery",
				Quantity:   1,
			})
		}
		s.pushMessageToUID(uid, "Forge Papers complete. Success — you produced a convincing forgery.")
	case skillcheck.Failure:
		s.pushMessageToUID(uid, "Forge Papers complete. Failure — the papers look suspicious. Your supplies are wasted.")
	default:
		s.pushMessageToUID(uid, "Forge Papers complete. Critical failure — you produced nothing usable and ruined your supplies.")
	}
}

// resolveSubsist resolves the "Subsist" downtime activity.
//
// Skill: scavenging (Savvy). DC: 15.
// CritSuccess: heal self for Level HP. Success: no penalty. Failure/CritFail: apply fatigued.
//
// Precondition: sess is non-nil; state already cleared.
// Postcondition: Console message delivered; HP or condition mutated per outcome.
func (s *GameServiceServer) resolveSubsist(uid string, sess *session.PlayerSession) {
	outcome := s.skillCheckOutcome(sess, "scavenging", defaultDC)

	var msg string
	switch outcome {
	case skillcheck.CritSuccess:
		heal := sess.Level
		if heal < 1 {
			heal = 1
		}
		sess.CurrentHP += heal
		if sess.CurrentHP > sess.MaxHP {
			sess.CurrentHP = sess.MaxHP
		}
		if s.charSaver != nil && sess.CharacterID > 0 {
			_ = s.charSaver.SaveState(context.Background(), sess.CharacterID, sess.RoomID, sess.CurrentHP)
		}
		s.pushHPUpdate(uid, sess)
		msg = fmt.Sprintf("Subsist complete. Critical success — you found enough to heal %d HP.", heal)
	case skillcheck.Success:
		msg = "Subsist complete. Success — you found enough to get by."
	case skillcheck.Failure:
		s.applyConditionToSession(uid, sess, "fatigue")
		msg = "Subsist complete. Failure — you barely survived and are now fatigued."
	default:
		s.applyConditionToSession(uid, sess, "fatigue")
		msg = "Subsist complete. Critical failure — you are starved and fatigued."
	}

	s.pushMessageToUID(uid, msg)
}

// resolveFightSickness resolves the "Fight Sickness" downtime activity.
//
// Skill: patch_job (Savvy). DC: 15 (or from metadata when condition DC lookup is wired).
//
// Stub: Full condition-stage progression is deferred until REQ-COND-* condition-sickness
// rules are implemented. This stub performs the skill check and reports the outcome.
//
// Precondition: sess is non-nil; state already cleared.
// Postcondition: Console message delivered describing check result.
func (s *GameServiceServer) resolveFightSickness(uid string, sess *session.PlayerSession) {
	// Future: look up condition DC from sess.DowntimeMetadata (REQ-COND-sickness).
	outcome := s.skillCheckOutcome(sess, "patch_job", defaultDC)

	var msg string
	switch outcome {
	case skillcheck.CritSuccess:
		msg = "Fight Sickness complete. Critical success — the sickness has been fully treated."
	case skillcheck.Success:
		msg = "Fight Sickness complete. Success — the sickness has been reduced by one stage."
	case skillcheck.Failure:
		msg = "Fight Sickness complete. Failure — the sickness holds firm. No change."
	default:
		msg = "Fight Sickness complete. Critical failure — the sickness has worsened by one stage."
	}

	s.pushMessageToUID(uid, msg)
}

// resolveFlushIt resolves the "Flush It" downtime activity.
//
// Skill: patch_job (Savvy). DC: 15.
//
// Stub: Full substance-flush stage mechanics are deferred until REQ-AH-* substance-addiction
// rules are integrated with downtime. This stub performs the skill check and reports the outcome.
//
// Precondition: sess is non-nil; state already cleared.
// Postcondition: Console message delivered describing check result.
func (s *GameServiceServer) resolveFlushIt(uid string, sess *session.PlayerSession) {
	// Future: integrate with REQ-AH-* substance-addiction flush stages.
	outcome := s.skillCheckOutcome(sess, "patch_job", defaultDC)

	var msg string
	switch outcome {
	case skillcheck.CritSuccess:
		msg = "Flush It complete. Critical success — the substance has been fully flushed from your system."
	case skillcheck.Success:
		msg = "Flush It complete. Success — the substance stage has been reduced."
	case skillcheck.Failure:
		msg = "Flush It complete. Failure — no progress. The substance lingers."
	default:
		msg = "Flush It complete. Critical failure — the withdrawal worsened. Seek help."
	}

	s.pushMessageToUID(uid, msg)
}

// resolveRunIntel resolves the "Run Intel" downtime activity.
//
// Skill: smooth_talk (Flair). DC: 15 (or from metadata).
//
// Stub: Full intel data retrieval is deferred until REQ-INTEL-* lore/faction data is wired.
//
// Precondition: sess is non-nil; state already cleared.
// Postcondition: Console message delivered describing check result.
func (s *GameServiceServer) resolveRunIntel(uid string, sess *session.PlayerSession) {
	// Future: retrieve target info from metadata and REQ-INTEL-* lore data.
	outcome := s.skillCheckOutcome(sess, "smooth_talk", defaultDC)

	var msg string
	switch outcome {
	case skillcheck.CritSuccess:
		msg = "Run Intel complete. Critical success — you gathered detailed intelligence on the target."
	case skillcheck.Success:
		msg = "Run Intel complete. Success — you gathered useful information on the target."
	case skillcheck.Failure:
		msg = "Run Intel complete. Failure — no useful information found on that target."
	default:
		msg = "Run Intel complete. Critical failure — your inquiries attracted unwanted attention."
	}

	s.pushMessageToUID(uid, msg)
}

// resolveAnalyzeTech resolves the "Analyze Tech" downtime activity.
//
// Skill: tech_lore (Reasoning). DC: 15 (or from metadata).
//
// Stub: Full item identification results are deferred until REQ-TECH-* tech-lore item identification
// rules are implemented.
//
// Precondition: sess is non-nil; state already cleared.
// Postcondition: Console message delivered describing check result.
func (s *GameServiceServer) resolveAnalyzeTech(uid string, sess *session.PlayerSession) {
	// Future: identify item from metadata and REQ-TECH-* identification rules.
	outcome := s.skillCheckOutcome(sess, "tech_lore", defaultDC)

	var msg string
	switch outcome {
	case skillcheck.CritSuccess:
		msg = "Analyze Tech complete. Critical success — full technical details revealed."
	case skillcheck.Success:
		msg = "Analyze Tech complete. Success — the technology has been identified."
	case skillcheck.Failure:
		msg = "Analyze Tech complete. Failure — you could not determine what the device does."
	default:
		msg = "Analyze Tech complete. Critical failure — your analysis corrupted the device's data port."
	}

	s.pushMessageToUID(uid, msg)
}

// resolveFieldRepair resolves the "Field Repair" downtime activity.
//
// Skill: rigging (Reasoning). DC: 15 (or from metadata).
//
// Stub: Full item durability restoration is deferred until REQ-GEAR-* item durability
// rules are implemented.
//
// Precondition: sess is non-nil; state already cleared.
// Postcondition: Console message delivered describing check result.
func (s *GameServiceServer) resolveFieldRepair(uid string, sess *session.PlayerSession) {
	// Future: restore item durability from metadata and REQ-GEAR-* durability rules.
	outcome := s.skillCheckOutcome(sess, "rigging", defaultDC)

	var msg string
	switch outcome {
	case skillcheck.CritSuccess:
		msg = "Field Repair complete. Critical success — the item is fully restored."
	case skillcheck.Success:
		msg = "Field Repair complete. Success — the item has been repaired."
	case skillcheck.Failure:
		msg = "Field Repair complete. Failure — the repair attempt yielded minimal improvement."
	default:
		msg = "Field Repair complete. Critical failure — the repair attempt made things worse."
	}

	s.pushMessageToUID(uid, msg)
}

// resolveCrackCode resolves the "Crack Code" downtime activity.
//
// Skill: intel (Reasoning) by default; may be overridden via metadata. DC: 15.
//
// Stub: Full decode result delivery is deferred until REQ-INTEL-* decode data rules
// are implemented.
//
// Precondition: sess is non-nil; state already cleared.
// Postcondition: Console message delivered describing check result.
func (s *GameServiceServer) resolveCrackCode(uid string, sess *session.PlayerSession) {
	// Future: read skill and DC overrides from DowntimeMetadata (REQ-INTEL-crack-code).
	outcome := s.skillCheckOutcome(sess, "intel", defaultDC)

	var msg string
	switch outcome {
	case skillcheck.CritSuccess:
		msg = "Crack Code complete. Critical success — the code has been fully decoded."
	case skillcheck.Success:
		msg = "Crack Code complete. Success — you successfully cracked the code."
	case skillcheck.Failure:
		msg = "Crack Code complete. Failure — the code resisted your attempts."
	default:
		msg = "Crack Code complete. Critical failure — your attempt triggered a security lockout."
	}

	s.pushMessageToUID(uid, msg)
}

// resolveRunCover resolves the "Run Cover" downtime activity.
//
// Skill: hustle (Flair). DC: 15.
// CritSuccess/Success: sets a zone circumstance bonus for hustle in the current zone.
// Failure/CritFail: delivers a failure message.
//
// Precondition: sess is non-nil; state already cleared.
// Postcondition: Console message delivered; ZoneCircumstanceBonus updated on success.
func (s *GameServiceServer) resolveRunCover(uid string, sess *session.PlayerSession) {
	outcome := s.skillCheckOutcome(sess, "hustle", defaultDC)

	var msg string
	switch outcome {
	case skillcheck.CritSuccess, skillcheck.Success:
		zoneID := ""
		if s.world != nil {
			if room, ok := s.world.GetRoom(sess.RoomID); ok {
				zoneID = room.ZoneID
			}
		}
		if zoneID != "" {
			if sess.ZoneCircumstanceBonus == nil {
				sess.ZoneCircumstanceBonus = make(map[string]int)
			}
			sess.ZoneCircumstanceBonus[zoneID+":hustle"] = runCoverCircumstanceBonus
		}
		msg = "Run Cover complete. Your street presence makes things easier in this area."
	case skillcheck.Failure:
		msg = "Run Cover complete. Failure — you kept your head down but nothing came of it."
	default:
		msg = "Run Cover complete. Critical failure — your cover was blown. Stay sharp."
	}

	s.pushMessageToUID(uid, msg)
}

// resolveApplyPressure resolves the "Apply Pressure" downtime activity.
//
// Skill: hard_look (Flair). DC: 15 (or from metadata).
//
// Precondition: sess is non-nil; state already cleared.
// Postcondition: Console message delivered describing check result.
func (s *GameServiceServer) resolveApplyPressure(uid string, sess *session.PlayerSession) {
	// Future: read target name and DC from DowntimeMetadata (REQ-PRESSURE-*).
	outcome := s.skillCheckOutcome(sess, "hard_look", defaultDC)

	var msg string
	switch outcome {
	case skillcheck.CritSuccess:
		msg = "Apply Pressure complete. Critical success — they capitulated completely."
	case skillcheck.Success:
		msg = "Apply Pressure complete. Success — they agreed to your terms."
	case skillcheck.Failure:
		msg = "Apply Pressure complete. Failure — they didn't budge."
	default:
		msg = "Apply Pressure complete. Critical failure — the confrontation turned hostile."
	}

	s.pushMessageToUID(uid, msg)
}

// resolveDowntimeCraft resolves the "Craft" downtime activity.
//
// Materials were consumed eagerly at activity start (REQ-CRAFT-DT-1).
// Rolls a rigging skill check vs recipe DC; delivers output items on Success/CritSuccess;
// refunds one material batch on CritSuccess (REQ-CRAFT-DT-4).
//
// Precondition: sess is non-nil; sess.DowntimeMetadata holds the recipe ID; state already cleared.
// Postcondition: Items delivered to backpack on success; material refund on crit success; message pushed.
func (s *GameServiceServer) resolveDowntimeCraft(uid string, sess *session.PlayerSession) {
	if sess.DowntimeMetadata == "" || s.recipeReg == nil {
		s.pushMessageToUID(uid, "(No recipe recorded.)")
		return
	}
	recipe, ok := s.recipeReg.Recipe(sess.DowntimeMetadata)
	if !ok {
		s.pushMessageToUID(uid, fmt.Sprintf("Unknown recipe %q; cannot complete craft.", sess.DowntimeMetadata))
		return
	}

	// Roll rigging skill check (same pattern as handleCraftConfirm).
	var roll int
	if s.dice != nil {
		roll = s.dice.Src().Intn(20) + 1
	} else {
		roll = 10
	}
	riggingRank := sess.Skills["rigging"]
	if riggingRank == "" {
		riggingRank = "untrained"
	}
	abilityMod := (sess.Abilities.Savvy - 10) / 2
	profBonus := skillcheck.ProficiencyBonus(riggingRank)
	total := roll + abilityMod + profBonus
	checkOutcome := skillcheck.OutcomeFor(total, recipe.DC)
	craftOutcome := crafting.Outcome(checkOutcome)

	// Compute output quantity.
	var outputQty int
	switch craftOutcome {
	case crafting.CritSuccess:
		outputQty = recipe.OutputCount + 1
	case crafting.Success:
		outputQty = recipe.OutputCount
	default:
		outputQty = 0
	}

	// Add output items to backpack.
	if outputQty > 0 && sess.Backpack != nil && s.invRegistry != nil && recipe.OutputItemID != "" {
		if _, err := sess.Backpack.Add(recipe.OutputItemID, outputQty, s.invRegistry); err != nil {
			s.pushMessageToUID(uid, fmt.Sprintf("Craft complete but failed to add %s to backpack.", recipe.Name))
			return
		}
	}

	// On CritSuccess, refund one material batch (REQ-CRAFT-DT-4).
	if craftOutcome == crafting.CritSuccess {
		for _, rm := range recipe.Materials {
			sess.Materials[rm.ID] += rm.Quantity
			if s.materialRepo != nil && sess.CharacterID > 0 {
				if errAdd := s.materialRepo.Add(context.Background(), sess.CharacterID, rm.ID, rm.Quantity); errAdd != nil && s.logger != nil {
					s.logger.Error("resolveDowntimeCraft: failed to persist material refund", zap.String("mat_id", rm.ID), zap.Error(errAdd))
				}
			}
		}
	}

	// Send outcome message.
	var msg string
	switch craftOutcome {
	case crafting.CritSuccess:
		msg = fmt.Sprintf("Critical success! You craft %d %s and recover your materials.", outputQty, recipe.Name)
	case crafting.Success:
		msg = fmt.Sprintf("Success! You craft %d %s.", outputQty, recipe.Name)
	case crafting.Failure:
		msg = fmt.Sprintf("Failure. You do not craft %s.", recipe.Name)
	default:
		msg = fmt.Sprintf("Critical failure! You do not craft %s.", recipe.Name)
	}
	s.pushMessageToUID(uid, msg)
}

// applyConditionToSession applies a condition by ID to the player's active condition set.
// No-op if condRegistry is nil or the condition is not found.
//
// Precondition: condID is a non-empty condition ID; sess is non-nil.
// Postcondition: Condition applied to sess.Conditions if registry and condition are available.
func (s *GameServiceServer) applyConditionToSession(uid string, sess *session.PlayerSession, condID string) {
	if s.condRegistry == nil || sess.Conditions == nil {
		return
	}
	def, ok := s.condRegistry.Get(condID)
	if !ok {
		return
	}
	_ = sess.Conditions.Apply(uid, def, 1, -1)
}

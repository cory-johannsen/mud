package gameserver

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cory-johannsen/mud/internal/game/crafting"
	"github.com/cory-johannsen/mud/internal/game/downtime"
	"github.com/cory-johannsen/mud/internal/game/session"
	"go.uber.org/zap"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// craftDowntimeDurationMinutesPerDay is the real-time minutes per downtime day for the
// craft activity. One downtime day = 2 real-time minutes.
const craftDowntimeDurationMinutesPerDay = 2

// handleDowntime processes a downtime subcommand from a player.
//
// Precondition: uid is a valid session UID; req is non-nil.
// Postcondition: Returns a ServerEvent describing the outcome; never returns a non-nil error.
func (s *GameServiceServer) handleDowntime(uid string, req *gamev1.DowntimeRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}

	sub := strings.ToLower(strings.TrimSpace(req.GetSubcommand()))

	if sub == "queue" {
		return s.handleDowntimeQueue(uid, sess, req.GetArgs())
	}

	switch sub {
	case "":
		return s.downtimeStatus(sess), nil
	case "list":
		return s.downtimeList(), nil
	case "cancel":
		return s.downtimeCancel(uid, sess), nil
	default:
		return s.downtimeStart(uid, sess, sub, req.GetArgs()), nil
	}
}

// handleDowntimeQueue dispatches downtime queue subcommands.
//
// Precondition: uid is a valid session; sess is non-nil; args is the remainder after "queue".
// Postcondition: Returns a ServerEvent describing the outcome.
func (s *GameServiceServer) handleDowntimeQueue(uid string, sess *session.PlayerSession, args string) (*gamev1.ServerEvent, error) {
	parts := strings.Fields(strings.TrimSpace(args))
	if len(parts) == 0 {
		return s.handleDowntimeQueueList(uid, sess)
	}
	sub := strings.ToLower(parts[0])
	rest := ""
	if len(parts) > 1 {
		rest = strings.Join(parts[1:], " ")
	}
	switch sub {
	case "list":
		return s.handleDowntimeQueueList(uid, sess)
	case "clear":
		return s.handleDowntimeQueueClear(uid, sess)
	case "remove":
		return s.handleDowntimeQueueRemove(uid, sess, rest)
	default:
		return s.handleDowntimeQueueAdd(uid, sess, sub, rest)
	}
}

// effectiveQueueLimit returns the player's downtime queue limit, defaulting to 3 (REQ-DTQ-14).
//
// Precondition: sess is non-nil.
// Postcondition: Returns >= 1.
func effectiveQueueLimit(sess *session.PlayerSession) int {
	if sess.DowntimeQueueLimit > 0 {
		return sess.DowntimeQueueLimit
	}
	return 3
}

// handleDowntimeQueueAdd adds an activity to the downtime queue.
//
// Precondition: uid is a valid session; alias is a downtime activity alias; activityArgs may be empty.
// Postcondition: On success, entry appended to queue; returns confirmation message.
func (s *GameServiceServer) handleDowntimeQueueAdd(uid string, sess *session.PlayerSession, alias, activityArgs string) (*gamev1.ServerEvent, error) {
	if s.downtimeQueueRepo == nil {
		return messageEvent("Downtime queue is not available."), nil
	}
	entries, err := s.downtimeQueueRepo.ListQueue(context.Background(), sess.CharacterID)
	if err != nil {
		return messageEvent("Failed to read queue."), nil
	}
	limit := effectiveQueueLimit(sess)
	if len(entries) >= limit {
		return messageEvent(fmt.Sprintf("Your downtime queue is full (%d/%d).", len(entries), limit)), nil
	}
	act, ok := downtime.ActivityByAlias(alias)
	if !ok {
		return messageEvent(fmt.Sprintf("Unknown downtime activity %q.", alias)), nil
	}
	if act.ID == "craft" && s.recipeReg != nil {
		if _, ok := s.recipeReg.Recipe(activityArgs); !ok {
			return messageEvent(fmt.Sprintf("Recipe %q not found.", activityArgs)), nil
		}
	}
	// REQ-DTQ-4: warn (do not fail) when analyze_tech or field_repair item is not currently in inventory.
	// Item presence is re-checked at start time; this is only a non-blocking advisory.
	if (act.ID == "analyze_tech" || act.ID == "field_repair") && sess.Backpack != nil && activityArgs != "" {
		if len(sess.Backpack.FindByItemDefID(activityArgs)) == 0 {
			s.pushMessageToUID(uid, fmt.Sprintf("Warning: %q is not currently in your inventory. It will be re-checked when the activity starts.", activityArgs))
		}
	}
	if err := s.downtimeQueueRepo.Enqueue(context.Background(), sess.CharacterID, act.ID, activityArgs); err != nil {
		return messageEvent("Failed to add to queue."), nil
	}
	return messageEvent(fmt.Sprintf("Queued: %s (position %d).", act.Name, len(entries)+1)), nil
}

// handleDowntimeQueueList returns the current queue with estimated start/end times.
//
// Precondition: uid is a valid session; sess is non-nil.
// Postcondition: Returns formatted queue listing or "empty" message.
func (s *GameServiceServer) handleDowntimeQueueList(uid string, sess *session.PlayerSession) (*gamev1.ServerEvent, error) {
	if s.downtimeQueueRepo == nil {
		return messageEvent("Downtime queue is not available."), nil
	}
	entries, err := s.downtimeQueueRepo.ListQueue(context.Background(), sess.CharacterID)
	if err != nil {
		return messageEvent("Failed to read queue."), nil
	}
	if len(entries) == 0 {
		return messageEvent("Your downtime queue is empty."), nil
	}
	limit := effectiveQueueLimit(sess)
	baseline := time.Now()
	if sess.DowntimeBusy && !sess.DowntimeCompletesAt.IsZero() {
		baseline = sess.DowntimeCompletesAt
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- Downtime Queue (%d/%d) ---\n", len(entries), limit))
	cursor := baseline
	for _, e := range entries {
		act, ok := downtime.ActivityByID(e.ActivityID)
		if !ok {
			sb.WriteString(fmt.Sprintf("  %d. %-20s  start:unknown  end:unknown\n", e.Position, e.ActivityID))
			continue
		}
		durationMin := downtimeActivityDuration(act, e.ActivityArgs, s.recipeReg)
		start := cursor
		end := cursor.Add(time.Duration(durationMin) * time.Minute)
		sb.WriteString(fmt.Sprintf("  %d. %-20s  start:%s  end:%s\n", e.Position, act.Name, start.Format("15:04"), end.Format("15:04")))
		cursor = end
	}
	return messageEvent(sb.String()), nil
}

// handleDowntimeQueueClear removes all queued activities without affecting the active activity.
// REQ-DTQ-16: does NOT cancel or modify the currently running activity.
//
// Precondition: uid is a valid session; sess is non-nil.
// Postcondition: queue cleared; active activity (if any) remains.
func (s *GameServiceServer) handleDowntimeQueueClear(uid string, sess *session.PlayerSession) (*gamev1.ServerEvent, error) {
	if s.downtimeQueueRepo == nil {
		return messageEvent("Downtime queue is not available."), nil
	}
	if err := s.downtimeQueueRepo.Clear(context.Background(), sess.CharacterID); err != nil {
		return messageEvent(fmt.Sprintf("Failed to clear queue: %v", err)), nil
	}
	return messageEvent("Downtime queue cleared. Active activity (if any) is unaffected."), nil
}

// handleDowntimeQueueRemove removes the entry at the given 1-based position from the queue.
//
// Precondition: uid is a valid session; sess is non-nil; posStr is the position argument string.
// Postcondition: On success, the entry at the given position is removed and remaining entries reindexed.
func (s *GameServiceServer) handleDowntimeQueueRemove(uid string, sess *session.PlayerSession, posStr string) (*gamev1.ServerEvent, error) {
	if s.downtimeQueueRepo == nil {
		return messageEvent("Downtime queue is not available."), nil
	}
	pos := 0
	fmt.Sscanf(posStr, "%d", &pos)
	if pos < 1 {
		return messageEvent("Usage: downtime queue remove <position>"), nil
	}
	if err := s.downtimeQueueRepo.RemoveAt(context.Background(), sess.CharacterID, pos); err != nil {
		return messageEvent(fmt.Sprintf("Failed to remove queue entry: %v", err)), nil
	}
	return messageEvent(fmt.Sprintf("Queue position %d removed.", pos)), nil
}

// downtimeStatus returns the current downtime activity status for a session.
//
// Precondition: sess is non-nil.
// Postcondition: Returns a message describing the current activity or "No active downtime activity."
func (s *GameServiceServer) downtimeStatus(sess *session.PlayerSession) *gamev1.ServerEvent {
	if !sess.DowntimeBusy || sess.DowntimeActivityID == "" {
		return messageEvent("No active downtime activity.")
	}
	act, ok := downtime.ActivityByID(sess.DowntimeActivityID)
	if !ok {
		return messageEvent(fmt.Sprintf("Unknown active activity: %s", sess.DowntimeActivityID))
	}
	remaining := time.Until(sess.DowntimeCompletesAt)
	if remaining < 0 {
		remaining = 0
	}
	mins := int(remaining.Minutes())
	secs := int(remaining.Seconds()) % 60
	return messageEvent(fmt.Sprintf("Active downtime: %s — %dm%02ds remaining.", act.Name, mins, secs))
}

// downtimeList returns a formatted list of all downtime activities.
//
// Precondition: none.
// Postcondition: Returns a ServerEvent listing all activities with alias, name, and duration.
func (s *GameServiceServer) downtimeList() *gamev1.ServerEvent {
	acts := downtime.AllActivities()
	var b strings.Builder
	b.WriteString("Downtime activities:\n")
	for _, a := range acts {
		dur := "varies"
		if a.DurationMinutes > 0 {
			dur = fmt.Sprintf("%dm", a.DurationMinutes)
		}
		b.WriteString(fmt.Sprintf("  %-12s %-28s %s\n", a.Alias, a.Name, dur))
	}
	return messageEvent(b.String())
}

// downtimeCancel cancels the current downtime activity if one is active.
//
// Precondition: uid is a valid session UID; sess is non-nil.
// Postcondition: sess.DowntimeBusy == false; sess.DowntimeActivityID == ""; repo cleared if non-nil.
func (s *GameServiceServer) downtimeCancel(uid string, sess *session.PlayerSession) *gamev1.ServerEvent {
	if !sess.DowntimeBusy {
		return messageEvent("You have no active downtime activity to cancel.")
	}
	actID := sess.DowntimeActivityID
	sess.DowntimeBusy = false
	sess.DowntimeActivityID = ""
	sess.DowntimeCompletesAt = time.Time{}
	sess.DowntimeMetadata = ""

	if s.downtimeRepo != nil && sess.CharacterID > 0 {
		_ = s.downtimeRepo.Clear(context.Background(), sess.CharacterID)
	}

	act, ok := downtime.ActivityByID(actID)
	if !ok {
		return messageEvent("Downtime activity cancelled.")
	}
	return messageEvent(fmt.Sprintf("%s cancelled.", act.Name))
}

// downtimeStart attempts to begin a downtime activity by alias.
//
// Precondition: uid is a valid session UID; sess is non-nil; alias is a lowercase command alias; activityArgs is the args string (may be empty).
// Postcondition: On success, sess.DowntimeBusy==true, sess.DowntimeActivityID is set, repo saved if non-nil.
func (s *GameServiceServer) downtimeStart(uid string, sess *session.PlayerSession, alias, activityArgs string) *gamev1.ServerEvent {
	// Resolve room tags.
	roomTags := ""
	if s.world != nil {
		if room, ok := s.world.GetRoom(sess.RoomID); ok && room.Properties != nil {
			roomTags = room.Properties["tags"]
		}
	}

	if errMsg := downtime.CanStart(alias, roomTags, sess.DowntimeBusy); errMsg != "" {
		return messageEvent(errMsg)
	}

	act, ok := downtime.ActivityByAlias(alias)
	if !ok {
		return messageEvent("Unknown downtime activity.")
	}

	// Gate forge_papers on having forgery_supplies in inventory. (REQ-DA-FORGE-1)
	if act.ID == "forge_papers" {
		if sess.Backpack == nil {
			return messageEvent("You need forgery supplies to begin forging papers.")
		}
		instances := sess.Backpack.FindByItemDefID("forgery_supplies")
		if len(instances) == 0 {
			return messageEvent("You need forgery supplies to begin forging papers.")
		}
		// Consume one forgery_supplies at activity start.
		if err := sess.Backpack.Remove(instances[0].InstanceID, 1); err != nil {
			return messageEvent("You need forgery supplies to begin forging papers.")
		}
	}

	// Gate craft on recipe existence and available materials. (REQ-CRAFT-DT-1)
	if act.ID == "craft" {
		if activityArgs == "" || s.recipeReg == nil {
			return messageEvent("Specify a recipe: downtime craft <recipe_id>.")
		}
		recipe, ok := s.recipeReg.Recipe(activityArgs)
		if !ok {
			return messageEvent(fmt.Sprintf("Recipe %q not found.", activityArgs))
		}
		// Validate materials.
		var missing []string
		for _, rm := range recipe.Materials {
			if sess.Materials[rm.ID] < rm.Quantity {
				missing = append(missing, rm.ID)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return messageEvent(fmt.Sprintf("Missing materials for %s: %s.", recipe.Name, strings.Join(missing, ", ")))
		}
		// Consume materials eagerly at start: DB write must succeed before in-memory deduction.
		if s.materialRepo != nil && len(recipe.Materials) > 0 {
			deductions := make(map[string]int, len(recipe.Materials))
			for _, rm := range recipe.Materials {
				deductions[rm.ID] = rm.Quantity
			}
			if err := s.materialRepo.DeductMany(context.Background(), sess.CharacterID, deductions); err != nil {
				return messageEvent("Failed to deduct materials. Please try again.")
			}
		}
		for _, rm := range recipe.Materials {
			sess.Materials[rm.ID] -= rm.Quantity
			if sess.Materials[rm.ID] <= 0 {
				delete(sess.Materials, rm.ID)
			}
		}
	}

	durationMin := downtimeActivityDuration(act, activityArgs, s.recipeReg)
	completesAt := time.Now().Add(time.Duration(durationMin) * time.Minute)

	sess.DowntimeBusy = true
	sess.DowntimeActivityID = act.ID
	sess.DowntimeCompletesAt = completesAt
	sess.DowntimeMetadata = activityArgs

	if s.downtimeRepo != nil && sess.CharacterID > 0 {
		state := postgres.DowntimeState{
			ActivityID:  act.ID,
			CompletesAt: completesAt,
			RoomID:      sess.RoomID,
		}
		_ = s.downtimeRepo.Save(context.Background(), sess.CharacterID, state)
	}

	return messageEvent(fmt.Sprintf(
		"You begin %s. Activity will complete in %d minute(s).",
		act.Name, durationMin,
	))
}

// resolveDowntimeActivity completes the player's active downtime activity, clears busy state,
// and persists the cleared state to the repo if available.
//
// Precondition: uid is a valid player UID; sess is non-nil.
// Postcondition: If DowntimeBusy was true, all four downtime fields are cleared and repo cleared if non-nil.
//   If DowntimeBusy was already false, this is a no-op.
func (s *GameServiceServer) resolveDowntimeActivity(uid string, sess *session.PlayerSession) {
	if !sess.DowntimeBusy {
		return
	}
	actID := sess.DowntimeActivityID
	sess.DowntimeBusy = false
	sess.DowntimeActivityID = ""
	sess.DowntimeCompletesAt = time.Time{}
	sess.DowntimeMetadata = ""

	if s.downtimeRepo != nil && sess.CharacterID > 0 {
		_ = s.downtimeRepo.Clear(context.Background(), sess.CharacterID)
	}

	if s.sessions != nil {
		s.resolveDowntimeActivityDispatch(uid, actID, sess)
	}
}

// checkDowntimeCompletion checks if the player's active downtime activity has elapsed
// and resolves it if so.  After resolution, auto-starts the next queued activity (REQ-DTQ).
//
// Precondition: uid is a valid player UID.
// Postcondition: If DowntimeBusy and CompletesAt has elapsed, resolves activity, clears busy state,
//
//	and starts the next queued activity if one is available.
func (s *GameServiceServer) checkDowntimeCompletion(uid string) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok || !sess.DowntimeBusy {
		return
	}
	if time.Now().Before(sess.DowntimeCompletesAt) {
		return
	}
	s.resolveDowntimeActivity(uid, sess)
	s.startNext(uid) // REQ-DTQ: auto-start next queued activity
}

// startNext pops and starts the next eligible queued activity for uid.
// Calls itself recursively if the head item is invalid (REQ-DTQ-10).
// Termination is guaranteed: PopHead removes one item per call; the queue is finite.
//
// Precondition: uid is a valid player session; downtimeQueueRepo may be nil (no-op if nil).
// Postcondition: either the next valid queued activity is started, or all invalid entries
//
//	are skipped until the queue is empty.
func (s *GameServiceServer) startNext(uid string) {
	if s.downtimeQueueRepo == nil {
		return
	}
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok || sess.DowntimeBusy {
		return
	}

	entry, err := s.downtimeQueueRepo.PopHead(context.Background(), sess.CharacterID)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("startNext: PopHead error", zap.Error(err))
		}
		return
	}
	if entry == nil {
		return // queue empty; no notification needed
	}

	act, ok := downtime.ActivityByID(entry.ActivityID)
	if !ok {
		s.pushMessageToUID(uid, fmt.Sprintf("Skipped queued activity %q: unknown.", entry.ActivityID))
		s.startNext(uid)
		return
	}

	// Validate room eligibility (REQ-DTQ-7).
	roomTags := ""
	if s.world != nil {
		if room, roomOK := s.world.GetRoom(sess.RoomID); roomOK && room.Properties != nil {
			roomTags = room.Properties["tags"]
		}
	}
	if errMsg := downtime.CanStart(act.Alias, roomTags, false); errMsg != "" {
		s.pushMessageToUID(uid, fmt.Sprintf("Skipped queued activity %s: %s", act.Name, errMsg))
		s.startNext(uid)
		return
	}

	// REQ-DTQ-8/9 (DEFERRED): For craft activities, materials should be deducted at this point
	// via downtimePreStartCraft(uid, sess, entry.ActivityArgs), and if deduction fails the
	// activity should be skipped via s.startNext(uid). This requires downtimePreStartCraft
	// to be implemented in the downtime plan (it was not present when this plan was executed).
	// Tracked in docs/superpowers/plans/2026-03-22-downtime.md.

	durationMin := downtimeActivityDuration(act, entry.ActivityArgs, s.recipeReg)
	completesAt := time.Now().Add(time.Duration(durationMin) * time.Minute)
	sess.DowntimeActivityID = act.ID
	sess.DowntimeCompletesAt = completesAt
	sess.DowntimeBusy = true
	sess.DowntimeMetadata = entry.ActivityArgs

	if s.downtimeRepo != nil && sess.CharacterID > 0 {
		state := postgres.DowntimeState{
			ActivityID:  act.ID,
			CompletesAt: completesAt,
			RoomID:      sess.RoomID,
		}
		_ = s.downtimeRepo.Save(context.Background(), sess.CharacterID, state)
	}

	s.pushMessageToUID(uid, fmt.Sprintf("Starting: %s.", act.Name))
}

// restoreDowntimeState loads persisted downtime state at login and restores or resolves it.
//
// Precondition: sess is a valid, newly-created PlayerSession; characterID > 0.
// Postcondition: If an active downtime row exists in DB, restores busy state;
//
//	if the activity has elapsed, resolves it immediately.
func (s *GameServiceServer) restoreDowntimeState(ctx context.Context, uid string, sess *session.PlayerSession, characterID int64) {
	if s.downtimeRepo == nil || characterID <= 0 {
		return
	}
	dtState, err := s.downtimeRepo.Load(ctx, characterID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("restoreDowntimeState: failed to load downtime state",
				zap.Int64("character_id", characterID), zap.Error(err))
		}
		return
	}
	if dtState == nil {
		return
	}
	sess.DowntimeActivityID = dtState.ActivityID
	sess.DowntimeBusy = true
	sess.DowntimeMetadata = dtState.Metadata
	sess.DowntimeCompletesAt = dtState.CompletesAt
	if time.Now().After(dtState.CompletesAt) {
		// Activity elapsed while offline — resolve immediately.
		s.resolveDowntimeActivity(uid, sess)
		// Process any queued activities that also elapsed while offline (REQ-DTQ-11).
		s.resolveOfflineQueue(uid, sess, dtState.CompletesAt)
	}
}

// resolveOfflineQueue processes queued activities that completed while the player was offline.
// Called from the reconnect path after the active activity has been resolved.
//
// Precondition: uid is a valid player session; sess is non-nil; cursor is the time baseline
//   (typically when the previously active activity completed).
// Postcondition: Elapsed queued activities are resolved; the first future-eligible activity
//   is started as the new active activity. Summary message sent if any activity was processed.
func (s *GameServiceServer) resolveOfflineQueue(uid string, sess *session.PlayerSession, cursor time.Time) {
	if s.downtimeQueueRepo == nil {
		return
	}
	queueEntries, err := s.downtimeQueueRepo.ListQueue(context.Background(), sess.CharacterID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("resolveOfflineQueue: failed to list queue", zap.String("uid", uid), zap.Error(err))
		}
		return
	}
	if len(queueEntries) == 0 {
		return
	}

	roomTags := ""
	if s.world != nil {
		if room, roomOK := s.world.GetRoom(sess.RoomID); roomOK && room.Properties != nil {
			roomTags = room.Properties["tags"]
		}
	}

	completed := 0
	skipped := 0

	for range queueEntries {
		entry, popErr := s.downtimeQueueRepo.PopHead(context.Background(), sess.CharacterID)
		if popErr != nil || entry == nil {
			break
		}

		act, ok := downtime.ActivityByID(entry.ActivityID)
		if !ok {
			s.pushMessageToUID(uid, fmt.Sprintf("Skipped (offline): unknown activity %q.", entry.ActivityID))
			skipped++
			continue
		}

		durationMin := downtimeActivityDuration(act, entry.ActivityArgs, s.recipeReg)
		hypotheticalEnd := cursor.Add(time.Duration(durationMin) * time.Minute)

		if time.Now().Before(hypotheticalEnd) {
			// This activity has not elapsed yet — start it as the new active activity if eligible.
			// If the room is ineligible, stop processing: remaining entries have not been reached yet.
			if errMsg := downtime.CanStart(act.Alias, roomTags, false); errMsg != "" {
				s.pushMessageToUID(uid, fmt.Sprintf("Skipped (offline): %s — %s", act.Name, errMsg))
				skipped++
				break
			}
			sess.DowntimeActivityID = act.ID
			sess.DowntimeCompletesAt = hypotheticalEnd
			sess.DowntimeBusy = true
			sess.DowntimeMetadata = entry.ActivityArgs
			if s.downtimeRepo != nil && sess.CharacterID > 0 {
				state := postgres.DowntimeState{
					ActivityID:  act.ID,
					CompletesAt: hypotheticalEnd,
					RoomID:      sess.RoomID,
				}
				_ = s.downtimeRepo.Save(context.Background(), sess.CharacterID, state)
			}
			break
		}

		// Activity elapsed offline — validate and resolve.
		if errMsg := downtime.CanStart(act.Alias, roomTags, false); errMsg != "" {
			s.pushMessageToUID(uid, fmt.Sprintf("Skipped (offline): %s — %s", act.Name, errMsg))
			skipped++
			cursor = hypotheticalEnd
			continue
		}

		sess.DowntimeActivityID = act.ID
		sess.DowntimeBusy = true
		sess.DowntimeMetadata = entry.ActivityArgs
		sess.DowntimeCompletesAt = hypotheticalEnd
		s.resolveDowntimeActivity(uid, sess)
		s.pushMessageToUID(uid, "(offline)")
		completed++
		cursor = hypotheticalEnd
	}

	if completed > 0 || skipped > 0 {
		s.pushMessageToUID(uid, fmt.Sprintf("While you were away: %d activities completed, %d skipped.", completed, skipped))
	}
}

// downtimeActivityDuration returns the real-time duration in minutes for an activity.
// For craft, the duration is derived from the recipe's DowntimeDays if available.
//
// Precondition: act is a valid Activity; activityArgs and recipeReg may be zero/nil.
// Postcondition: Returns a positive integer duration in minutes.
func downtimeActivityDuration(act downtime.Activity, activityArgs string, recipeReg *crafting.RecipeRegistry) int {
	if act.DurationMinutes > 0 {
		return act.DurationMinutes
	}
	// Stub durations for computed-at-start activities.
	switch act.ID {
	case "craft":
		if recipeReg != nil && activityArgs != "" {
			if recipe, ok := recipeReg.Recipe(activityArgs); ok {
				days := recipe.DowntimeDays()
				if days < 1 {
					days = 1
				}
				return days * craftDowntimeDurationMinutesPerDay
			}
		}
		return 10
	case "retrain":
		return 8
	default:
		return 5
	}
}

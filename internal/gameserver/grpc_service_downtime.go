package gameserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cory-johannsen/mud/internal/game/downtime"
	"github.com/cory-johannsen/mud/internal/game/session"
	"go.uber.org/zap"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

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
		return s.downtimeStart(uid, sess, sub), nil
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
		pos := 0
		fmt.Sscanf(rest, "%d", &pos)
		if pos < 1 {
			return messageEvent("Usage: downtime queue remove <position>"), nil
		}
		if err := s.downtimeQueueRepo.RemoveAt(context.Background(), sess.CharacterID, pos); err != nil {
			return messageEvent(fmt.Sprintf("Failed to remove queue entry: %v", err)), nil
		}
		return messageEvent(fmt.Sprintf("Queue position %d removed.", pos)), nil
	default:
		return s.handleDowntimeQueueAdd(uid, sess, sub, rest)
	}
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
	limit := sess.DowntimeQueueLimit
	if limit <= 0 {
		limit = 3
	}
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
	limit := sess.DowntimeQueueLimit
	if limit <= 0 {
		limit = 3
	}
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
		durationMin := downtimeActivityDuration(act)
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
// Precondition: uid is a valid session UID; sess is non-nil; alias is a lowercase command alias.
// Postcondition: On success, sess.DowntimeBusy==true, sess.DowntimeActivityID is set, repo saved if non-nil.
func (s *GameServiceServer) downtimeStart(uid string, sess *session.PlayerSession, alias string) *gamev1.ServerEvent {
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

	durationMin := downtimeActivityDuration(act)
	completesAt := time.Now().Add(time.Duration(durationMin) * time.Minute)

	sess.DowntimeBusy = true
	sess.DowntimeActivityID = act.ID
	sess.DowntimeCompletesAt = completesAt
	sess.DowntimeMetadata = ""

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

	durationMin := downtimeActivityDuration(act)
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
	}
}

// downtimeActivityDuration returns the real-time duration in minutes for an activity.
// For activities with DurationMinutes==0 (computed at start), stubs are used pending Task 9.
//
// Precondition: act is a valid Activity.
// Postcondition: Returns a positive integer duration in minutes.
func downtimeActivityDuration(act downtime.Activity) int {
	if act.DurationMinutes > 0 {
		return act.DurationMinutes
	}
	// Stub durations for computed-at-start activities (full resolver in Task 9).
	switch act.ID {
	case "craft":
		return 10
	case "retrain":
		return 8
	default:
		return 5
	}
}

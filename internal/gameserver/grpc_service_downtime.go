package gameserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cory-johannsen/mud/internal/game/downtime"
	"github.com/cory-johannsen/mud/internal/game/session"
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
	sess.DowntimeBusy = false
	sess.DowntimeActivityID = ""
	sess.DowntimeCompletesAt = time.Time{}
	sess.DowntimeMetadata = ""

	if s.downtimeRepo != nil && sess.CharacterID > 0 {
		_ = s.downtimeRepo.Clear(context.Background(), sess.CharacterID)
	}
}

// checkDowntimeCompletion checks if the player's active downtime activity has elapsed
// and resolves it if so.
//
// Precondition: uid is a valid player UID.
// Postcondition: If DowntimeBusy and CompletesAt has elapsed, resolves activity and clears busy state.
func (s *GameServiceServer) checkDowntimeCompletion(uid string) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok || !sess.DowntimeBusy {
		return
	}
	if time.Now().Before(sess.DowntimeCompletesAt) {
		return
	}
	s.resolveDowntimeActivity(uid, sess)
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
	if err != nil || dtState == nil {
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

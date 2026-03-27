package gameserver

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// findJobTrainerInRoom locates a job_trainer NPC by name in roomID.
//
// Precondition: roomID and npcName are non-empty.
// Postcondition: Returns (inst, "") on success; (nil, errMsg) on failure.
func (s *GameServiceServer) findJobTrainerInRoom(roomID, npcName string) (*npc.Instance, string) {
	inst := s.npcMgr.FindInRoom(roomID, npcName)
	if inst == nil {
		return nil, fmt.Sprintf("You don't see %q here.", npcName)
	}
	if inst.NPCType != "job_trainer" {
		return nil, fmt.Sprintf("%s is not a job trainer.", inst.Name())
	}
	if inst.Cowering {
		return nil, fmt.Sprintf("%s is cowering in fear and won't respond right now.", inst.Name())
	}
	return inst, ""
}

// handleTrainJob processes a player's request to learn a new job from a trainer.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// On success, the job is added to sess.Jobs at level 1; if it is the first job,
// sess.ActiveJobID is set to the trained job ID (REQ-NPC-9).
func (s *GameServiceServer) handleTrainJob(uid string, req *gamev1.TrainJobRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findJobTrainerInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	// Enemy faction non-combat NPC check (REQ-FA-28).
	if s.factionSvc != nil && s.factionSvc.IsEnemyOf(sess, inst.FactionID) {
		return messageEvent(fmt.Sprintf("%s eyes you coldly. 'We don't serve your kind here.'", inst.Name())), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.JobTrainer == nil {
		return messageEvent("This trainer has no jobs configured."), nil
	}
	jobID := req.GetJobId()
	var trainable *npc.TrainableJob
	for i := range tmpl.JobTrainer.OfferedJobs {
		if tmpl.JobTrainer.OfferedJobs[i].JobID == jobID {
			trainable = &tmpl.JobTrainer.OfferedJobs[i]
			break
		}
	}
	if trainable == nil {
		return messageEvent(fmt.Sprintf("%s doesn't offer training for %q.", inst.Name(), jobID)), nil
	}
	playerAttrs := buildPlayerAttrs(sess)
	playerSkills := sess.Skills
	if playerSkills == nil {
		playerSkills = map[string]string{}
	}
	playerJobs := sess.Jobs
	if playerJobs == nil {
		playerJobs = map[string]int{}
	}
	playerFeats := make([]string, 0, len(sess.PassiveFeats))
	for featID := range sess.PassiveFeats {
		playerFeats = append(playerFeats, featID)
	}
	if err := npc.CheckJobPrerequisites(*trainable, sess.Level, playerJobs, playerAttrs, playerSkills, playerFeats); err != nil {
		return messageEvent(err.Error()), nil
	}
	if sess.Currency < trainable.TrainingCost {
		return messageEvent(fmt.Sprintf("Training costs %d credits but you only have %d.", trainable.TrainingCost, sess.Currency)), nil
	}
	sess.Currency -= trainable.TrainingCost
	if sess.Jobs == nil {
		sess.Jobs = map[string]int{}
	}
	sess.Jobs[jobID] = 1
	if sess.ActiveJobID == "" {
		sess.ActiveJobID = jobID
	}
	if s.jobRegistry != nil {
		heldJobs := s.resolveHeldJobs(sess)
		_, newFeats, _ := character.ComputeHeldJobBenefitsWithDrawbacks(heldJobs)
		if sess.PassiveFeats == nil {
			sess.PassiveFeats = make(map[string]bool)
		}
		for _, feat := range newFeats {
			if _, already := sess.PassiveFeats[feat]; !already {
				if s.featRegistry != nil {
					if f, ok := s.featRegistry.Feat(feat); ok && !f.Active {
						sess.PassiveFeats[feat] = true
					}
				}
			}
		}
		// Apply passive drawback conditions for the newly trained job (REQ-JD-8).
		if newJob, ok := s.jobRegistry.Job(jobID); ok && sess.Conditions != nil && s.condRegistry != nil {
			for _, db := range newJob.Drawbacks {
				if db.Type != "passive" || db.ConditionID == "" {
					continue
				}
				source := "drawback:" + jobID
				if def, ok := s.condRegistry.Get(db.ConditionID); ok {
					_ = sess.Conditions.ApplyTagged(uid, def, 1, -1, source)
				}
			}
		}
	}
	if s.charSaver != nil && sess.CharacterID > 0 {
		if saveErr := s.charSaver.SaveJobs(context.Background(), sess.CharacterID, sess.Jobs, sess.ActiveJobID); saveErr != nil {
			s.logger.Warn("failed to save jobs after training",
				zap.String("uid", uid),
				zap.Int64("character_id", sess.CharacterID),
				zap.Error(saveErr),
			)
		}
	}
	// Persist the new job to character_jobs table (REQ-JD-4).
	if s.characterJobsRepo != nil && sess.CharacterID > 0 {
		if err := s.characterJobsRepo.AddJob(context.Background(), sess.CharacterID, jobID); err != nil {
			s.logger.Warn("failed to persist job to character_jobs",
				zap.String("uid", uid),
				zap.Int64("character_id", sess.CharacterID),
				zap.String("job_id", jobID),
				zap.Error(err),
			)
		}
	}
	// Update HeldJobs in-memory (REQ-JD-4).
	sess.HeldJobs = append(sess.HeldJobs, jobID)
	return messageEvent(fmt.Sprintf(
		"%s trains you in %q. Cost: %d credits. Your jobs: %s.",
		inst.Name(), jobID, trainable.TrainingCost, formatJobList(sess.Jobs, sess.ActiveJobID),
	)), nil
}

// handleListJobs lists all jobs held by the player with level and active marker.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleListJobs(uid string, _ *gamev1.ListJobsRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	if len(sess.Jobs) == 0 {
		return messageEvent("You have no jobs trained yet. Find a job trainer to begin."), nil
	}
	var sb strings.Builder
	sb.WriteString("Your jobs:\n")
	ids := make([]string, 0, len(sess.Jobs))
	for id := range sess.Jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		marker := ""
		if id == sess.ActiveJobID {
			marker = " [active]"
		}
		sb.WriteString(fmt.Sprintf("  %-20s level %d%s\n", id, sess.Jobs[id], marker))
	}
	return messageEvent(sb.String()), nil
}

// handleSetJob changes the player's active job. REQ-NPC-17: available from any room.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// On success, sess.ActiveJobID is updated.
func (s *GameServiceServer) handleSetJob(uid string, req *gamev1.SetJobRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	jobID := req.GetJobId()
	if _, has := sess.Jobs[jobID]; !has {
		return messageEvent(fmt.Sprintf("You have not trained %q. Use 'jobs' to see your trained jobs.", jobID)), nil
	}
	sess.ActiveJobID = jobID
	if s.charSaver != nil && sess.CharacterID > 0 {
		if saveErr := s.charSaver.SaveJobs(context.Background(), sess.CharacterID, sess.Jobs, sess.ActiveJobID); saveErr != nil {
			s.logger.Warn("failed to save jobs after set-job",
				zap.String("uid", uid),
				zap.Int64("character_id", sess.CharacterID),
				zap.Error(saveErr),
			)
		}
	}
	return messageEvent(fmt.Sprintf("Active job set to %q (level %d).", jobID, sess.Jobs[jobID])), nil
}

// formatJobList returns a compact comma-separated listing of job:level pairs with active marker.
func formatJobList(jobs map[string]int, activeID string) string {
	ids := make([]string, 0, len(jobs))
	for id := range jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		entry := fmt.Sprintf("%s(L%d)", id, jobs[id])
		if id == activeID {
			entry += " [active]"
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, ", ")
}

// resolveHeldJobs returns Job objects for all jobs held in the session.
//
// Precondition: sess must not be nil.
// Postcondition: Returns a slice of *ruleset.Job for all jobs found in the registry.
func (s *GameServiceServer) resolveHeldJobs(sess *session.PlayerSession) []*ruleset.Job {
	var jobs []*ruleset.Job
	for jobID := range sess.Jobs {
		if j, ok := s.jobRegistry.Job(jobID); ok {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

// buildPlayerAttrs constructs the attribute map from a player session's AbilityScores.
func buildPlayerAttrs(sess *session.PlayerSession) map[string]int {
	return map[string]int{
		"brutality": sess.Abilities.Brutality,
		"grit":      sess.Abilities.Grit,
		"quickness": sess.Abilities.Quickness,
		"reasoning": sess.Abilities.Reasoning,
		"savvy":     sess.Abilities.Savvy,
		"flair":     sess.Abilities.Flair,
	}
}

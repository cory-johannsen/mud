package gameserver

import (
	"sort"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// buildHealerView constructs a HealerView ServerEvent for a healer NPC examine.
//
// Precondition: uid identifies an active player session; inst is a healer NPC.
// Postcondition: Returns a non-nil ServerEvent wrapping HealerView; error is always nil.
func (s *GameServiceServer) buildHealerView(uid string, inst *npc.Instance) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	pricePerHP := int32(0)
	capacityRemaining := int32(0)
	if tmpl != nil && tmpl.Healer != nil {
		pricePerHP = int32(tmpl.Healer.PricePerHP)
		state := s.healerStateFor(inst.ID)
		if state == nil {
			s.initHealerRuntimeState(inst)
			state = s.healerStateFor(inst.ID)
		}
		if state != nil {
			cap := tmpl.Healer.DailyCapacity - state.CapacityUsed
			if cap < 0 {
				cap = 0
			}
			capacityRemaining = int32(cap)
		}
	}
	missingHP := int32(sess.MaxHP - sess.CurrentHP)
	if missingHP < 0 {
		missingHP = 0
	}
	healable := missingHP
	if capacityRemaining < healable {
		healable = capacityRemaining
	}
	fullHealCost := pricePerHP * healable

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_HealerView{
			HealerView: &gamev1.HealerView{
				NpcName:           inst.Name(),
				Description:       inst.Description,
				PricePerHp:        pricePerHP,
				MissingHp:         missingHP,
				FullHealCost:      fullHealCost,
				CapacityRemaining: capacityRemaining,
				PlayerCurrency:    int32(sess.Currency),
				CurrentHp:         int32(sess.CurrentHP),
				MaxHp:             int32(sess.MaxHP),
			},
		},
	}, nil
}

// buildTrainerView constructs a TrainerView ServerEvent for a job_trainer NPC examine.
//
// Precondition: uid identifies an active player session; inst is a job_trainer NPC.
// Postcondition: Returns a non-nil ServerEvent wrapping TrainerView; error is always nil.
func (s *GameServiceServer) buildTrainerView(uid string, inst *npc.Instance) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	var jobEntries []*gamev1.JobOfferEntry
	if tmpl != nil && tmpl.JobTrainer != nil {
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
		offered := make([]npc.TrainableJob, len(tmpl.JobTrainer.OfferedJobs))
		copy(offered, tmpl.JobTrainer.OfferedJobs)
		sort.Slice(offered, func(i, j int) bool { return offered[i].JobID < offered[j].JobID })

		for _, job := range offered {
			entry := &gamev1.JobOfferEntry{
				JobId:         job.JobID,
				JobName:       job.JobID, // use ID as name; job registry lookup below
				TrainingCost:  int32(job.TrainingCost),
				AlreadyTrained: playerJobs[job.JobID] > 0,
			}
			if s.jobRegistry != nil {
				if j, ok := s.jobRegistry.Job(job.JobID); ok && j.Name != "" {
					entry.JobName = j.Name
				}
			}
			if entry.AlreadyTrained {
				entry.Available = false
				entry.UnavailableReason = "already trained"
			} else {
				if err := npc.CheckJobPrerequisites(job, sess.Level, playerJobs, playerAttrs, playerSkills, playerFeats); err != nil {
					entry.Available = false
					entry.UnavailableReason = err.Error()
				} else {
					entry.Available = true
				}
			}
			jobEntries = append(jobEntries, entry)
		}
	}

	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_TrainerView{
			TrainerView: &gamev1.TrainerView{
				NpcName:        inst.Name(),
				Description:    inst.Description,
				Jobs:           jobEntries,
				PlayerCurrency: int32(sess.Currency),
			},
		},
	}, nil
}

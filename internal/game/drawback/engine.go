package drawback

import (
	"time"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

// Drawback type constants.
const (
	DrawbackPassive     = "passive"
	DrawbackSituational = "situational"
)

// Trigger IDs for situational drawbacks (REQ-JD-10).
const (
	TriggerOnLeaveCombatWithoutKill           = "on_leave_combat_without_kill"
	TriggerOnTakeDamageInOneHitAboveThreshold = "on_take_damage_in_one_hit_above_threshold"
	TriggerOnFailSkillCheck                   = "on_fail_skill_check"
	TriggerOnEnterRoomDangerLevel             = "on_enter_room_danger_level"
)

// ConditionDefLookup provides condition definitions by ID.
type ConditionDefLookup interface {
	Get(id string) (*condition.ConditionDef, bool)
}

// Engine evaluates situational drawback triggers and applies their conditions.
//
// Precondition: condDefs must not be nil.
type Engine struct {
	condDefs ConditionDefLookup
}

// NewEngine creates a new Engine.
//
// Precondition: condDefs must not be nil.
func NewEngine(condDefs ConditionDefLookup) *Engine {
	return &Engine{condDefs: condDefs}
}

// FireTrigger evaluates all held jobs' drawbacks for the given trigger and applies
// any matching conditions to activeSet.
//
// Precondition: trigger is one of the Trigger* constants; jobs and activeSet are non-nil.
// Postcondition: matching situational drawback conditions are applied to activeSet
// with source "drawback:<job_id>" and a real-time ExpiresAt derived from now + duration.
func (e *Engine) FireTrigger(uid string, trigger string, jobs []*ruleset.Job, activeSet *condition.ActiveSet, now time.Time) {
	for _, job := range jobs {
		for _, db := range job.Drawbacks {
			if db.Type != DrawbackSituational || db.Trigger != trigger {
				continue
			}
			def, ok := e.condDefs.Get(db.EffectConditionID)
			if !ok {
				continue
			}
			dur := time.Hour // default 1h
			if db.Duration != "" {
				if parsed, err := time.ParseDuration(db.Duration); err == nil {
					dur = parsed
				}
			}
			expiresAt := now.Add(dur)
			source := "drawback:" + job.ID
			_ = activeSet.ApplyTaggedWithExpiry(uid, def, 1, source, expiresAt)
		}
	}
}

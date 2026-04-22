// internal/game/reaction/ready_test.go
package reaction_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/reaction"
)

// allowedTriggers enumerates the three triggers a player may bind a Ready action to.
var allowedTriggers = []reaction.ReactionTriggerType{
	reaction.TriggerOnEnemyEntersRoom,
	reaction.TriggerOnEnemyMoveAdjacent,
	reaction.TriggerOnAllyDamaged,
}

// uidGen produces short, bounded UID strings so the generator explores variation
// without producing pathological values.
var uidGen = rapid.StringMatching(`[a-z][a-z0-9]{0,5}`)

// TestProperty_ReadyRegistry_ConsumeAtomic asserts that after adding n entries
// for the same (UID, trigger), exactly n calls to Consume return non-nil and
// any subsequent call returns nil.
func TestProperty_ReadyRegistry_ConsumeAtomic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 10).Draw(t, "n")
		trigger := rapid.SampledFrom(allowedTriggers).Draw(t, "trigger")
		uid := uidGen.Draw(t, "uid")
		round := rapid.IntRange(1, 5).Draw(t, "round")

		r := reaction.NewReadyRegistry()
		for i := 0; i < n; i++ {
			r.Add(reaction.ReadyEntry{UID: uid, Trigger: trigger, RoundSet: round})
		}

		for i := 0; i < n; i++ {
			e := r.Consume(uid, trigger, "")
			if e == nil {
				t.Fatalf("Consume #%d/%d: expected entry, got nil", i+1, n)
			}
		}
		if extra := r.Consume(uid, trigger, ""); extra != nil {
			t.Fatalf("Consume #%d: expected nil after %d entries were consumed, got entry", n+1, n)
		}
	})
}

// TestProperty_ReadyRegistry_ExpireRoundClearsOnlyThatRound asserts that after
// ExpireRound(R), no entry with RoundSet == R remains consumable, while every
// original non-R entry is still consumable exactly once.
func TestProperty_ReadyRegistry_ExpireRoundClearsOnlyThatRound(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		roundGen := rapid.IntRange(1, 5)
		victim := roundGen.Draw(t, "victim")

		type spec struct {
			uid     string
			trigger reaction.ReactionTriggerType
			round   int
		}
		specGen := rapid.Custom(func(t *rapid.T) spec {
			return spec{
				uid:     uidGen.Draw(t, "uid"),
				trigger: rapid.SampledFrom(allowedTriggers).Draw(t, "trigger"),
				round:   roundGen.Draw(t, "round"),
			}
		})
		specs := rapid.SliceOfN(specGen, 1, 10).Draw(t, "specs")

		r := reaction.NewReadyRegistry()
		for _, s := range specs {
			r.Add(reaction.ReadyEntry{UID: s.uid, Trigger: s.trigger, RoundSet: s.round})
		}

		r.ExpireRound(victim)

		// Group the non-victim specs by (uid, trigger) so we know how many
		// Consume calls should succeed for each key.
		type key struct {
			uid     string
			trigger reaction.ReactionTriggerType
		}
		remaining := map[key]int{}
		for _, s := range specs {
			if s.round == victim {
				continue
			}
			remaining[key{s.uid, s.trigger}]++
		}

		// Every non-victim entry should consume exactly once.
		for k, count := range remaining {
			for i := 0; i < count; i++ {
				if e := r.Consume(k.uid, k.trigger, ""); e == nil {
					t.Fatalf("non-victim entry %+v (occurrence %d/%d) should still be consumable", k, i+1, count)
				}
			}
			if extra := r.Consume(k.uid, k.trigger, ""); extra != nil {
				t.Fatalf("non-victim entry %+v: expected nil after %d consumes, got entry", k, count)
			}
		}

		// No victim-round entry should remain. Attempting to consume any of the
		// original victim-round (uid, trigger) pairs must return nil.
		for _, s := range specs {
			if s.round != victim {
				continue
			}
			// If another non-victim spec shares this (uid, trigger), those have
			// already been fully consumed above, so the registry should have no
			// matching entry for this pair.
			if e := r.Consume(s.uid, s.trigger, ""); e != nil {
				t.Fatalf("victim-round entry %+v should have been expired", s)
			}
		}
	})
}

// TestProperty_ReadyRegistry_CancelRemovesAllForUID asserts that after
// Cancel(victim), no Consume succeeds for the victim UID, while every
// non-victim UID's entry remains consumable.
func TestProperty_ReadyRegistry_CancelRemovesAllForUID(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Draw a distinct set of UIDs (2..5). We dedupe on the fly rather than
		// relying on rapid to produce unique strings.
		raw := rapid.SliceOfN(uidGen, 2, 5).Draw(t, "uids")
		seen := map[string]bool{}
		uids := make([]string, 0, len(raw))
		for _, u := range raw {
			if seen[u] {
				continue
			}
			seen[u] = true
			uids = append(uids, u)
		}
		if len(uids) < 2 {
			t.Skip("need at least two distinct uids")
		}
		victimIdx := rapid.IntRange(0, len(uids)-1).Draw(t, "victimIdx")
		victim := uids[victimIdx]
		trigger := rapid.SampledFrom(allowedTriggers).Draw(t, "trigger")

		r := reaction.NewReadyRegistry()
		for _, u := range uids {
			r.Add(reaction.ReadyEntry{UID: u, Trigger: trigger, RoundSet: 1})
		}

		r.Cancel(victim)

		if e := r.Consume(victim, trigger, ""); e != nil {
			t.Fatalf("Cancel(%q) should have removed victim's entry", victim)
		}
		for _, u := range uids {
			if u == victim {
				continue
			}
			if e := r.Consume(u, trigger, ""); e == nil {
				t.Fatalf("non-victim %q entry should still be consumable after Cancel(%q)", u, victim)
			}
		}
	})
}

// TestProperty_ReadyRegistry_TriggerTgtFiltering asserts that a single entry
// with TriggerTgt=T is consumed by a Consume(uid, trigger, sourceUID) call iff
// T == "" OR T == sourceUID; in the success case the entry is removed and a
// second Consume returns nil; in the failure case it remains and a matching
// Consume still succeeds.
func TestProperty_ReadyRegistry_TriggerTgtFiltering(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		uid := uidGen.Draw(t, "uid")
		trigger := rapid.SampledFrom(allowedTriggers).Draw(t, "trigger")
		// Allow an empty TriggerTgt (unrestricted) about a third of the time so
		// both arms of the iff are exercised.
		triggerTgt := rapid.OneOf(
			rapid.Just(""),
			uidGen,
		).Draw(t, "triggerTgt")
		sourceUID := rapid.OneOf(
			rapid.Just(""),
			uidGen,
		).Draw(t, "sourceUID")

		r := reaction.NewReadyRegistry()
		r.Add(reaction.ReadyEntry{
			UID:        uid,
			Trigger:    trigger,
			TriggerTgt: triggerTgt,
			RoundSet:   1,
		})

		shouldMatch := triggerTgt == "" || triggerTgt == sourceUID

		e := r.Consume(uid, trigger, sourceUID)
		if shouldMatch {
			if e == nil {
				t.Fatalf("Consume(uid=%q, trigger=%v, src=%q) with TriggerTgt=%q should have matched", uid, trigger, sourceUID, triggerTgt)
			}
			// Entry should have been removed.
			if again := r.Consume(uid, trigger, sourceUID); again != nil {
				t.Fatalf("second Consume should return nil after entry was consumed")
			}
		} else {
			if e != nil {
				t.Fatalf("Consume(uid=%q, trigger=%v, src=%q) with TriggerTgt=%q should NOT have matched", uid, trigger, sourceUID, triggerTgt)
			}
			// Entry should still be present and consumable by a matching source.
			if still := r.Consume(uid, trigger, triggerTgt); still == nil {
				t.Fatalf("entry should still be present and consumable via sourceUID=%q", triggerTgt)
			}
		}
	})
}

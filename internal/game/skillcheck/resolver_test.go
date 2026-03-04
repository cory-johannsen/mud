package skillcheck

import (
	"testing"

	"pgregory.net/rapid"
)

// TestProficiencyBonus verifies exact bonus values for all known ranks and unknown rank.
func TestProficiencyBonus(t *testing.T) {
	cases := []struct {
		rank string
		want int
	}{
		{"untrained", 0},
		{"trained", 2},
		{"expert", 4},
		{"master", 6},
		{"legendary", 8},
		{"unknown", 0},
		{"", 0},
		{"TRAINED", 0}, // case-sensitive: unknown rank
	}
	for _, tc := range cases {
		got := ProficiencyBonus(tc.rank)
		if got != tc.want {
			t.Errorf("ProficiencyBonus(%q) = %d, want %d", tc.rank, got, tc.want)
		}
	}
}

// TestProperty_ProficiencyBonus_NonNegative asserts ProficiencyBonus never returns negative.
func TestProperty_ProficiencyBonus_NonNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		rank := rapid.String().Draw(rt, "rank")
		got := ProficiencyBonus(rank)
		if got < 0 {
			rt.Fatalf("ProficiencyBonus(%q) = %d, want >= 0", rank, got)
		}
	})
}

// TestProperty_ProficiencyBonus_Monotone asserts untrained <= trained <= expert <= master <= legendary.
func TestProperty_ProficiencyBonus_Monotone(t *testing.T) {
	ranks := []string{"untrained", "trained", "expert", "master", "legendary"}
	for i := 1; i < len(ranks); i++ {
		prev := ProficiencyBonus(ranks[i-1])
		curr := ProficiencyBonus(ranks[i])
		if prev > curr {
			t.Errorf("ProficiencyBonus(%q)=%d > ProficiencyBonus(%q)=%d, want monotone non-decreasing",
				ranks[i-1], prev, ranks[i], curr)
		}
	}
}

// TestOutcomeFor verifies all four outcome tiers with representative inputs.
func TestOutcomeFor(t *testing.T) {
	cases := []struct {
		total, dc int
		want      CheckOutcome
	}{
		// CritSuccess: total >= dc+10
		{20, 10, CritSuccess},
		{10, 0, CritSuccess},
		{25, 15, CritSuccess},
		// Success: dc <= total < dc+10
		{10, 10, Success},
		{14, 10, Success},
		{19, 10, Success},
		// Failure: dc-10 <= total < dc
		{9, 10, Failure},
		{5, 10, Failure},
		{0, 10, Failure},
		// CritFailure: total < dc-10
		{-1, 10, CritFailure},
		{-5, 10, CritFailure},
		{0, 12, CritFailure},
	}
	for _, tc := range cases {
		got := OutcomeFor(tc.total, tc.dc)
		if got != tc.want {
			t.Errorf("OutcomeFor(%d, %d) = %v, want %v", tc.total, tc.dc, got, tc.want)
		}
	}
}

// TestProperty_OutcomeFor_NeverPanics asserts OutcomeFor never panics on arbitrary int inputs.
func TestProperty_OutcomeFor_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		total := rapid.Int().Draw(rt, "total")
		dc := rapid.Int().Draw(rt, "dc")
		// Should never panic.
		_ = OutcomeFor(total, dc)
	})
}

// TestProperty_OutcomeFor_Ordering asserts the tier ordering invariants.
func TestProperty_OutcomeFor_Ordering(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dc := rapid.IntRange(-100, 100).Draw(rt, "dc")
		total := rapid.IntRange(-200, 200).Draw(rt, "total")
		outcome := OutcomeFor(total, dc)

		// If CritSuccess then also Success conditions hold (total >= dc)
		if outcome == CritSuccess && total < dc {
			rt.Fatalf("CritSuccess but total(%d) < dc(%d)", total, dc)
		}
		// If CritFailure then Failure conditions also hold (total < dc)
		if outcome == CritFailure && total >= dc {
			rt.Fatalf("CritFailure but total(%d) >= dc(%d)", total, dc)
		}
		// If total >= dc+10 then outcome must be CritSuccess
		if total >= dc+10 && outcome != CritSuccess {
			rt.Fatalf("total(%d) >= dc(%d)+10 but outcome is %v, want CritSuccess", total, dc, outcome)
		}
		// If total < dc-10 then outcome must be CritFailure
		if total < dc-10 && outcome != CritFailure {
			rt.Fatalf("total(%d) < dc(%d)-10 but outcome is %v, want CritFailure", total, dc, outcome)
		}
	})
}

// TestResolve verifies Total = Roll+AbilityMod+ProfBonus and Outcome is correct.
func TestResolve(t *testing.T) {
	def := TriggerDef{
		Skill:   "athletics",
		DC:      15,
		Trigger: "on_enter",
		Outcomes: OutcomeMap{
			CritSuccess: &Outcome{Message: "You leap across effortlessly!"},
			Success:     &Outcome{Message: "You make it across."},
			Failure:     &Outcome{Message: "You slip but catch yourself."},
			CritFailure: &Outcome{Message: "You tumble into the pit!"},
		},
	}

	cases := []struct {
		roll       int
		abilityMod int
		rank       string
		dc         int
		wantTotal  int
		wantOutcome CheckOutcome
	}{
		// roll=15, abilityMod=2, rank="trained"(+2), dc=15 => total=19, Success
		{15, 2, "trained", 15, 19, Success},
		// roll=15, abilityMod=2, rank="expert"(+4), dc=15 => total=21, CritSuccess (21>=15+6? no; 21>=15 yes, 21<25 no CritSuccess check: 21>=15+10=25? no, so Success)
		// Actually: 21 >= 15+10=25? No. 21 >= 15? Yes. => Success
		{15, 2, "expert", 15, 21, Success},
		// roll=20, abilityMod=3, rank="legendary"(+8), dc=15 => total=31, CritSuccess (31>=25)
		{20, 3, "legendary", 15, 31, CritSuccess},
		// roll=1, abilityMod=-2, rank="untrained"(+0), dc=15 => total=-1, CritFailure (-1<5)
		{1, -2, "untrained", 15, -1, CritFailure},
		// roll=5, abilityMod=0, rank="untrained"(+0), dc=15 => total=5, Failure (5<15 and 5>=5)
		{5, 0, "untrained", 15, 5, Failure},
	}

	for _, tc := range cases {
		result := Resolve(tc.roll, tc.abilityMod, tc.rank, tc.dc, def)
		if result.Total != tc.wantTotal {
			t.Errorf("Resolve(roll=%d, abilityMod=%d, rank=%q, dc=%d).Total = %d, want %d",
				tc.roll, tc.abilityMod, tc.rank, tc.dc, result.Total, tc.wantTotal)
		}
		if result.Outcome != tc.wantOutcome {
			t.Errorf("Resolve(roll=%d, abilityMod=%d, rank=%q, dc=%d).Outcome = %v, want %v",
				tc.roll, tc.abilityMod, tc.rank, tc.dc, result.Outcome, tc.wantOutcome)
		}
		if result.Roll != tc.roll {
			t.Errorf("Resolve result.Roll = %d, want %d", result.Roll, tc.roll)
		}
		if result.AbilityMod != tc.abilityMod {
			t.Errorf("Resolve result.AbilityMod = %d, want %d", result.AbilityMod, tc.abilityMod)
		}
		profBonus := ProficiencyBonus(tc.rank)
		if result.ProfBonus != profBonus {
			t.Errorf("Resolve result.ProfBonus = %d, want %d", result.ProfBonus, profBonus)
		}
		if result.TriggerDef.Skill != def.Skill {
			t.Errorf("Resolve result.TriggerDef.Skill = %q, want %q", result.TriggerDef.Skill, def.Skill)
		}
	}
}

// TestProperty_Resolve_TotalInvariant asserts Roll + AbilityMod + ProfBonus == Total.
func TestProperty_Resolve_TotalInvariant(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roll := rapid.IntRange(1, 20).Draw(rt, "roll")
		abilityMod := rapid.IntRange(-5, 10).Draw(rt, "abilityMod")
		rankIdx := rapid.IntRange(0, 4).Draw(rt, "rankIdx")
		ranks := []string{"untrained", "trained", "expert", "master", "legendary"}
		rank := ranks[rankIdx]
		dc := rapid.IntRange(1, 30).Draw(rt, "dc")

		def := TriggerDef{Skill: "acrobatics", DC: dc, Trigger: "on_enter"}
		result := Resolve(roll, abilityMod, rank, dc, def)

		profBonus := ProficiencyBonus(rank)
		wantTotal := roll + abilityMod + profBonus
		if result.Total != wantTotal {
			rt.Fatalf("Total invariant violated: roll(%d)+abilityMod(%d)+profBonus(%d)=%d but got %d",
				roll, abilityMod, profBonus, wantTotal, result.Total)
		}
		wantOutcome := OutcomeFor(wantTotal, dc)
		if result.Outcome != wantOutcome {
			rt.Fatalf("Outcome mismatch: OutcomeFor(%d,%d)=%v but got %v",
				wantTotal, dc, wantOutcome, result.Outcome)
		}
	})
}

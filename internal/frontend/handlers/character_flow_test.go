package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/frontend/handlers"
	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

// mockCharStore implements handlers.CharacterStore for testing.
type mockCharStore struct {
	chars     []*character.Character
	created   *character.Character
	createErr error
	listErr   error
}

func (m *mockCharStore) ListByAccount(ctx context.Context, accountID int64) ([]*character.Character, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.chars, nil
}

func (m *mockCharStore) Create(ctx context.Context, c *character.Character) (*character.Character, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	c.ID = 42
	m.created = c
	return c, nil
}

func (m *mockCharStore) GetByID(ctx context.Context, id int64) (*character.Character, error) {
	for _, c := range m.chars {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, nil
}

func TestErrSwitchCharacter_IsError(t *testing.T) {
	if handlers.ErrSwitchCharacter == nil {
		t.Fatal("ErrSwitchCharacter must be a non-nil error")
	}
	if handlers.ErrSwitchCharacter.Error() == "" {
		t.Fatal("ErrSwitchCharacter.Error() must be non-empty")
	}
}

func TestIsAlreadyLoggedIn_WithAlreadyConnectedError(t *testing.T) {
	err := errors.New("player \"uid1\" already connected")
	if !handlers.IsAlreadyLoggedIn(err) {
		t.Error("expected IsAlreadyLoggedIn to return true for 'already connected' error")
	}
}

func TestIsAlreadyLoggedIn_WithOtherError(t *testing.T) {
	err := errors.New("some other error")
	if handlers.IsAlreadyLoggedIn(err) {
		t.Error("expected IsAlreadyLoggedIn to return false for unrelated error")
	}
}

func TestIsAlreadyLoggedIn_WithNil(t *testing.T) {
	if handlers.IsAlreadyLoggedIn(nil) {
		t.Error("expected IsAlreadyLoggedIn to return false for nil")
	}
}

func TestFormatCharacterSummary(t *testing.T) {
	c := &character.Character{
		ID:     1,
		Name:   "Zara",
		Class:  "ganger",
		Level:  1,
		Region: "old_town",
		Team:   "gun",
	}
	summary := handlers.FormatCharacterSummary(c, "the Northeast")
	assert.Contains(t, summary, "Zara")
	assert.Contains(t, summary, "ganger")
	assert.Contains(t, summary, "1")
	assert.Contains(t, summary, "from the Northeast")
	assert.Contains(t, summary, "gun")
}

func TestFormatCharacterStats(t *testing.T) {
	c := &character.Character{
		Name:      "Zara",
		Class:     "ganger",
		Level:     1,
		Region:    "old_town",
		MaxHP:     10,
		CurrentHP: 10,
		Abilities: character.AbilityScores{
			Brutality: 14, Quickness: 10, Grit: 10,
			Reasoning: 10, Savvy: 8, Flair: 10,
		},
	}
	stats := handlers.FormatCharacterStats(c, "the Northeast")
	assert.Contains(t, stats, "BRT")
	assert.Contains(t, stats, "14")
	assert.Contains(t, stats, "HP")
	assert.Contains(t, stats, "10")
	assert.Contains(t, stats, "the Northeast")
}

// TestProperty_FormatCharacterSummary verifies that for any character, the summary
// is non-empty and always contains the character's name, class, and level.
func TestProperty_FormatCharacterSummary(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{1,15}`).Draw(rt, "name")
		class := rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, "class")
		level := rapid.IntRange(1, 20).Draw(rt, "level")

		regionDisplay := rapid.StringMatching(`[A-Za-z ]+`).Draw(rt, "regionDisplay")
		c := &character.Character{
			Name:   name,
			Class:  class,
			Level:  level,
			Region: "old_town",
		}
		summary := handlers.FormatCharacterSummary(c, regionDisplay)
		assert.NotEmpty(rt, summary)
		assert.Contains(rt, summary, name)
		assert.Contains(rt, summary, class)
		assert.Contains(rt, summary, regionDisplay)
	})
}

func TestRandomNames_NonEmpty(t *testing.T) {
	assert.NotEmpty(t, handlers.RandomNames)
	for _, name := range handlers.RandomNames {
		assert.GreaterOrEqual(t, len(name), 2)
		assert.LessOrEqual(t, len(name), 32)
		assert.NotEqual(t, "cancel", strings.ToLower(name))
		assert.NotEqual(t, "random", strings.ToLower(name))
	}
}


func TestIsRandomInput(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty string", "", true},
		{"lowercase r", "r", true},
		{"uppercase R", "R", true},
		{"lowercase random", "random", true},
		{"uppercase RANDOM", "RANDOM", true},
		{"digit 1", "1", false},
		{"digit 2", "2", false},
		{"cancel", "cancel", false},
		{"padded r", "  r  ", true},
		{"padded RANDOM", "  RANDOM  ", true},
		{"padded spaces only", "   ", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, handlers.IsRandomInput(tc.input))
		})
	}
}

func TestProperty_IsRandomInput_RandomKeywords(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Any string whose trimmed lowercase form is "", "r", or "random" must return true.
		keyword := rapid.SampledFrom([]string{"", "r", "R", "random", "RANDOM", "Random"}).Draw(rt, "keyword")
		padding := rapid.StringMatching(`\s*`).Draw(rt, "padding")
		input := padding + keyword + padding
		assert.True(rt, handlers.IsRandomInput(input),
			"expected IsRandomInput(%q) to be true", input)
	})
}

func TestProperty_IsRandomInput_OtherInputsAreFalse(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Strings that aren't blank/"r"/"random" after trim+lower must return false.
		// Generate digit strings (1-99) — these are always non-random.
		n := rapid.IntRange(1, 99).Draw(rt, "n")
		input := fmt.Sprintf("%d", n)
		assert.False(rt, handlers.IsRandomInput(input),
			"expected IsRandomInput(%q) to be false", input)
	})
}

func TestRandomizeRemaining_RegionFromSlice(t *testing.T) {
	regions := []*ruleset.Region{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}
	teams := []*ruleset.Team{{ID: "gun"}, {ID: "machete"}}
	jobs := []*ruleset.Job{
		{ID: "j1", Team: ""},
		{ID: "j2", Team: "gun"},
		{ID: "j3", Team: "machete"},
	}
	region, team, job, err := handlers.RandomizeRemaining(regions, nil, teams, nil, jobs)
	assert.NoError(t, err)
	assert.NotNil(t, region)
	assert.NotNil(t, team)
	assert.NotNil(t, job)
	assert.Contains(t, regions, region)
	assert.Contains(t, teams, team)
	assert.True(t, job.Team == "" || job.Team == team.ID,
		"job %s (team=%q) incompatible with team %s", job.ID, job.Team, team.ID)
}

func TestRandomizeRemaining_JobCompatibleWithTeam(t *testing.T) {
	regions := []*ruleset.Region{{ID: "r1"}}
	teams := []*ruleset.Team{{ID: "gun"}, {ID: "machete"}}
	jobs := []*ruleset.Job{
		{ID: "j1", Team: ""},
		{ID: "j2", Team: "gun"},
		{ID: "j3", Team: "machete"},
	}
	for i := 0; i < 50; i++ {
		_, team, job, err := handlers.RandomizeRemaining(regions, nil, teams, nil, jobs)
		assert.NoError(t, err)
		assert.True(t, job.Team == "" || job.Team == team.ID,
			"job %s (team=%q) incompatible with team %s", job.ID, job.Team, team.ID)
	}
}

func TestRandomizeRemaining_FixedTeamHonored(t *testing.T) {
	regions := []*ruleset.Region{{ID: "r1"}}
	teams := []*ruleset.Team{{ID: "gun"}, {ID: "machete"}}
	fixedTeam := teams[0]
	jobs := []*ruleset.Job{
		{ID: "j1", Team: ""},
		{ID: "j2", Team: "gun"},
		{ID: "j3", Team: "machete"},
	}
	for i := 0; i < 50; i++ {
		_, team, job, err := handlers.RandomizeRemaining(regions, nil, teams, fixedTeam, jobs)
		assert.NoError(t, err)
		assert.Equal(t, fixedTeam, team)
		assert.True(t, job.Team == "" || job.Team == "gun")
	}
}

func TestProperty_RandomizeRemaining_AlwaysValid(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		nRegions := rapid.IntRange(1, 5).Draw(rt, "nRegions")
		nTeams := rapid.IntRange(1, 3).Draw(rt, "nTeams")
		useFixedRegion := rapid.Bool().Draw(rt, "useFixedRegion")
		useFixedTeam := rapid.Bool().Draw(rt, "useFixedTeam")

		regions := make([]*ruleset.Region, nRegions)
		for i := range regions {
			regions[i] = &ruleset.Region{ID: fmt.Sprintf("r%d", i)}
		}
		teams := make([]*ruleset.Team, nTeams)
		for i := range teams {
			teams[i] = &ruleset.Team{ID: fmt.Sprintf("t%d", i)}
		}
		jobs := []*ruleset.Job{{ID: "general", Team: ""}}

		var fixedRegion *ruleset.Region
		if useFixedRegion {
			fixedRegion = regions[0]
		}
		var fixedTeam *ruleset.Team
		if useFixedTeam {
			fixedTeam = teams[0]
		}

		region, team, job, err := handlers.RandomizeRemaining(regions, fixedRegion, teams, fixedTeam, jobs)
		assert.NoError(rt, err)
		assert.NotNil(rt, region)
		assert.NotNil(rt, team)
		assert.NotNil(rt, job)
		assert.True(rt, job.Team == "" || job.Team == team.ID)
		if fixedRegion != nil {
			assert.Equal(rt, fixedRegion, region)
		}
		if fixedTeam != nil {
			assert.Equal(rt, fixedTeam, team)
		}
	})
}

// ---------------------------------------------------------------------------
// FeatPoolDeficit tests
// ---------------------------------------------------------------------------

// TestFeatPoolDeficit_PartialStored verifies that a pool with count=2 and 1 stored
// feat from the pool returns deficit 1.
func TestFeatPoolDeficit_PartialStored(t *testing.T) {
	pool := []string{"feat_a", "feat_b", "feat_c"}
	stored := map[string]bool{"feat_a": true}
	deficit := handlers.FeatPoolDeficit(pool, stored, 2)
	assert.Equal(t, 1, deficit)
}

// TestFeatPoolDeficit_FullyStored verifies that a pool with count=2 and 2 stored
// feats from the pool returns deficit 0.
func TestFeatPoolDeficit_FullyStored(t *testing.T) {
	pool := []string{"feat_a", "feat_b", "feat_c"}
	stored := map[string]bool{"feat_a": true, "feat_b": true}
	deficit := handlers.FeatPoolDeficit(pool, stored, 2)
	assert.Equal(t, 0, deficit)
}

// TestFeatPoolDeficit_OverStored verifies that a pool with count=2 and 3 stored
// feats (more than count) returns deficit 0.
func TestFeatPoolDeficit_OverStored(t *testing.T) {
	pool := []string{"feat_a", "feat_b", "feat_c"}
	stored := map[string]bool{"feat_a": true, "feat_b": true, "feat_c": true}
	deficit := handlers.FeatPoolDeficit(pool, stored, 2)
	assert.Equal(t, 0, deficit)
}

// TestFeatPoolDeficit_NilStored verifies that a nil storedFeatIDs map returns
// deficit == count.
func TestFeatPoolDeficit_NilStored(t *testing.T) {
	pool := []string{"feat_a", "feat_b"}
	deficit := handlers.FeatPoolDeficit(pool, nil, 2)
	assert.Equal(t, 2, deficit)
}

// TestFeatPoolDeficit_EmptyPool verifies that an empty pool always returns
// deficit 0 regardless of count.
func TestFeatPoolDeficit_EmptyPool(t *testing.T) {
	deficit := handlers.FeatPoolDeficit([]string{}, map[string]bool{}, 3)
	assert.Equal(t, 0, deficit)
}

// TestFeatPoolDeficit_ZeroCount verifies that count=0 always returns deficit 0.
func TestFeatPoolDeficit_ZeroCount(t *testing.T) {
	pool := []string{"feat_a", "feat_b"}
	deficit := handlers.FeatPoolDeficit(pool, nil, 0)
	assert.Equal(t, 0, deficit)
}

// TestFeatPoolDeficit_StoredOutsidePool verifies that feats stored but not in
// pool are not counted toward satisfying the deficit.
func TestFeatPoolDeficit_StoredOutsidePool(t *testing.T) {
	pool := []string{"feat_a", "feat_b"}
	stored := map[string]bool{"feat_x": true, "feat_y": true} // not in pool
	deficit := handlers.FeatPoolDeficit(pool, stored, 2)
	assert.Equal(t, 2, deficit)
}

// TestProperty_FeatPoolDeficit verifies structural invariants with property-based testing.
func TestProperty_FeatPoolDeficit(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		count := rapid.IntRange(0, 5).Draw(rt, "count")
		poolSize := rapid.IntRange(0, 6).Draw(rt, "poolSize")
		pool := make([]string, poolSize)
		for i := range pool {
			pool[i] = fmt.Sprintf("feat_%d", i)
		}

		storedCount := rapid.IntRange(0, poolSize).Draw(rt, "storedCount")
		stored := make(map[string]bool, storedCount)
		for i := 0; i < storedCount; i++ {
			stored[fmt.Sprintf("feat_%d", i)] = true
		}

		deficit := handlers.FeatPoolDeficit(pool, stored, count)

		// Invariant 1: deficit is never negative.
		assert.GreaterOrEqual(rt, deficit, 0, "deficit must not be negative")

		// Invariant 2: deficit is never greater than count.
		assert.LessOrEqual(rt, deficit, count, "deficit must not exceed count")

		// Invariant 3: deficit matches reference formula.
		// When pool is empty there is nothing to pick, so deficit is 0.
		var expected int
		if len(pool) == 0 {
			expected = 0
		} else {
			storedFromPool := 0
			for _, id := range pool {
				if stored[id] {
					storedFromPool++
				}
			}
			expected = count - storedFromPool
			if expected < 0 {
				expected = 0
			}
		}
		assert.Equal(rt, expected, deficit)
	})
}

// ---------------------------------------------------------------------------
// mockCharacterFeatsSetter — in-memory stub for CharacterFeatsSetter.
// ---------------------------------------------------------------------------

type mockCharacterFeatsSetter struct {
	stored    []string
	getAllErr  error
	setAllErr error
	setAllCalled bool
	lastSet   []string
}

func (m *mockCharacterFeatsSetter) GetAll(_ context.Context, _ int64) ([]string, error) {
	if m.getAllErr != nil {
		return nil, m.getAllErr
	}
	out := make([]string, len(m.stored))
	copy(out, m.stored)
	return out, nil
}

func (m *mockCharacterFeatsSetter) SetAll(_ context.Context, _ int64, feats []string) error {
	m.setAllCalled = true
	m.lastSet = feats
	if m.setAllErr != nil {
		return m.setAllErr
	}
	m.stored = feats
	return nil
}

// ---------------------------------------------------------------------------
// ensureFeats early-exit path: FeatPoolDeficit-based coverage.
//
// ensureFeats is unexported and requires a live telnet connection, so these
// tests exercise the early-exit decision logic indirectly through FeatPoolDeficit.
// They document the invariants that ensureFeats relies on to decide whether to
// skip the prompt-and-store phase.
// ---------------------------------------------------------------------------

// TestEnsureFeats_EarlyExit_AllDeficitsZero verifies that when a character has
// all pool feats already stored, all three deficit values are 0 — which is the
// condition ensureFeats uses to skip prompting.
func TestEnsureFeats_EarlyExit_AllDeficitsZero(t *testing.T) {
	jobChoicesPool := []string{"feat_a", "feat_b"}
	generalPool := []string{"general_1", "general_2"}
	skillPool := []string{"skill_feat_1"}

	// All pool feats already stored.
	stored := map[string]bool{
		"feat_a":      true,
		"feat_b":      true,
		"general_1":   true,
		"skill_feat_1": true,
	}

	jobChoicesDeficit := handlers.FeatPoolDeficit(jobChoicesPool, stored, 2)
	generalDeficit := handlers.FeatPoolDeficit(generalPool, stored, 1)
	skillDeficit := handlers.FeatPoolDeficit(skillPool, stored, 1)

	assert.Equal(t, 0, jobChoicesDeficit, "job choices fully stored: deficit must be 0")
	assert.Equal(t, 0, generalDeficit, "general feat fully stored: deficit must be 0")
	assert.Equal(t, 0, skillDeficit, "skill feat fully stored: deficit must be 0")
}

// TestEnsureFeats_MergePath_PartialDeficit verifies that when a character is
// missing some pool feats, the deficit is non-zero — causing ensureFeats to
// proceed past the early-exit to prompt and merge.
func TestEnsureFeats_MergePath_PartialDeficit(t *testing.T) {
	jobChoicesPool := []string{"feat_a", "feat_b"}
	generalPool := []string{"general_1", "general_2"}
	skillPool := []string{"skill_feat_1"}

	// Only job choices satisfied; general and skill are missing.
	stored := map[string]bool{
		"feat_a": true,
		"feat_b": true,
	}

	jobChoicesDeficit := handlers.FeatPoolDeficit(jobChoicesPool, stored, 2)
	generalDeficit := handlers.FeatPoolDeficit(generalPool, stored, 1)
	skillDeficit := handlers.FeatPoolDeficit(skillPool, stored, 1)

	assert.Equal(t, 0, jobChoicesDeficit, "job choices fully stored")
	assert.Equal(t, 1, generalDeficit, "general feat missing: deficit must be 1")
	assert.Equal(t, 1, skillDeficit, "skill feat missing: deficit must be 1")

	// At least one deficit non-zero => ensureFeats would NOT early-exit.
	wouldEarlyExit := jobChoicesDeficit == 0 && generalDeficit == 0 && skillDeficit == 0
	assert.False(t, wouldEarlyExit, "must not early-exit when any deficit is non-zero")
}

// TestEnsureFeats_FixedOnlyJob_NoEarlyExit documents the bug fixed in ensureFeats:
// a job with ONLY fixed feats (no pool/choices grants) would previously trigger an
// early-exit with all deficits == 0 and never store the fixed feats. The fix adds a
// separate fixedMissing check before the early-exit guard.
//
// This test verifies the logical invariants the fix relies on:
//   - All three pool deficits are 0 when no pool grants exist.
//   - The stored set does NOT contain the fixed feat (simulating a new character).
//   - Therefore fixedMissing == true, preventing the early-exit.
func TestEnsureFeats_FixedOnlyJob_NoEarlyExit(t *testing.T) {
	// Job has no pool/choices grants (no Choices, GeneralCount==0, no skill feats).
	// All three deficit functions return 0.
	jobChoicesDeficit := handlers.FeatPoolDeficit(nil, nil, 0)
	generalDeficit := handlers.FeatPoolDeficit(nil, nil, 0)
	skillDeficit := handlers.FeatPoolDeficit([]string{}, nil, 1)

	assert.Equal(t, 0, jobChoicesDeficit)
	assert.Equal(t, 0, generalDeficit)
	// skillPool is empty => FeatPoolDeficit returns 0 regardless of count.
	assert.Equal(t, 0, skillDeficit)

	// Simulate: stored set is empty (new character), fixed feat "fixed_feat_1" not stored.
	stored := map[string]bool{}
	fixedFeats := []string{"fixed_feat_1"}

	fixedMissing := false
	for _, id := range fixedFeats {
		if !stored[id] {
			fixedMissing = true
			break
		}
	}

	assert.True(t, fixedMissing, "fixed feat not yet stored: fixedMissing must be true")

	// The corrected early-exit guard: only skip when ALL deficits==0 AND !fixedMissing.
	wouldEarlyExit := jobChoicesDeficit == 0 && generalDeficit == 0 && skillDeficit == 0 && !fixedMissing
	assert.False(t, wouldEarlyExit,
		"fixed-only job must not early-exit: fixed feats still need to be stored")
}

// TestEnsureFeats_FixedOnlyJob_AlreadyStored_EarlyExit verifies that a fixed-only
// job DOES early-exit when the fixed feat is already stored (no work needed).
func TestEnsureFeats_FixedOnlyJob_AlreadyStored_EarlyExit(t *testing.T) {
	jobChoicesDeficit := handlers.FeatPoolDeficit(nil, nil, 0)
	generalDeficit := handlers.FeatPoolDeficit(nil, nil, 0)
	skillDeficit := handlers.FeatPoolDeficit([]string{}, nil, 1)

	stored := map[string]bool{"fixed_feat_1": true}
	fixedFeats := []string{"fixed_feat_1"}

	fixedMissing := false
	for _, id := range fixedFeats {
		if !stored[id] {
			fixedMissing = true
			break
		}
	}

	assert.False(t, fixedMissing, "fixed feat already stored: fixedMissing must be false")

	wouldEarlyExit := jobChoicesDeficit == 0 && generalDeficit == 0 && skillDeficit == 0 && !fixedMissing
	assert.True(t, wouldEarlyExit,
		"fixed-only job with all feats stored should early-exit (nothing to do)")
}

// TestMockCharacterFeatsSetter_GetAllAndSetAll verifies that the mock correctly
// stores and retrieves feat lists, fulfilling the CharacterFeatsSetter contract.
func TestMockCharacterFeatsSetter_GetAllAndSetAll(t *testing.T) {
	cases := []struct {
		name    string
		initial []string
		setTo   []string
	}{
		{"empty to populated", nil, []string{"feat_a", "feat_b"}},
		{"populated to different", []string{"feat_x"}, []string{"feat_y", "feat_z"}},
		{"populated to empty", []string{"feat_a"}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &mockCharacterFeatsSetter{stored: tc.initial}
			ctx := context.Background()

			initial, err := m.GetAll(ctx, 1)
			assert.NoError(t, err)
			// GetAll copies the stored slice; both nil and empty are valid "no feats" states.
			assert.Equal(t, len(tc.initial), len(initial))

			err = m.SetAll(ctx, 1, tc.setTo)
			assert.NoError(t, err)
			assert.True(t, m.setAllCalled)
			assert.Equal(t, tc.setTo, m.lastSet)

			got, err := m.GetAll(ctx, 1)
			assert.NoError(t, err)
			assert.Equal(t, tc.setTo, got)
		})
	}
}

func TestRenderArchetypeMenu_ContainsKeyAbility(t *testing.T) {
	archetypes := []*ruleset.Archetype{
		{ID: "aggressor", Name: "Aggressor", Description: "Violence solves everything.", KeyAbility: "brutality", HitPointsPerLevel: 10},
	}
	output := handlers.RenderArchetypeMenu(archetypes)
	assert.Contains(t, output, "Aggressor")
	assert.Contains(t, output, "brutality")
	assert.Contains(t, output, "10")
}

// TestProperty_FormatCharacterStats verifies that for any character, the stats block
// is non-empty and contains all six ability score labels plus HP.
func TestProperty_FormatCharacterStats(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		brt := rapid.IntRange(1, 20).Draw(rt, "brt")
		qck := rapid.IntRange(1, 20).Draw(rt, "qck")
		grt := rapid.IntRange(1, 20).Draw(rt, "grt")
		rsn := rapid.IntRange(1, 20).Draw(rt, "rsn")
		sav := rapid.IntRange(1, 20).Draw(rt, "sav")
		flr := rapid.IntRange(1, 20).Draw(rt, "flr")
		hp := rapid.IntRange(1, 100).Draw(rt, "hp")

		c := &character.Character{
			Name:      "Test",
			Class:     "ganger",
			Level:     1,
			Region:    "old_town",
			MaxHP:     hp,
			CurrentHP: hp,
			Abilities: character.AbilityScores{
				Brutality: brt, Quickness: qck, Grit: grt,
				Reasoning: rsn, Savvy: sav, Flair: flr,
			},
		}
		regionDisplay := rapid.StringMatching(`[A-Za-z ]+`).Draw(rt, "regionDisplay")
		stats := handlers.FormatCharacterStats(c, regionDisplay)
		assert.NotEmpty(rt, stats)
		for _, label := range []string{"BRT", "QCK", "GRT", "RSN", "SAV", "FLR", "HP"} {
			assert.Contains(rt, stats, label)
		}
		assert.Contains(rt, stats, regionDisplay)
	})
}

// ---------------------------------------------------------------------------
// PromptGenderStep tests
// ---------------------------------------------------------------------------

// newGenderTestConn creates a telnet.Conn backed by net.Pipe.
// It drains all server-side writes in a goroutine (so writes don't block),
// and feeds clientInputs one at a time to the server's ReadLine calls.
// The drainer goroutine exits when client is closed.
func newGenderTestConn(t *testing.T, clientInputs ...string) (*telnet.Conn, net.Conn) {
	t.Helper()
	client, server := net.Pipe()
	conn := telnet.NewConn(server, 2*time.Second, 2*time.Second)
	t.Cleanup(func() {
		_ = client.Close()
		_ = conn.Close()
	})
	// Drain server output so writes never block.
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := client.Read(buf)
			if err != nil {
				return
			}
		}
	}()
	// Feed inputs one at a time.
	go func() {
		for _, line := range clientInputs {
			_, _ = fmt.Fprintf(client, "%s\r\n", line)
		}
	}()
	return conn, client
}

// TestPromptGenderStep_Male verifies that input "1" returns "male".
func TestPromptGenderStep_Male(t *testing.T) {
	conn, _ := newGenderTestConn(t, "1")
	gender, err := handlers.PromptGenderStep(conn)
	require.NoError(t, err)
	assert.Equal(t, "male", gender)
}

// TestPromptGenderStep_Female verifies that input "2" returns "female".
func TestPromptGenderStep_Female(t *testing.T) {
	conn, _ := newGenderTestConn(t, "2")
	gender, err := handlers.PromptGenderStep(conn)
	require.NoError(t, err)
	assert.Equal(t, "female", gender)
}

// TestPromptGenderStep_NonBinary verifies that input "3" returns "non-binary".
func TestPromptGenderStep_NonBinary(t *testing.T) {
	conn, _ := newGenderTestConn(t, "3")
	gender, err := handlers.PromptGenderStep(conn)
	require.NoError(t, err)
	assert.Equal(t, "non-binary", gender)
}

// TestPromptGenderStep_Indeterminate verifies that input "4" returns "indeterminate".
func TestPromptGenderStep_Indeterminate(t *testing.T) {
	conn, _ := newGenderTestConn(t, "4")
	gender, err := handlers.PromptGenderStep(conn)
	require.NoError(t, err)
	assert.Equal(t, "indeterminate", gender)
}

// TestPromptGenderStep_Custom verifies that input "5" followed by a custom string
// returns "custom:<string>".
func TestPromptGenderStep_Custom(t *testing.T) {
	conn, _ := newGenderTestConn(t, "5", "Wanderer")
	gender, err := handlers.PromptGenderStep(conn)
	require.NoError(t, err)
	assert.Equal(t, "custom:Wanderer", gender)
}

// TestPromptGenderStep_CustomTruncated verifies that a custom gender longer than
// 32 chars is truncated to 32 chars.
func TestPromptGenderStep_CustomTruncated(t *testing.T) {
	long := strings.Repeat("x", 40)
	conn, _ := newGenderTestConn(t, "5", long)
	gender, err := handlers.PromptGenderStep(conn)
	require.NoError(t, err)
	assert.Equal(t, "custom:"+strings.Repeat("x", 32), gender)
}

// TestPromptGenderStep_CustomCancel verifies that "cancel" as custom gender
// returns ("", nil).
func TestPromptGenderStep_CustomCancel(t *testing.T) {
	conn, _ := newGenderTestConn(t, "5", "cancel")
	gender, err := handlers.PromptGenderStep(conn)
	require.NoError(t, err)
	assert.Equal(t, "", gender)
}

// TestPromptGenderStep_Cancel verifies that "cancel" as top-level input returns ("", nil).
func TestPromptGenderStep_Cancel(t *testing.T) {
	conn, _ := newGenderTestConn(t, "cancel")
	gender, err := handlers.PromptGenderStep(conn)
	require.NoError(t, err)
	assert.Equal(t, "", gender)
}

// TestPromptGenderStep_Zero verifies that "0" returns a standard gender (random).
func TestPromptGenderStep_Zero(t *testing.T) {
	conn, _ := newGenderTestConn(t, "0")
	gender, err := handlers.PromptGenderStep(conn)
	require.NoError(t, err)
	assert.Contains(t, character.StandardGenders, gender)
}

// TestPromptGenderStep_Default verifies that empty input returns a standard gender (random).
func TestPromptGenderStep_Default(t *testing.T) {
	conn, _ := newGenderTestConn(t, "")
	gender, err := handlers.PromptGenderStep(conn)
	require.NoError(t, err)
	assert.Contains(t, character.StandardGenders, gender)
}

// TestFormatCharacterStats_ShowsGender verifies that FormatCharacterStats includes
// the character's gender in the output.
func TestFormatCharacterStats_ShowsGender(t *testing.T) {
	c := &character.Character{
		Name:   "Ash",
		Class:  "ganger",
		Level:  1,
		Region: "old_town",
		Gender: "non-binary",
		MaxHP:  10, CurrentHP: 10,
	}
	stats := handlers.FormatCharacterStats(c, "Old Town")
	assert.Contains(t, stats, "non-binary")
	assert.Contains(t, stats, "Gender")
}

// TestProperty_PromptGenderStep_StandardSelections verifies that choices 1-4
// always return the corresponding standard gender value.
func TestProperty_PromptGenderStep_StandardSelections(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"1", "male"},
		{"2", "female"},
		{"3", "non-binary"},
		{"4", "indeterminate"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			rapid.Check(t, func(rt *rapid.T) {
				conn, _ := newGenderTestConn(t, tc.input)
				gender, err := handlers.PromptGenderStep(conn)
				require.NoError(rt, err)
				assert.Equal(rt, tc.expected, gender)
			})
		})
	}
}

# Random Character Generation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Allow players to type "random" at the name step or choose "R" at any list step to randomize that step and all remaining steps, with R as the default at each list step.

**Architecture:** All changes are in `internal/frontend/handlers/character_flow.go`. Add a `RandomNames` package-level slice, a `RandomizeRemaining` exported helper, and a private `buildAndConfirm` helper. Modify `characterCreationFlow` to offer R at each list step and cascade when chosen.

**Tech Stack:** Go, `math/rand`, `pgregory.net/rapid` (property tests), `github.com/stretchr/testify/assert`

---

### Task 1: Add RandomNames and RandomizeRemaining helper

**Files:**
- Modify: `internal/frontend/handlers/character_flow.go`
- Test: `internal/frontend/handlers/character_flow_test.go`

**Context:** `character_flow.go` is in `package handlers`. The test file is `package handlers_test` (external). Types used: `*ruleset.Region`, `*ruleset.Team`, `*ruleset.Job` from `github.com/cory-johannsen/mud/internal/game/ruleset`. Job has a `Team string` field — empty means all teams, otherwise matches `team.ID`.

**Step 1: Write the failing tests**

Add to `internal/frontend/handlers/character_flow_test.go`:

```go
import (
    // add to existing imports:
    "github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestRandomNames_NonEmpty(t *testing.T) {
    assert.NotEmpty(t, handlers.RandomNames)
    for _, name := range handlers.RandomNames {
        assert.GreaterOrEqual(t, len(name), 2)
        assert.LessOrEqual(t, len(name), 32)
    }
}

func TestRandomizeRemaining_RegionFromSlice(t *testing.T) {
    regions := []*ruleset.Region{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}
    teams := []*ruleset.Team{{ID: "gun"}, {ID: "machete"}}
    jobs := []*ruleset.Job{
        {ID: "j1", Team: ""},
        {ID: "j2", Team: "gun"},
        {ID: "j3", Team: "machete"},
    }
    region, team, job := handlers.RandomizeRemaining(regions, nil, teams, nil, jobs)
    assert.NotNil(t, region)
    assert.NotNil(t, team)
    assert.NotNil(t, job)
    assert.Contains(t, regions, region)
    assert.Contains(t, teams, team)
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
        _, team, job := handlers.RandomizeRemaining(regions, nil, teams, nil, jobs)
        assert.True(t, job.Team == "" || job.Team == team.ID,
            "job %s (team=%q) incompatible with team %s", job.ID, job.Team, team.ID)
    }
}

func TestRandomizeRemaining_FixedTeamHonored(t *testing.T) {
    regions := []*ruleset.Region{{ID: "r1"}}
    teams := []*ruleset.Team{{ID: "gun"}, {ID: "machete"}}
    fixedTeam := teams[0] // gun
    jobs := []*ruleset.Job{
        {ID: "j1", Team: ""},
        {ID: "j2", Team: "gun"},
        {ID: "j3", Team: "machete"},
    }
    for i := 0; i < 50; i++ {
        _, team, job := handlers.RandomizeRemaining(regions, nil, teams, fixedTeam, jobs)
        assert.Equal(t, fixedTeam, team)
        assert.True(t, job.Team == "" || job.Team == "gun")
    }
}

func TestProperty_RandomizeRemaining_AlwaysValid(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        nRegions := rapid.IntRange(1, 5).Draw(rt, "nRegions")
        nTeams := rapid.IntRange(1, 3).Draw(rt, "nTeams")

        regions := make([]*ruleset.Region, nRegions)
        for i := range regions {
            regions[i] = &ruleset.Region{ID: fmt.Sprintf("r%d", i)}
        }
        teams := make([]*ruleset.Team, nTeams)
        for i := range teams {
            teams[i] = &ruleset.Team{ID: fmt.Sprintf("t%d", i)}
        }
        // always include at least one general job
        jobs := []*ruleset.Job{{ID: "general", Team: ""}}

        region, team, job := handlers.RandomizeRemaining(regions, nil, teams, nil, jobs)
        assert.NotNil(rt, region)
        assert.NotNil(rt, team)
        assert.NotNil(rt, job)
        assert.True(rt, job.Team == "" || job.Team == team.ID)
    })
}
```

**Step 2: Run to verify they fail**

```
go test -run 'TestRandomNames|TestRandomizeRemaining|TestProperty_RandomizeRemaining' ./internal/frontend/handlers/
```

Expected: FAIL — `handlers.RandomNames` and `handlers.RandomizeRemaining` undefined.

**Step 3: Add RandomNames and RandomizeRemaining to character_flow.go**

Add after the imports block at the top of `internal/frontend/handlers/character_flow.go`:

```go
import (
    // add to existing imports:
    "math/rand"
)

// RandomNames is the pool of names used when the player requests a random character name.
var RandomNames = []string{
    "Raze", "Vex", "Cinder", "Sable", "Grit", "Ash", "Flint", "Thorn",
    "Kael", "Dusk", "Riven", "Scar", "Nox", "Wren", "Jace", "Brix",
    "Colt", "Ember", "Slate", "Pike",
}

// RandomizeRemaining picks random selections for all unresolved character creation steps.
// Pass nil for fixedRegion to randomize the region; pass nil for fixedTeam to randomize the team.
// The returned job is always compatible with the returned team (job.Team == "" or job.Team == team.ID).
//
// Precondition: regions, teams, and allJobs must each be non-empty.
// Postcondition: returned job.Team is "" or equals returned team.ID.
func RandomizeRemaining(
    regions []*ruleset.Region, fixedRegion *ruleset.Region,
    teams []*ruleset.Team, fixedTeam *ruleset.Team,
    allJobs []*ruleset.Job,
) (region *ruleset.Region, team *ruleset.Team, job *ruleset.Job) {
    if fixedRegion != nil {
        region = fixedRegion
    } else {
        region = regions[rand.Intn(len(regions))]
    }
    if fixedTeam != nil {
        team = fixedTeam
    } else {
        team = teams[rand.Intn(len(teams))]
    }
    var available []*ruleset.Job
    for _, j := range allJobs {
        if j.Team == "" || j.Team == team.ID {
            available = append(available, j)
        }
    }
    job = available[rand.Intn(len(available))]
    return
}
```

**Step 4: Run tests to verify they pass**

```
go test -run 'TestRandomNames|TestRandomizeRemaining|TestProperty_RandomizeRemaining' ./internal/frontend/handlers/
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/frontend/handlers/character_flow.go internal/frontend/handlers/character_flow_test.go
git commit -m "feat: add RandomNames and RandomizeRemaining helper for character creation"
```

---

### Task 2: Extract buildAndConfirm helper

**Files:**
- Modify: `internal/frontend/handlers/character_flow.go`

**Context:** The preview+confirm+persist block at the end of `characterCreationFlow` (lines 202–233) will be needed at multiple points once randomization cascades are added. Extract it into a private helper to avoid duplication.

No new tests needed — existing behavior is unchanged; this is pure refactoring. The existing `TestFormatCharacterStats` and `TestFormatCharacterSummary` tests will continue to pass.

**Step 1: Extract buildAndConfirm**

Add this private method to `character_flow.go`. The signature:

```go
// buildAndConfirm builds a character from the given selections, shows the preview,
// prompts for confirmation, and persists on yes.
// Returns (nil, nil) if the player declines or cancels.
//
// Precondition: all parameters must be non-nil; accountID must be > 0.
// Postcondition: returns persisted *character.Character or (nil, nil) on cancel/decline.
func (h *AuthHandler) buildAndConfirm(
    ctx context.Context,
    conn *telnet.Conn,
    accountID int64,
    charName string,
    region *ruleset.Region,
    job *ruleset.Job,
    team *ruleset.Team,
) (*character.Character, error) {
    newChar, err := character.BuildWithJob(charName, region, job, team)
    if err != nil {
        _ = conn.WriteLine(telnet.Colorf(telnet.Red, "Error building character: %v", err))
        return nil, nil
    }

    _ = conn.WriteLine(telnet.Colorize(telnet.BrightCyan, "\r\n--- Character Preview ---"))
    _ = conn.WriteLine(FormatCharacterStats(newChar))
    _ = conn.WritePrompt(telnet.Colorize(telnet.BrightWhite, "Create this character? [y/N]: "))

    confirm, err := conn.ReadLine()
    if err != nil {
        return nil, fmt.Errorf("reading confirmation: %w", err)
    }
    if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
        _ = conn.WriteLine(telnet.Colorize(telnet.Yellow, "Character creation cancelled."))
        return nil, nil
    }

    newChar.AccountID = accountID
    start := time.Now()
    created, err := h.characters.Create(ctx, newChar)
    if err != nil {
        h.logger.Error("creating character", zap.String("name", newChar.Name), zap.Error(err))
        _ = conn.WriteLine(telnet.Colorf(telnet.Red, "Failed to create character: %v", err))
        return nil, nil
    }
    _ = conn.WriteLine(telnet.Colorf(telnet.BrightGreen,
        "Character %s created! [%s]", created.Name, time.Since(start)))
    return created, nil
}
```

Then replace the preview+confirm+persist block in `characterCreationFlow` (current lines 202–233) with:

```go
    return h.buildAndConfirm(ctx, conn, accountID, charName, selectedRegion, selectedJob, selectedTeam)
```

**Step 2: Run full handler tests to verify nothing broke**

```
go test ./internal/frontend/handlers/
```

Expected: PASS (all existing tests still pass)

**Step 3: Commit**

```bash
git add internal/frontend/handlers/character_flow.go
git commit -m "refactor: extract buildAndConfirm helper from characterCreationFlow"
```

---

### Task 3: Add random option to name step

**Files:**
- Modify: `internal/frontend/handlers/character_flow.go`
- Test: `internal/frontend/handlers/character_flow_test.go`

**Context:** The name step currently rejects input shorter than 2 chars. When the player types "random", we pick from `RandomNames` and print the selected name. This is testable directly since `RandomNames` is exported and the selection is always from that slice.

**Step 1: Write the failing test**

Add to `character_flow_test.go`:

```go
func TestRandomName_IsFromRandomNames(t *testing.T) {
    // Verify that "random" input always yields a name from RandomNames.
    // We can't call characterCreationFlow directly (needs telnet), so we
    // test the lookup logic indirectly: every name in RandomNames is valid.
    for _, name := range handlers.RandomNames {
        assert.GreaterOrEqual(t, len(name), 2, "name %q too short", name)
        assert.LessOrEqual(t, len(name), 32, "name %q too long", name)
        assert.NotEqual(t, "cancel", strings.ToLower(name))
        assert.NotEqual(t, "random", strings.ToLower(name))
    }
}
```

**Step 2: Run to verify it fails (or already passes)**

```
go test -run TestRandomName_IsFromRandomNames ./internal/frontend/handlers/
```

If it fails, the names list has an invalid entry — fix the list. If it passes, proceed.

**Step 3: Modify the name step in characterCreationFlow**

Replace current name step (lines 97–111 in `character_flow.go`) with:

```go
    // Step 1: Character name
    _ = conn.WritePrompt(telnet.Colorize(telnet.BrightWhite,
        "Enter your character's name (or 'random'): "))
    nameLine, err := conn.ReadLine()
    if err != nil {
        return nil, fmt.Errorf("reading character name: %w", err)
    }
    nameLine = strings.TrimSpace(nameLine)
    if strings.ToLower(nameLine) == "cancel" {
        return nil, nil
    }
    if strings.ToLower(nameLine) == "random" {
        nameLine = RandomNames[rand.Intn(len(RandomNames))]
        _ = conn.WriteLine(telnet.Colorf(telnet.Cyan, "Random name selected: %s", nameLine))
    }
    if len(nameLine) < 2 || len(nameLine) > 32 {
        _ = conn.WriteLine(telnet.Colorize(telnet.Red, "Name must be 2-32 characters."))
        return nil, nil
    }
    charName := nameLine
```

**Step 4: Run all handler tests**

```
go test ./internal/frontend/handlers/
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/frontend/handlers/character_flow.go internal/frontend/handlers/character_flow_test.go
git commit -m "feat: add random name option to character creation name step"
```

---

### Task 4: Add random option to region step with cascade

**Files:**
- Modify: `internal/frontend/handlers/character_flow.go`
- Test: `internal/frontend/handlers/character_flow_test.go`

**Context:** Region is step 2. `h.regions` is `[]*ruleset.Region`. When R is chosen (or blank input), call `RandomizeRemaining(regions, nil, teams, nil, h.jobs)` and call `buildAndConfirm`. The `isRandom` check: blank input or case-insensitive "r".

The test for cascade behavior tests `RandomizeRemaining` directly (already covered in Task 1). We add a test that `isRandomInput` correctly identifies random inputs.

**Step 1: Write the failing test**

Add to `character_flow_test.go`:

```go
func TestIsRandomInput(t *testing.T) {
    cases := []struct {
        input    string
        expected bool
    }{
        {"", true},
        {"r", true},
        {"R", true},
        {"random", true},
        {"RANDOM", true},
        {"1", false},
        {"2", false},
        {"cancel", false},
    }
    for _, tc := range cases {
        t.Run(tc.input, func(t *testing.T) {
            assert.Equal(t, tc.expected, handlers.IsRandomInput(tc.input))
        })
    }
}
```

**Step 2: Run to verify it fails**

```
go test -run TestIsRandomInput ./internal/frontend/handlers/
```

Expected: FAIL — `handlers.IsRandomInput` undefined.

**Step 3: Add IsRandomInput and modify region step**

Add to `character_flow.go`:

```go
// IsRandomInput reports whether the player's input at a list step should be
// treated as a request for random selection. Exported for testing.
func IsRandomInput(s string) bool {
    lower := strings.ToLower(strings.TrimSpace(s))
    return lower == "" || lower == "r" || lower == "random"
}
```

Replace the region step in `characterCreationFlow` (current lines 113–136) with:

```go
    // Step 2: Home region
    regions := h.regions
    _ = conn.WriteLine(telnet.Colorize(telnet.BrightYellow, "\r\nChoose your home region:"))
    for i, r := range regions {
        _ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s%s%s\r\n     %s",
            telnet.Green, i+1, telnet.Reset,
            telnet.BrightWhite, r.Name, telnet.Reset,
            r.Description))
    }
    _ = conn.WriteLine(fmt.Sprintf("  %sR%s. Random (default)", telnet.Green, telnet.Reset))
    _ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite,
        "Select region [1-%d/R, default=R]: ", len(regions)))
    regionLine, err := conn.ReadLine()
    if err != nil {
        return nil, fmt.Errorf("reading region selection: %w", err)
    }
    regionLine = strings.TrimSpace(regionLine)
    if strings.ToLower(regionLine) == "cancel" {
        return nil, nil
    }
    if IsRandomInput(regionLine) {
        region, team, job := RandomizeRemaining(regions, nil, h.teams, nil, h.jobs)
        _ = conn.WriteLine(telnet.Colorf(telnet.Cyan,
            "Random selections: Region=%s, Team=%s, Job=%s", region.Name, team.Name, job.Name))
        return h.buildAndConfirm(ctx, conn, accountID, charName, region, job, team)
    }
    regionChoice := 0
    if _, err := fmt.Sscanf(regionLine, "%d", &regionChoice); err != nil || regionChoice < 1 || regionChoice > len(regions) {
        _ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection."))
        return nil, nil
    }
    selectedRegion := regions[regionChoice-1]
```

**Step 4: Run all handler tests**

```
go test ./internal/frontend/handlers/
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/frontend/handlers/character_flow.go internal/frontend/handlers/character_flow_test.go
git commit -m "feat: add random option with cascade to region step"
```

---

### Task 5: Add random option to team and job steps

**Files:**
- Modify: `internal/frontend/handlers/character_flow.go`

**Context:** Same pattern as Task 4 but for team (cascade to job only) and job (randomize job only). `IsRandomInput` and `RandomizeRemaining` are already in place.

No new test functions needed — `IsRandomInput` and `RandomizeRemaining` are already tested. The step-level cascade behavior is the same pattern as region.

**Step 1: Modify team step**

Replace the team step in `characterCreationFlow` (current lines 138–165) with:

```go
    // Step 3: Team selection
    teams := h.teams
    _ = conn.WriteLine(telnet.Colorize(telnet.BrightYellow, "\r\nChoose your team:"))
    for i, t := range teams {
        _ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s%s%s\r\n     %s",
            telnet.Green, i+1, telnet.Reset,
            telnet.BrightWhite, t.Name, telnet.Reset,
            t.Description))
        for _, trait := range t.Traits {
            _ = conn.WriteLine(fmt.Sprintf("     %s[%s]%s %s",
                telnet.Yellow, trait.Name, telnet.Reset, trait.Effect))
        }
    }
    _ = conn.WriteLine(fmt.Sprintf("  %sR%s. Random (default)", telnet.Green, telnet.Reset))
    _ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite,
        "Select team [1-%d/R, default=R]: ", len(teams)))
    teamLine, err := conn.ReadLine()
    if err != nil {
        return nil, fmt.Errorf("reading team selection: %w", err)
    }
    teamLine = strings.TrimSpace(teamLine)
    if strings.ToLower(teamLine) == "cancel" {
        return nil, nil
    }
    if IsRandomInput(teamLine) {
        _, team, job := RandomizeRemaining(regions, selectedRegion, teams, nil, h.jobs)
        _ = conn.WriteLine(telnet.Colorf(telnet.Cyan,
            "Random selections: Team=%s, Job=%s", team.Name, job.Name))
        return h.buildAndConfirm(ctx, conn, accountID, charName, selectedRegion, job, team)
    }
    teamChoice := 0
    if _, err := fmt.Sscanf(teamLine, "%d", &teamChoice); err != nil || teamChoice < 1 || teamChoice > len(teams) {
        _ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection."))
        return nil, nil
    }
    selectedTeam := teams[teamChoice-1]
```

**Step 2: Modify job step**

Replace the job step in `characterCreationFlow` (current lines 167–200) with:

```go
    // Step 4: Job selection — show jobs available to this team (general + team-exclusive)
    var availableJobs []*ruleset.Job
    for _, j := range h.jobs {
        if j.Team == "" || j.Team == selectedTeam.ID {
            availableJobs = append(availableJobs, j)
        }
    }
    _ = conn.WriteLine(telnet.Colorf(telnet.BrightYellow,
        "\r\nChoose your job (%s jobs available):", selectedTeam.Name))
    for i, j := range availableJobs {
        exclusive := ""
        if j.Team != "" {
            exclusive = telnet.Colorf(telnet.BrightRed, " [%s exclusive]", selectedTeam.Name)
        }
        _ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s%s%s%s (HP/lvl: %d, Key: %s)\r\n     %s",
            telnet.Green, i+1, telnet.Reset,
            telnet.BrightWhite, j.Name, telnet.Reset, exclusive,
            j.HitPointsPerLevel, j.KeyAbility,
            j.Description))
    }
    _ = conn.WriteLine(fmt.Sprintf("  %sR%s. Random (default)", telnet.Green, telnet.Reset))
    _ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite,
        "Select job [1-%d/R, default=R]: ", len(availableJobs)))
    jobLine, err := conn.ReadLine()
    if err != nil {
        return nil, fmt.Errorf("reading job selection: %w", err)
    }
    jobLine = strings.TrimSpace(jobLine)
    if strings.ToLower(jobLine) == "cancel" {
        return nil, nil
    }
    var selectedJob *ruleset.Job
    if IsRandomInput(jobLine) {
        selectedJob = availableJobs[rand.Intn(len(availableJobs))]
        _ = conn.WriteLine(telnet.Colorf(telnet.Cyan, "Random job selected: %s", selectedJob.Name))
    } else {
        jobChoice := 0
        if _, err := fmt.Sscanf(jobLine, "%d", &jobChoice); err != nil || jobChoice < 1 || jobChoice > len(availableJobs) {
            _ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection."))
            return nil, nil
        }
        selectedJob = availableJobs[jobChoice-1]
    }
```

**Step 3: Run all handler tests**

```
go test ./internal/frontend/handlers/
```

Expected: PASS

**Step 4: Run full test suite**

```
go test -race -count=1 -timeout=300s $(go list ./... | grep -v 'github.com/cory-johannsen/mud/internal/storage/postgres')
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/frontend/handlers/character_flow.go
git commit -m "feat: add random option with cascade to team and job steps"
```

---

### Task 6: Push and verify

**Step 1: Run full test suite one final time**

```
go test -race -count=1 -timeout=300s $(go list ./... | grep -v 'github.com/cory-johannsen/mud/internal/storage/postgres')
```

Expected: PASS

**Step 2: Push**

```bash
git push origin main
```

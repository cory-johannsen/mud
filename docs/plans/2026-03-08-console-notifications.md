# Console Notifications Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Notify players every time a skill is used automatically (with full roll detail) and every time XP is awarded (with amount and reason).

**Architecture:** Two targeted changes — `internal/game/xp/service.go` prepends XP grant messages to its return values; `internal/gameserver/grpc_service.go` builds a skill check detail line and adds it to the messages sent to the player. Three tasks total.

**Tech Stack:** Go, existing proto/push infrastructure.

---

## Background

### Current XP flow
`AwardRoomDiscovery(ctx, sess, charID)` and `AwardSkillCheck(ctx, sess, dc, isCrit, charID)` return `([]string, error)`. Currently they return `nil, nil` when no level-up occurs, so silent XP awards are invisible to the player. They return level-up announcement messages only on level-up.

Kill XP is already notified at the call site in `combat_handler.go` — no change needed there.

### Current skill check flow
`applyRoomSkillChecks` and `applyNPCSkillChecks` in `grpc_service.go` resolve skill checks but only append `outcome.Message` from YAML — no mechanical detail line. The `roll`, `amod`, `result.Total`, and `result.Outcome` are all available locally.

### Skill name display
`trigger.Skill` is a snake_case ID like `"parkour"` or `"tech_lore"`. Use a helper `skillDisplayName(id string) string` that title-cases words: `"parkour"` → `"Parkour"`, `"tech_lore"` → `"Tech Lore"`. Place it in `grpc_service.go`.

### Outcome display
`result.Outcome.String()` returns `"crit_success"`, `"success"`, `"failure"`, `"crit_failure"`. Map these to human-readable: `"critical success"`, `"success"`, `"failure"`, `"critical failure"`. Use a helper `outcomeDisplayName(o skillcheck.CheckOutcome) string`.

---

## Task 1: Add XP grant messages to AwardRoomDiscovery and AwardSkillCheck

**Files:**
- Modify: `internal/game/xp/service.go`
- Modify: `internal/game/xp/service_test.go`

### Step 1: Write failing tests

In `internal/game/xp/service_test.go`, add these tests after the existing ones:

```go
func TestService_AwardRoomDiscovery_ReturnsGrantMessage(t *testing.T) {
	cfg := defaultTestConfig()
	svc := NewService(cfg, &noopSaver{})
	sess := &session.PlayerSession{Level: 1, Experience: 0, MaxHP: 10, CurrentHP: 10}
	msgs, err := svc.AwardRoomDiscovery(context.Background(), sess, 0)
	require.NoError(t, err)
	require.NotEmpty(t, msgs, "must return at least the XP grant message")
	assert.Contains(t, msgs[0], "You gain", "first message must be XP grant")
	assert.Contains(t, msgs[0], "XP", "first message must mention XP")
	assert.Contains(t, msgs[0], "discovering a new room", "first message must state reason")
}

func TestService_AwardSkillCheck_ReturnsGrantMessage(t *testing.T) {
	cfg := defaultTestConfig()
	svc := NewService(cfg, &noopSaver{})
	sess := &session.PlayerSession{Level: 1, Experience: 0, MaxHP: 10, CurrentHP: 10}
	msgs, err := svc.AwardSkillCheck(context.Background(), sess, "parkour", 14, false, 0)
	require.NoError(t, err)
	require.NotEmpty(t, msgs, "must return at least the XP grant message")
	assert.Contains(t, msgs[0], "You gain", "first message must be XP grant")
	assert.Contains(t, msgs[0], "XP", "first message must mention XP")
	assert.Contains(t, msgs[0], "parkour", "first message must mention skill name")
}

func TestPropertyService_AwardRoomDiscovery_GrantMessageAlwaysFirst(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cfg := defaultTestConfig()
		svc := NewService(cfg, &noopSaver{})
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		xp := rapid.IntRange(0, 999).Draw(rt, "xp")
		sess := &session.PlayerSession{Level: level, Experience: xp, MaxHP: 10, CurrentHP: 10}
		msgs, err := svc.AwardRoomDiscovery(context.Background(), sess, 0)
		if err != nil {
			rt.Fatal(err)
		}
		if len(msgs) == 0 {
			rt.Fatal("expected at least one message (XP grant)")
		}
		if !strings.Contains(msgs[0], "You gain") {
			rt.Fatalf("first message must be XP grant, got: %q", msgs[0])
		}
	})
}
```

Note: `AwardSkillCheck` now takes `skillName string` as the second parameter (before `dc`). The existing tests that call `AwardSkillCheck(ctx, sess, dc, isCrit, charID)` will fail to compile — update them in Step 3 below.

### Step 2: Run tests to verify they fail

```bash
go test ./internal/game/xp/... -run "TestService_AwardRoomDiscovery_ReturnsGrantMessage|TestService_AwardSkillCheck_ReturnsGrantMessage|TestPropertyService_AwardRoomDiscovery" -v
```
Expected: compile error or FAIL — `AwardSkillCheck` signature doesn't match yet.

### Step 3: Update `AwardRoomDiscovery` and `AwardSkillCheck` in `internal/game/xp/service.go`

Replace `AwardRoomDiscovery`:

```go
// AwardRoomDiscovery awards XP for entering a previously unseen room.
// Always returns at least one message (the XP grant notification).
//
// Precondition: sess must be non-nil.
// Postcondition: sess.Experience updated; SaveProgress called on level-up.
func (s *Service) AwardRoomDiscovery(ctx context.Context, sess *session.PlayerSession, characterID int64) ([]string, error) {
	xpAmount := s.cfg.Awards.NewRoomXP
	levelMsgs, err := s.award(ctx, sess, characterID, xpAmount)
	if err != nil {
		return nil, err
	}
	grant := fmt.Sprintf("You gain %d XP for discovering a new room.", xpAmount)
	return append([]string{grant}, levelMsgs...), nil
}
```

Replace `AwardSkillCheck` — note the new `skillName string` parameter as second argument:

```go
// AwardSkillCheck awards XP for a successful or crit-successful skill check.
// Always returns at least one message (the XP grant notification).
// Set isCrit=true for crit_success outcomes.
//
// Precondition: sess must be non-nil; dc >= 0; skillName must be non-empty.
// Postcondition: sess.Experience updated; SaveProgress called on level-up.
func (s *Service) AwardSkillCheck(ctx context.Context, sess *session.PlayerSession, skillName string, dc int, isCrit bool, characterID int64) ([]string, error) {
	base := s.cfg.Awards.SkillCheckSuccessXP
	if isCrit {
		base = s.cfg.Awards.SkillCheckCritSuccessXP
	}
	xpAmount := base + dc*s.cfg.Awards.SkillCheckDCMultiplier
	levelMsgs, err := s.award(ctx, sess, characterID, xpAmount)
	if err != nil {
		return nil, err
	}
	grant := fmt.Sprintf("You gain %d XP for the %s check.", xpAmount, skillName)
	return append([]string{grant}, levelMsgs...), nil
}
```

Also update the two existing `AwardSkillCheck` tests in the file to pass the skill name as the second argument:

Find:
```go
_, err := svc.AwardSkillCheck(context.Background(), sess, 14, false, 0)
```
Replace with:
```go
_, err := svc.AwardSkillCheck(context.Background(), sess, "parkour", 14, false, 0)
```

(Two occurrences: `TestService_AwardSkillCheck_Success` and `TestService_AwardSkillCheck_CritSuccess`.)

Also add `"strings"` to the import in `service_test.go` if not already present.

### Step 4: Run tests to verify they pass

```bash
go test ./internal/game/xp/... -v
```
Expected: all PASS

### Step 5: Fix compile errors in `grpc_service.go` call sites (minimal fix — just add the skill name arg)

`grpc_service.go` has two calls to `AwardSkillCheck` that will now fail to compile. Add `trigger.Skill` as the second argument to both:

Line ~1423:
```go
if xpMsgs, xpErr := s.xpSvc.AwardSkillCheck(context.Background(), sess, trigger.Skill, trigger.DC, isCrit, sess.CharacterID); xpErr != nil {
```

Line ~1504:
```go
if xpMsgs, xpErr := s.xpSvc.AwardSkillCheck(context.Background(), sess, trigger.Skill, trigger.DC, isCrit, sess.CharacterID); xpErr != nil {
```

### Step 6: Run full suite to verify compile

```bash
go test ./... -count=1 2>&1 | tail -10
```
Expected: all PASS

### Step 7: Commit

```bash
git add internal/game/xp/service.go internal/game/xp/service_test.go internal/gameserver/grpc_service.go
git commit -m "feat: XP award notifications for room discovery and skill checks"
```

---

## Task 2: Add skill check detail notification in applyRoomSkillChecks and applyNPCSkillChecks

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

### Step 1: Write failing test

There are no direct unit tests for `applyRoomSkillChecks` / `applyNPCSkillChecks`. Add tests for the two new helpers (`skillDisplayName` and `outcomeDisplayName`) in `internal/gameserver/grpc_service_test.go`:

```go
func TestSkillDisplayName(t *testing.T) {
	cases := []struct {
		id   string
		want string
	}{
		{"parkour", "Parkour"},
		{"tech_lore", "Tech Lore"},
		{"hard_look", "Hard Look"},
		{"smooth_talk", "Smooth Talk"},
		{"rep", "Rep"},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			got := skillDisplayName(tc.id)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestOutcomeDisplayName(t *testing.T) {
	assert.Equal(t, "critical success", outcomeDisplayName(skillcheck.CritSuccess))
	assert.Equal(t, "success", outcomeDisplayName(skillcheck.Success))
	assert.Equal(t, "failure", outcomeDisplayName(skillcheck.Failure))
	assert.Equal(t, "critical failure", outcomeDisplayName(skillcheck.CritFailure))
}
```

Also add a property test:

```go
func TestPropertySkillDisplayName_NeverEmpty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.StringMatching(`[a-z][a-z_]{0,19}`).Draw(rt, "id")
		result := skillDisplayName(id)
		if result == "" {
			rt.Fatal("skillDisplayName must never return empty string")
		}
	})
}
```

You will need these imports in `grpc_service_test.go` (add if not present):
```go
"github.com/cory-johannsen/mud/internal/game/skillcheck"
"pgregory.net/rapid"
```

### Step 2: Run tests to verify they fail

```bash
go test ./internal/gameserver/... -run "TestSkillDisplayName|TestOutcomeDisplayName|TestPropertySkillDisplayName" -v
```
Expected: compile error — functions not defined yet.

### Step 3: Add helpers and update applyRoomSkillChecks/applyNPCSkillChecks

Add these two helpers anywhere in `internal/gameserver/grpc_service.go` (e.g., near `abilityModFrom`):

```go
// skillDisplayName converts a snake_case skill ID to a title-cased display name.
//
// Precondition: id must be non-empty.
// Postcondition: Returns a non-empty title-cased string.
func skillDisplayName(id string) string {
	parts := strings.Split(id, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// outcomeDisplayName converts a CheckOutcome to a human-readable string.
//
// Postcondition: Returns one of "critical success", "success", "failure", "critical failure".
func outcomeDisplayName(o skillcheck.CheckOutcome) string {
	switch o {
	case skillcheck.CritSuccess:
		return "critical success"
	case skillcheck.Success:
		return "success"
	case skillcheck.Failure:
		return "failure"
	case skillcheck.CritFailure:
		return "critical failure"
	default:
		return "unknown"
	}
}
```

Ensure `"strings"` is in the import block of `grpc_service.go` (it likely already is; verify with `grep '"strings"' internal/gameserver/grpc_service.go`).

Then in `applyRoomSkillChecks`, after `result := skillcheck.Resolve(...)` and before appending `outcome.Message`, prepend the detail line:

Find the block (around line 1410):
```go
		result := skillcheck.Resolve(roll, amod, rank, trigger.DC, trigger)

		outcome := trigger.Outcomes.ForOutcome(result.Outcome)
		if outcome != nil {
			if outcome.Message != "" {
				msgs = append(msgs, outcome.Message)
			}
```

Replace with:
```go
		result := skillcheck.Resolve(roll, amod, rank, trigger.DC, trigger)

		detail := fmt.Sprintf("%s check (DC %d): rolled %d+%d=%d — %s.",
			skillDisplayName(trigger.Skill), trigger.DC, roll, amod, result.Total, outcomeDisplayName(result.Outcome))
		msgs = append(msgs, detail)

		outcome := trigger.Outcomes.ForOutcome(result.Outcome)
		if outcome != nil {
			if outcome.Message != "" {
				msgs = append(msgs, outcome.Message)
			}
```

Apply the identical change in `applyNPCSkillChecks` (around line 1488). Find the equivalent block:
```go
			result := skillcheck.Resolve(roll, amod, rank, trigger.DC, trigger)

			outcome := trigger.Outcomes.ForOutcome(result.Outcome)
			if outcome != nil {
				if outcome.Message != "" {
					msgs = append(msgs, outcome.Message)
				}
```

Replace with:
```go
			result := skillcheck.Resolve(roll, amod, rank, trigger.DC, trigger)

			detail := fmt.Sprintf("%s check (DC %d): rolled %d+%d=%d — %s.",
				skillDisplayName(trigger.Skill), trigger.DC, roll, amod, result.Total, outcomeDisplayName(result.Outcome))
			msgs = append(msgs, detail)

			outcome := trigger.Outcomes.ForOutcome(result.Outcome)
			if outcome != nil {
				if outcome.Message != "" {
					msgs = append(msgs, outcome.Message)
				}
```

### Step 4: Run tests

```bash
go test ./internal/gameserver/... -run "TestSkillDisplayName|TestOutcomeDisplayName|TestPropertySkillDisplayName" -v
```
Expected: all PASS

### Step 5: Run full suite

```bash
go test ./... -count=1 2>&1 | tail -10
```
Expected: all PASS

### Step 6: Update FEATURES.md

In `docs/requirements/FEATURES.md`, find:
```
- [ ] Console notifications
  - [ ] The player should be notified in the console every time a skill is used automatically
  - [ ] The player should be notified in the console every time XP is awarded and why
```

Change all three lines to `[x]`.

### Step 7: Commit

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_test.go docs/requirements/FEATURES.md
git commit -m "feat: skill check detail notifications and mark console notifications complete"
```

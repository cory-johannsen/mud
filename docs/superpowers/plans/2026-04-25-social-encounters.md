# Social Encounters — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the PF2E **Influence** subsystem as `SocialEncounter` content + a per-(character, npc) session runtime. Players spend rounds choosing one influence skill at a time; the shared DoS resolver from #252 grades each roll; tally accumulates with NPC-bias multipliers (`receptive` ×2 success / `resistant` ×0.5 success and ×2 failure / `neutral` ×1). Encounter ends on threshold success, threshold failure, round-limit, or walk-away. Outcomes apply via the same effect-block dispatch as #255, extended with `set_disposition`, `shift_disposition`, `change_faction_rep`, `unlock_quest`, `lock_quest`, `unlock_dialogue_topic`. Discovery (Recall Knowledge) reveals biases per (character, npc) and persists across sessions until the NPC's disposition changes.

**Spec:** [docs/superpowers/specs/2026-04-25-social-encounters.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-25-social-encounters.md) (PR [#284](https://github.com/cory-johannsen/mud/pull/284))

**Architecture:** Three layers, all reusing already-shipped or co-shipping primitives. (1) Content + loader (`internal/game/social/encounter/def.go`) — YAML schema validated against NPC templates, factions, quests, and Mud skills. (2) Service (`service.go`) — `Begin` / `Choose` / `WalkAway`; tally math in a small pure function `ApplyTally(dos, bias) (sucDelta, failDelta int)`; threshold scaling via `effective_threshold = max(2, base + floor((npcLvl - charLvl)/2))`; DoS computation imported from `internal/game/skillaction/dos.go` (shared with #252 / #255). (3) Effect dispatch (`effect.go`) extends `internal/game/exploration/challenge/effect.go` with the six new effect kinds; the disposition mutator persists to the NPC instance store; the faction-rep mutator calls `factionSvc.SaveRep`; the quest-unlock effect writes to a new per-(character, npc) overlay map consulted by `handleTalk`. Discovery is a one-shot RPC `BeginRecallNPCRequest` that rolls the player's best `discovery_skills` skill and sets a persistent flag in `character_npc_discoveries` until the NPC's disposition next changes (cleared by `set_disposition` / `shift_disposition`).

**Tech Stack:** Go (`internal/game/social/encounter/`, `internal/game/npc/`, `internal/storage/postgres/`, `internal/gameserver/`), `pgregory.net/rapid` for property tests, protobuf, telnet, React/TypeScript (`cmd/webclient/ui/src/game/social/`).

**Prerequisite:** None hard. #252 is a soft dep — the DoS function lives in `internal/game/skillaction/dos.go` and is shared. #255 is a soft dep — the effect-dispatch pattern is reused; the new effect kinds slot into the same registry. Faction service exists at `internal/game/faction/service.go`.

**Note on spec PR**: Spec is on PR #284, not yet merged. Plan PR depends on spec PR landing first.

---

## File Map

| Action | Path |
|--------|------|
| Create | `internal/game/social/encounter/def.go` |
| Create | `internal/game/social/encounter/def_test.go` |
| Create | `internal/game/social/encounter/service.go` |
| Create | `internal/game/social/encounter/service_test.go` |
| Create | `internal/game/social/encounter/store.go` |
| Create | `internal/game/social/encounter/effect.go` |
| Create | `internal/game/social/encounter/effect_test.go` |
| Create | `internal/game/social/encounter/testdata/rapid/TestEncounterProperty/` |
| Create | `internal/storage/postgres/npc_discoveries.go` |
| Create | `internal/storage/postgres/npc_discoveries_test.go` |
| Modify | `internal/game/npc/instance.go` (per-(char, npc) overlay maps for quests/topics) |
| Modify | `internal/gameserver/grpc_service.go` (`BeginRecallNPC`, `BeginSocial`, `ChooseSkill`, `WalkAway` RPCs) |
| Modify | `internal/gameserver/grpc_service_quest_giver.go` (`handleTalk` extension for "Begin social encounter") |
| Modify | `api/proto/game/v1/game.proto` (`SocialEncounterView`, `RoundResult`, encounter request messages) |
| Create | `cmd/webclient/ui/src/game/social/SocialEncounterModal.tsx` |
| Create | `cmd/webclient/ui/src/game/social/SocialEncounterModal.test.tsx` |
| Create | `migrations/NNN_character_npc_discoveries.up.sql`, `.down.sql` |
| Create | `content/social/encounters/` (3 exemplar YAML files) |
| Modify | `docs/architecture/social.md` (or new doc) |

---

### Task 1: Content schema + loader

**Files:**
- Create: `internal/game/social/encounter/def.go`
- Create: `internal/game/social/encounter/def_test.go`

- [ ] **Step 1: Failing tests** (SE-1, SE-3):

```go
func TestLoadEncounter_AllFieldsParse(t *testing.T) {
    def, err := encounter.Load([]byte(extractInformationYAML))
    require.NoError(t, err)
    require.Equal(t, "extract_info_from_jenkins", def.ID)
    require.Equal(t, "jenkins", def.NPCID)
    require.Equal(t, 4, def.InfluenceThreshold)
    require.Equal(t, 3, def.FailureThreshold)
    require.Equal(t, 6, def.RoundLimit)
    require.Equal(t, encounter.BiasReceptive, def.InfluenceSkills[0].Bias)
}

func TestLoadEncounter_RejectsUnknownNPC(t *testing.T) {
    _, err := encounter.Load(yamlWithNPCID("nonexistent_npc"))
    require.Error(t, err)
    require.Contains(t, err.Error(), "nonexistent_npc")
}

func TestLoadEncounter_RejectsUnknownQuestInUnlock(t *testing.T) {
    _, err := encounter.Load(yamlWithUnlockQuest("nonexistent_quest"))
    require.Error(t, err)
}

func TestLoadEncounter_RejectsUnknownFactionInRepChange(t *testing.T) {
    _, err := encounter.Load(yamlWithFactionDelta("not_a_faction"))
    require.Error(t, err)
}
```

- [ ] **Step 2: Implement** the schema:

```go
type Definition struct {
    ID                  string
    DisplayName         string
    Description         string
    NPCID               string
    Goal                string
    InfluenceThreshold  int
    FailureThreshold    int
    RoundLimit          int
    DiscoveryDC         DC
    DiscoverySkills     []string
    InfluenceSkills     []*InfluenceSkill
    Outcomes            map[OutcomeKind]*OutcomeBlock
}

type InfluenceSkill struct {
    Skill string
    Bias  Bias // Receptive | Neutral | Resistant
    DC    DC
}

type OutcomeKind int

const (
    OutcomeSuccess OutcomeKind = iota
    OutcomeFailure
    OutcomeWalkAway
)
```

`DC` and `OutcomeBlock` are imported from the shared challenge package (#255) — same shape, same effect interface. Effect implementations new to this package:

```go
type SetDisposition       struct{ Value npc.Disposition }
type ShiftDisposition     struct{ Delta int }
type ChangeFactionRep     struct{ FactionID string; Delta int }
type UnlockQuest          struct{ QuestID string }
type LockQuest            struct{ QuestID string }
type UnlockDialogueTopic  struct{ TopicID string }
```

- [ ] **Step 3:** Loader entry reads `content/social/encounters/*.yaml` and validates references at startup.

---

### Task 2: Per-(char, npc) discovery persistence

**Files:**
- Create: `migrations/NNN_character_npc_discoveries.up.sql`, `.down.sql`
- Create: `internal/storage/postgres/npc_discoveries.go`
- Create: `internal/storage/postgres/npc_discoveries_test.go`

- [ ] **Step 1: Author migration** (SE-7):

```sql
CREATE TABLE character_npc_discoveries (
    character_id        TEXT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    npc_id              TEXT NOT NULL,
    biases_discovered   BOOLEAN NOT NULL DEFAULT FALSE,
    discovered_at       TIMESTAMPTZ,
    PRIMARY KEY (character_id, npc_id)
);
```

- [ ] **Step 2: Failing tests**:

```go
func TestDiscoveries_RoundTrip(t *testing.T) {
    s := newPGStore(t)
    require.False(t, mustBool(s.IsBiasesDiscovered("c1", "jenkins")))
    require.NoError(t, s.MarkDiscovered("c1", "jenkins"))
    require.True(t, mustBool(s.IsBiasesDiscovered("c1", "jenkins")))
}

func TestDiscoveries_ScopedPerCharacter(t *testing.T) {
    s := newPGStore(t)
    s.MarkDiscovered("c1", "jenkins")
    require.False(t, mustBool(s.IsBiasesDiscovered("c2", "jenkins")))
}

func TestDiscoveries_ClearOnDispositionChange(t *testing.T) {
    s := newPGStore(t)
    s.MarkDiscovered("c1", "jenkins")
    require.NoError(t, s.ClearDiscovery("c1", "jenkins"))
    require.False(t, mustBool(s.IsBiasesDiscovered("c1", "jenkins")))
}
```

- [ ] **Step 3: Implement** the repository.

---

### Task 3: Tally math + DoS share + threshold scaling

**Files:**
- Create: `internal/game/social/encounter/service.go` (initial skeleton + helpers)
- Modify: `internal/game/social/encounter/service_test.go`

- [ ] **Step 1: Failing tests** (SE-10, SE-12):

```go
func TestApplyTally_BiasMultipliers(t *testing.T) {
    cases := []struct{
        dos    skillaction.DegreeOfSuccess
        bias   encounter.Bias
        wantS  int
        wantF  int
    }{
        {skillaction.CritSuccess, encounter.BiasReceptive,  4, 0}, // 2 base * 2 receptive
        {skillaction.Success,     encounter.BiasReceptive,  2, 0},
        {skillaction.Success,     encounter.BiasNeutral,    1, 0},
        {skillaction.Success,     encounter.BiasResistant,  0, 0}, // floor(0.5*1) = 0
        {skillaction.Failure,     encounter.BiasResistant,  0, 2}, // 1 base * 2 resistant
        {skillaction.Failure,     encounter.BiasNeutral,    0, 1},
        {skillaction.CritFailure, encounter.BiasReceptive,  0, 2},
        {skillaction.CritFailure, encounter.BiasResistant,  0, 4},
    }
    for _, c := range cases {
        s, f := encounter.ApplyTally(c.dos, c.bias)
        require.Equal(t, c.wantS, s, "successes for %v %v", c.dos, c.bias)
        require.Equal(t, c.wantF, f, "failures for %v %v", c.dos, c.bias)
    }
}

func TestEffectiveThreshold_Scales(t *testing.T) {
    require.Equal(t, 4, encounter.EffectiveThreshold(4 /*base*/, 5 /*npc*/, 5 /*char*/))
    require.Equal(t, 6, encounter.EffectiveThreshold(4, 9 /*npc*/, 5 /*char*/))
    require.Equal(t, 8 /*cap +4*/, encounter.EffectiveThreshold(4, 20, 5))
    require.Equal(t, 2 /*floor*/, encounter.EffectiveThreshold(4, 1, 20))
}
```

- [ ] **Step 2: Implement**:

```go
func ApplyTally(dos skillaction.DegreeOfSuccess, bias Bias) (successDelta, failureDelta int) {
    var s, f int
    switch dos {
    case skillaction.CritSuccess: s = 2
    case skillaction.Success:     s = 1
    case skillaction.Failure:     f = 1
    case skillaction.CritFailure: f = 2
    }
    switch bias {
    case BiasReceptive:
        s *= 2
    case BiasResistant:
        s = s / 2 // integer floor
        f *= 2
    }
    return s, f
}

func EffectiveThreshold(base, npcLevel, charLevel int) int {
    eff := base + (npcLevel-charLevel)/2
    if eff < 2 { eff = 2 }
    if eff > base+4 { eff = base+4 }
    return eff
}
```

---

### Task 4: `Begin` / `Choose` / `WalkAway` service

**Files:**
- Modify: `internal/game/social/encounter/service.go`
- Modify: `internal/game/social/encounter/service_test.go`

- [ ] **Step 1: Failing tests** (SE-8, SE-9, SE-11, SE-13):

```go
func TestBegin_ReturnsRound0View(t *testing.T) {
    svc := newSvc(t, withEncounter(extractInformation))
    sess, _ := svc.Begin("c1", "jenkins-uid", "extract_info")
    require.Equal(t, 0, sess.RoundIndex)
    require.Equal(t, []string{"smooth_talk", "grift", "intimidate"}, skillIDs(sess.AvailableSkills))
    require.True(t, sess.WalkAwayAllowed)
    require.Equal(t, 0, sess.Successes)
    require.Equal(t, 0, sess.Failures)
}

func TestChoose_IncrementsTallyAndAdvances(t *testing.T) {
    svc := newSvc(t, withEncounter(extractInformation), withDice(15))
    sess, _ := svc.Begin("c1", "jenkins-uid", "extract_info")
    res, err := svc.Choose(sess.ID, "smooth_talk")
    require.NoError(t, err)
    require.Equal(t, skillaction.Success, res.DoS)
    require.Equal(t, 1, res.View.Successes)
    require.Equal(t, 1, res.View.RoundIndex)
}

func TestChoose_TerminatesOnInfluenceThreshold(t *testing.T) {
    svc := newSvc(t, withEncounter(extractInformationLowThreshold(2)), withDice(20))
    sess, _ := svc.Begin("c1", "jenkins-uid", "extract_info")
    svc.Choose(sess.ID, "smooth_talk") // crit success → 2 + receptive
    require.True(t, svc.SessionTerminated(sess.ID))
    require.Equal(t, encounter.OutcomeSuccess, svc.LastOutcome(sess.ID))
}

func TestChoose_TerminatesOnFailureThreshold(t *testing.T) { ... }
func TestChoose_TerminatesOnRoundLimit(t *testing.T) { ... }
func TestWalkAway_AppliesWalkAwayOutcome(t *testing.T) { ... }
func TestCancellationOnCombatEntry_NoOutcomeApplied(t *testing.T) {
    svc := newSvc(t, withEncounter(extractInformation))
    sess, _ := svc.Begin("c1", "jenkins-uid", "extract_info")
    svc.OnCombatStarted("c1", "jenkins-uid")
    require.True(t, svc.SessionCancelled(sess.ID))
    require.Empty(t, svc.AppliedEffects(sess.ID), "SE-13 cancellation produces no effects")
}
```

- [ ] **Step 2: Implement** the service. Sessions held in memory keyed by `fmt.Sprintf("%s:%s", charID, npcUID)`. The `OnCombatStarted` / `OnRoomLeft` / `OnDisconnect` hooks are called from the gameserver entry points.

```go
func (s *Service) Choose(sessID, skillID string) (RoundResult, error) {
    sess := s.sessions[sessID]
    sk := findSkill(sess.Def, skillID)
    roll := s.dice.Roll("1d20")
    bonus := s.skillBonus(sess.CharID, skillID)
    dc := evaluateDC(sess, sk.DC)
    dos := skillaction.DoS(roll, bonus, dc)
    sDelta, fDelta := ApplyTally(dos, sk.Bias)
    sess.Successes += sDelta
    sess.Failures  += fDelta
    sess.RoundIndex++

    var outcome OutcomeKind
    switch {
    case sess.Successes >= sess.EffectiveThreshold:
        outcome = OutcomeSuccess
    case sess.Failures >= sess.Def.FailureThreshold || sess.RoundIndex >= sess.Def.RoundLimit:
        outcome = OutcomeFailure
    default:
        return RoundResult{DoS: dos, View: viewOf(sess)}, nil
    }
    s.applyOutcome(sess, outcome)
    return RoundResult{DoS: dos, View: terminalViewOf(sess), Outcome: &outcome}, nil
}
```

---

### Task 5: Effect dispatch — six new effect kinds

**Files:**
- Create: `internal/game/social/encounter/effect.go`
- Create: `internal/game/social/encounter/effect_test.go`

- [ ] **Step 1: Failing tests** (SE-14..18):

```go
func TestEffect_SetDispositionPersists(t *testing.T) {
    npcStore := mockNPCStore()
    challenge.ApplyEffect(ctx, &encounter.SetDisposition{Value: npc.DispositionFriendly}, npcStore)
    require.Equal(t, npc.DispositionFriendly, npcStore.Disposition("jenkins-uid"))
}

func TestEffect_ShiftDispositionClampsAtBounds(t *testing.T) {
    npcStore := mockNPCStoreWith("jenkins-uid", npc.DispositionFriendly)
    challenge.ApplyEffect(ctx, &encounter.ShiftDisposition{Delta: 5}, npcStore)
    require.Equal(t, npc.DispositionFriendly, npcStore.Disposition("jenkins-uid"), "clamped at top")
}

func TestEffect_ChangeFactionRepCallsService(t *testing.T) {
    fSvc := mockFactionSvc()
    challenge.ApplyEffect(ctx, &encounter.ChangeFactionRep{FactionID: "fixers", Delta: 5}, fSvc)
    require.Equal(t, []factionCall{{Faction: "fixers", Delta: 5}}, fSvc.Calls)
}

func TestEffect_UnlockQuestAddsToOverlay(t *testing.T) {
    overlay := newOverlay()
    challenge.ApplyEffect(ctx, &encounter.UnlockQuest{QuestID: "deliver_the_chip"}, overlay)
    require.Contains(t, overlay.QuestsForNPC("c1", "jenkins-uid"), "deliver_the_chip")
}

func TestEffect_LockQuestRemovesFromOverlay(t *testing.T) { ... }
func TestEffect_UnlockDialogueTopicSetsFlag(t *testing.T) { ... }
```

- [ ] **Step 2: Implement** each effect's `Apply` method using the existing dispatch:

```go
func (e *SetDisposition) Apply(ctx Context) {
    ctx.NPCStore.SetDisposition(ctx.NPCUID, e.Value)
    ctx.DiscoveryStore.ClearDiscovery(ctx.CharID, ctx.NPCID) // SE-Q1: per-disposition cycle
}

func (e *ShiftDisposition) Apply(ctx Context) {
    cur := ctx.NPCStore.Disposition(ctx.NPCUID)
    next := clampDisposition(int(cur) + e.Delta)
    ctx.NPCStore.SetDisposition(ctx.NPCUID, next)
    if next != cur {
        ctx.DiscoveryStore.ClearDiscovery(ctx.CharID, ctx.NPCID)
    }
}

func (e *ChangeFactionRep) Apply(ctx Context) {
    ctx.FactionSvc.SaveRep(ctx.CharID, e.FactionID, ctx.FactionSvc.Rep(ctx.CharID, e.FactionID)+e.Delta)
}

func (e *UnlockQuest) Apply(ctx Context) {
    ctx.NPCOverlay.AddQuest(ctx.CharID, ctx.NPCID, e.QuestID)
}

func (e *LockQuest) Apply(ctx Context) {
    ctx.NPCOverlay.RemoveQuest(ctx.CharID, ctx.NPCID, e.QuestID)
}

func (e *UnlockDialogueTopic) Apply(ctx Context) {
    ctx.NPCOverlay.AddTopic(ctx.CharID, ctx.NPCID, e.TopicID)
}
```

- [ ] **Step 3:** Register each new effect kind in the shared challenge effect registry (`internal/game/exploration/challenge/effect.go`) so both packages share the dispatch table.

---

### Task 6: Per-(char, npc) overlay store + `handleTalk` extension

**Files:**
- Modify: `internal/game/npc/instance.go`
- Modify: `internal/gameserver/grpc_service_quest_giver.go`
- Modify: `internal/gameserver/grpc_service_quest_giver_test.go`

- [ ] **Step 1: Failing tests** (SE-17, SE-26):

```go
func TestNPCOverlay_QuestUnlockShowsInTalk(t *testing.T) {
    overlay := newOverlay()
    overlay.AddQuest("c1", "jenkins-uid", "deliver_the_chip")
    out := s.handleTalk(t, "c1", "jenkins-uid")
    require.Contains(t, out.OfferedQuests, "deliver_the_chip")
}

func TestNPCOverlay_QuestLockHidesFromTalk(t *testing.T) {
    base := npcWithBaseQuests("default_quest")
    overlay := newOverlay()
    overlay.RemoveQuest("c1", "jenkins-uid", "default_quest")
    out := s.handleTalk(t, "c1", "jenkins-uid", base)
    require.NotContains(t, out.OfferedQuests, "default_quest")
}

func TestHandleTalk_OffersBeginSocialEncounterWhenAvailable(t *testing.T) {
    out := s.handleTalk(t, "c1", "jenkins-uid", withSocialEncounter("extract_info"))
    require.Contains(t, out.Options, "Begin social encounter: Extract Information")
}
```

- [ ] **Step 2: Implement** the overlay (in-memory + persisted via a small `character_npc_quest_overlay` table; or for v1 in-memory only, since session-scoped behaviour is acceptable per discussion). Default plan: persist via PG.

```sql
CREATE TABLE character_npc_quest_overlay (
    character_id TEXT NOT NULL,
    npc_id       TEXT NOT NULL,
    quest_id     TEXT NOT NULL,
    locked       BOOLEAN NOT NULL,
    PRIMARY KEY (character_id, npc_id, quest_id)
);
```

- [ ] **Step 3: Extend `handleTalk`** to:
  - Build the offered-quests list from the base NPC pool plus overlay (added quests minus locked quests).
  - When the NPC has a social encounter declared and the player has not yet completed it (or it is repeatable), include a "Begin social encounter: <display_name>" option in the talk response.

---

### Task 7: gRPC RPCs for encounters + recall

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_social_test.go`

- [ ] **Step 1: Add proto messages** (SE-5, SE-19, SE-20):

```proto
message BeginRecallNPCRequest { string character_id = 1; string npc_uid = 2; }
message BeginRecallNPCResponse { bool success = 1; string narrative = 2; }

message BeginSocialRequest    { string character_id = 1; string npc_uid = 2; string encounter_id = 3; }
message BeginSocialResponse   { SocialEncounterView view = 1; }

message ChooseSkillRequest    { string session_id = 1; string skill = 2; }
message ChooseSkillResponse   { RoundResult result = 1; SocialEncounterView view = 2; }

message WalkAwayRequest       { string session_id = 1; }
message WalkAwayResponse      { SocialEncounterView view = 1; }

message SocialEncounterView {
  string  session_id = 1;
  string  goal = 2;
  int32   round_index = 3;
  int32   round_limit = 4;
  int32   successes = 5;
  int32   failures = 6;
  int32   influence_threshold = 7;
  int32   failure_threshold = 8;
  bool    walk_away_allowed = 9;
  bool    biases_discovered = 10;
  repeated InfluenceSkillView available_skills = 11;
  RoundResult last_round = 12;
  optional string outcome = 13; // "success" | "failure" | "walk_away" when terminal
}

message InfluenceSkillView {
  string skill = 1;
  string label = 2;
  string bias  = 3;  // "receptive" | "neutral" | "resistant" | "" (hidden until discovered)
  string dc_summary = 4;
}

message RoundResult {
  int32 roll = 1;
  int32 bonus = 2;
  int32 dc = 3;
  string dos = 4;
  int32 success_delta = 5;
  int32 failure_delta = 6;
  string narrative = 7;
}
```

- [ ] **Step 2: Failing handler tests** for each of the four RPCs.

- [ ] **Step 3: Implement** the handlers as thin wrappers over `encounter.Service`. `BeginRecallNPC` rolls the player's best `discovery_skills` skill vs the encounter's `discovery_dc` and writes to `character_npc_discoveries` on success. `BiasesDiscovered` flag is consulted when building `InfluenceSkillView.bias`.

---

### Task 8: Telnet UX — `social` + `recall` commands + 90s timeout

**Files:**
- Create: `internal/frontend/telnet/social_handler.go`
- Create: `internal/frontend/telnet/social_handler_test.go`

- [ ] **Step 1: Failing tests** (SE-19..22):

```go
func TestSocialCommand_StartsEncounter(t *testing.T) {
    h := newHandler(t, withSocialNPC("jenkins"))
    out := h.Run("social jenkins")
    require.Contains(t, out, "Round 1/6")
    require.Contains(t, out, "1) Smooth Talk")
}

func TestSocialCommand_PicksSkillByNumber(t *testing.T) { ... }
func TestSocialCommand_ZeroIsWalkAway(t *testing.T) { ... }

func TestSocialCommand_TimeoutWalksAway(t *testing.T) {
    h := newHandler(t, withSocialNPC("jenkins"))
    h.Run("social jenkins")
    h.AdvanceTime(91 * time.Second)
    require.True(t, h.SessionWalkedAway())
}

func TestSocialCommand_PerRoundNarrativeIncludesRollDetails(t *testing.T) {
    h := newHandler(t, withSocialNPC("jenkins"), withDice(17))
    h.Run("social jenkins")
    h.Run("1")
    require.Regexp(t, `Roll 17 \+ \d+ = \d+ vs DC \d+ → (success|failure|critical success|critical failure)`, h.LastOutput())
}

func TestRecallCommand_RollsDiscoveryAndAnnotatesBiases(t *testing.T) {
    h := newHandler(t, withSocialNPC("jenkins"), withDice(20))
    h.Run("recall jenkins")
    h.Run("social jenkins")
    require.Contains(t, h.LastOutput(), "(receptive)") // bias annotation now visible
}
```

- [ ] **Step 2: Implement** the `social <npc_name>` and `recall <npc_name>` commands. The session timer resets each round; combat-entry / room-leave dispatches `Service.OnCombatStarted` / `OnRoomLeft` to cancel.

---

### Task 9: Web `SocialEncounterModal`

**Files:**
- Create: `cmd/webclient/ui/src/game/social/SocialEncounterModal.tsx`
- Create: `cmd/webclient/ui/src/game/social/SocialEncounterModal.test.tsx`

- [ ] **Step 1: Failing component tests** (SE-23, SE-24, SE-25):

```ts
test("modal renders skill buttons + tally bars + walk-away", () => {
  render(<SocialEncounterModal view={twoSkillView} />);
  expect(screen.getByRole("button", { name: /Smooth Talk/i })).toBeVisible();
  expect(screen.getByLabelText("Successes")).toHaveValue(0);
  expect(screen.getByLabelText("Failures")).toHaveValue(0);
  expect(screen.getByRole("button", { name: /walk away/i })).toBeVisible();
});

test("bias annotations only appear when biases_discovered", () => {
  const { rerender } = render(<SocialEncounterModal view={withBiasesUndiscovered} />);
  expect(screen.queryByText(/receptive/i)).toBeNull();
  rerender(<SocialEncounterModal view={withBiasesDiscovered} />);
  expect(screen.getByText(/receptive/i)).toBeVisible();
});

test("recall knowledge button calls BeginRecallNPC", () => {
  const dispatch = jest.fn();
  render(<SocialEncounterModal dispatch={dispatch} view={preFirstRoundView} />);
  fireEvent.click(screen.getByRole("button", { name: /Recall Knowledge/i }));
  expect(dispatch).toHaveBeenCalledWith(expect.objectContaining({ type: "BeginRecallNPC" }));
});
```

- [ ] **Step 2: Implement** the modal. Tally bars use the same progress-bar primitive as the existing combat UI. Recall Knowledge button is visible only when `biases_discovered == false` AND `round_index == 0`.

---

### Task 10: Property tests

**Files:**
- Create: `internal/game/social/encounter/testdata/rapid/TestEncounterProperty/`
- Modify: `internal/game/social/encounter/service_test.go`

- [ ] **Step 1: Property tests** (SE-29):

```go
func TestProperty_Determinism(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        seed := rapid.IntRange(1, 1000000).Draw(t, "seed")
        a := simulateEncounter(seed)
        b := simulateEncounter(seed)
        require.Equal(t, a, b)
    })
}

func TestProperty_TallyMonotonicity(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        rounds := simulateRandomEncounter(t)
        var prevS, prevF int
        for _, r := range rounds {
            require.GreaterOrEqual(t, r.Successes, prevS, "successes monotonically non-decreasing")
            require.GreaterOrEqual(t, r.Failures,  prevF, "failures monotonically non-decreasing")
            prevS, prevF = r.Successes, r.Failures
        }
    })
}
```

---

### Task 11: Exemplar content + docs

**Files:**
- Create: `content/social/encounters/extract_info_from_jenkins.yaml`
- Create: `content/social/encounters/negotiate_with_fixer_marlowe.yaml`
- Create: `content/social/encounters/gain_ally_warlord_grim.yaml`
- Modify: `docs/architecture/social.md` (or create)

- [ ] **Step 1: Checkpoint (SE-4).** Confirm with user which existing NPCs to attach the three exemplars to:
  - Wary informant scenario (extract information).
  - Fixer scenario (negotiate access).
  - Friendly faction NPC (gain ally).

- [ ] **Step 2: Author the three exemplars** with realistic biases, DCs, and outcome blocks exercising each new effect kind:

```yaml
# content/social/encounters/negotiate_with_fixer_marlowe.yaml
id: negotiate_with_fixer_marlowe
display_name: Negotiate Access with Marlowe
description: Marlowe runs the safehouse. Convince him you're worth the risk.
npc_id: marlowe
goal: Persuade Marlowe to let you use the safehouse.
influence_threshold: 4
failure_threshold: 3
round_limit: 6
discovery_dc: { kind: npc_level, expr: "10 + level" }
discovery_skills: [reasoning, street_smarts]
influence_skills:
  - { skill: smooth_talk, bias: receptive, dc: { kind: target_will } }
  - { skill: grift,       bias: neutral,   dc: { kind: target_will } }
  - { skill: muscle,      bias: resistant, dc: { kind: target_will } }
outcomes:
  success:
    - shift_disposition: 1
    - unlock_dialogue_topic: { topic_id: safehouse_access }
    - change_faction_rep: { faction_id: fixers, delta: 2 }
    - grant_credits: { amount: 0 }
  failure:
    - shift_disposition: -1
    - lock_quest: { quest_id: marlowe_safehouse_quest }
  walk_away:
    - shift_disposition: 0
```

- [ ] **Step 3: Architecture doc** — section explaining the encounter framework, the tally math, the bias model, the discovery cycle, the effect dispatch, and integration with `handleTalk`. Cross-link spec, plan, exemplars, #252 / #255.

---

## Verification

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
make migrate-up && make migrate-down
```

Additional sanity:

- `go vet ./...` clean.
- `make proto` re-runs cleanly with no diff.
- Telnet smoke test: enter Marlowe's room, run `talk marlowe`, verify the "Begin social encounter" option; run `social marlowe`, verify the menu, pick a skill; verify the tally updates; complete or walk away; verify the outcome effects (disposition shift, faction-rep delta) persist; verify `recall marlowe` first reveals the biases.
- Web smoke test: same scenario in the modal; biases annotated only after discovery; tally bars render; walk-away cancels.

---

## Rollout / Open Questions Resolved at Plan Time

- **SE-Q1**: Discovery is per-disposition cycle. Set/shift effects clear the discovery flag.
- **SE-Q2**: No in-fiction penalty on encounter failure in v1. Authors can add a `change_faction_rep` block in `outcomes.failure` if they want one.
- **SE-Q3**: Effective-threshold floor stays at 2.
- **SE-Q4**: Walk-away has no default reputation cost. Authors opt in via `outcomes.walk_away`.
- **SE-Q5**: Bribery integration deferred. Bribe and encounter remain separate paths in v1.

## Non-Goals Reaffirmed

Per spec §2.2:

- No multi-NPC group conversations.
- No real-time / timed choice.
- No NLP / free-text input.
- Existing `handleSeduce` / `handleBribe` continue unchanged.
- No persistent NPC memory beyond disposition / discovery / unlock state.
- No authoring GUI.

# Non-Combat Actions Against Combat NPCs — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make skill-vs-NPC combat actions actually useful: route every action through a single PF2E degrees-of-success resolver, express conditions and skill actions in YAML, expose a discovery RPC + UI surfacing on telnet and web, audit-and-migrate the nine existing handlers, and ship four new high-value skill actions (Create a Diversion, Bon Mot, Tumble Through, Recall Knowledge).

**Spec:** [docs/superpowers/specs/2026-04-24-noncombat-actions-vs-combat-npcs.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-24-noncombat-actions-vs-combat-npcs.md)

**Architecture:** Today, nine skill-vs-NPC handlers live as bespoke functions in `grpc_service.go`, each doing AP economy + binary outcome + ad-hoc condition. The plan extracts a single `internal/game/skillaction/` package with `Resolve(ctx, def)` returning `Outcome{DoS, Roll, DC, Bonus, Narrative}`, plus a `ValidateTarget` companion. Conditions move to `content/conditions/*.yaml`; skill actions move to `content/skill_actions/*.yaml`. Each existing handler becomes a thin wrapper that builds a `ResolveContext`, spends AP on success, and calls `Resolve`. A new `ListCombatActions` RPC enumerates available actions with `available` + `unavailable_reason` fields driving the telnet `skills` command and web "Skill Actions" submenu. A runtime compatibility step at combat start maps any legacy ad-hoc `Combatant.Conditions` entries (`-1 AC from demoralize`, `-2 AC from feint`) onto the canonical `frightened` / `off_guard` conditions so save data survives the migration.

**Tech Stack:** Go (`internal/game/skillaction/`, `internal/game/condition/`, `internal/gameserver/`), YAML content under `content/`, protobuf (`api/proto/game/v1/game.proto`), telnet handlers (`internal/frontend/handlers/`), web UI (`cmd/webclient/ui/src/`).

**Prerequisite:** None hard. #245 (typed bonuses) is a soft dep — `Condition.Effects` uses the typed-bonuses model when present; otherwise falls back to inline penalty fields. #260 (natural 1 / natural 20) is a soft dep — DoS computation includes the `nat 1 → DoS down one step` / `nat 20 → DoS up one step` rule per spec NCA-7. #267 (visibility / LoS) plugs into a no-op `PostValidateTarget` hook for line-of-fire when it lands.

**Audit checkpoint:** NCA-28 explicitly requires user confirmation of every migration choice in the audit table before the migration lands. Task 6 below contains that checkpoint and MUST NOT be bypassed.

---

## File Map

| Action | Path |
|--------|------|
| Create | `internal/game/skillaction/def.go` |
| Create | `internal/game/skillaction/def_test.go` |
| Create | `internal/game/skillaction/resolver.go` |
| Create | `internal/game/skillaction/resolver_test.go` |
| Create | `internal/game/skillaction/target.go` |
| Create | `internal/game/skillaction/target_test.go` |
| Create | `internal/game/skillaction/apply.go` |
| Create | `internal/game/skillaction/apply_test.go` |
| Modify | `internal/game/condition/loader.go` (load `content/conditions/*.yaml`) |
| Create | `content/conditions/frightened.yaml`, `off_guard.yaml`, `clumsy.yaml`, `stupefied.yaml`, `prone.yaml`, `grabbed.yaml`, `fleeing.yaml` |
| Create | `content/skill_actions/demoralize.yaml`, `feint.yaml`, `grapple.yaml`, `trip.yaml`, `disarm.yaml`, `shove.yaml`, `first_aid.yaml`, `create_a_diversion.yaml`, `bon_mot.yaml`, `tumble_through.yaml`, `recall_knowledge.yaml` |
| Modify | `internal/gameserver/grpc_service.go` (existing handlers → thin wrappers; legacy condition migration on combat start) |
| Create | `internal/gameserver/grpc_service_skillaction_test.go` |
| Create | `internal/gameserver/list_combat_actions.go` |
| Create | `internal/gameserver/list_combat_actions_test.go` |
| Modify | `api/proto/game/v1/game.proto` (`ListCombatActionsRequest` / `Response`, `CombatActionEntry`) |
| Create | `internal/frontend/handlers/skills_command.go` |
| Create | `internal/frontend/handlers/skills_command_test.go` |
| Create | `cmd/webclient/ui/src/combat/SkillActionsMenu.tsx` |
| Create | `cmd/webclient/ui/src/combat/SkillActionsMenu.test.tsx` |
| Modify | `docs/architecture/combat.md` (Skill actions section) |

---

### Task 1: `ActionDef` + YAML loader

**Files:**
- Create: `internal/game/skillaction/def.go`
- Create: `internal/game/skillaction/def_test.go`

- [ ] **Step 1: Failing tests** for the schema and validation:

```go
func TestLoadActionDef_AllFieldsParse(t *testing.T) {
    def, err := skillaction.Load([]byte(demoralizeYAML))
    require.NoError(t, err)
    require.Equal(t, "demoralize", def.ID)
    require.Equal(t, 1, def.APCost)
    require.Equal(t, skillaction.DCKindTargetWill, def.DC.Kind) // post-Q1 alignment
    require.Equal(t, skillaction.RangeRanged, def.Range.Kind)
    require.Contains(t, def.Outcomes[skillaction.CritSuccess].Effects[0].(*skillaction.ApplyCondition).ID, "frightened")
}

func TestLoadActionDef_RejectsUnknownCondition(t *testing.T) {
    _, err := skillaction.Load([]byte(`
id: bogus
ap_cost: 1
skill: smooth_talk
dc: { kind: target_perception }
range: { kind: melee_reach }
target_kinds: [npc]
outcomes:
  success:
    - apply_condition: { id: not_a_condition, duration_rounds: 1 }
`))
    require.Error(t, err)
    require.Contains(t, err.Error(), "not_a_condition")
}

func TestLoadActionDef_RejectsNegativeAPCost(t *testing.T) {
    _, err := skillaction.Load(yamlWithAPCost(-1))
    require.Error(t, err)
}
```

- [ ] **Step 2: Implement** the schema:

```go
type ActionDef struct {
    ID            string
    DisplayName   string
    Description   string
    APCost        int
    Skill         string
    DC            DC
    Range         Range
    TargetKinds   []TargetKind
    Outcomes      map[DegreeOfSuccess]*OutcomeDef
}

type DC struct {
    Kind   DCKind   // target_perception | target_will | target_ac | fixed | formula
    Value  int      // for fixed
    Expr   string   // for formula
}

type Range struct {
    Kind  RangeKind // melee_reach | ranged | self
    Feet  int       // for ranged
}

type OutcomeDef struct {
    Effects   []Effect // ApplyCondition | Damage | Move | Narrative
    Narrative string
    APRefund  bool     // NCA-33 + Q5 generalisation
}

type Effect interface { effect() }
type ApplyCondition struct {
    ID             string
    Stacks         int
    DurationRounds int
}
type Damage struct{ Expr string }
type Move struct{ Feet int }
type Narrative struct{ Text string }
```

- [ ] **Step 3: Loader validation** per NCA-3:
  - Every `apply_condition.id` resolves against the loaded condition catalog (Task 2 dependency — wire after Task 2 has loaded conditions).
  - Every `skill` resolves against the Mud skill catalog.
  - Every numeric field is non-negative (AP cost, durations, stacks, feet, fixed DC values).

- [ ] **Step 4:** Loader entry point reads `content/skill_actions/*.yaml` and returns `map[string]*ActionDef`.

---

### Task 2: Condition YAML catalog + loader

**Files:**
- Modify: `internal/game/condition/loader.go`
- Create: `content/conditions/frightened.yaml`, `off_guard.yaml`, `clumsy.yaml`, `stupefied.yaml`, `prone.yaml`, `grabbed.yaml`, `fleeing.yaml`

- [ ] **Step 1: Failing tests** for the seven required conditions per NCA-4:

```go
func TestConditionCatalog_LoadsRequiredCanonicalSet(t *testing.T) {
    cat := condition.LoadCatalog(t, "content/conditions")
    for _, id := range []string{"frightened", "off_guard", "clumsy", "stupefied", "prone", "grabbed", "fleeing"} {
        require.NotNil(t, cat.ByID(id), "missing canonical condition %s", id)
    }
}

func TestCondition_FrightenedHasStatusPenaltyToChecksAndDCs(t *testing.T) {
    c := condition.LoadCatalog(t, "content/conditions").ByID("frightened")
    eff := c.Effects[0]
    require.Equal(t, "status", eff.BonusType)
    require.Equal(t, "checks_and_dcs", eff.Target)
    require.Equal(t, -1, eff.PerStack)
}

func TestCondition_FrightenedTicksDownPerRound(t *testing.T) {
    c := condition.LoadCatalog(t, "content/conditions").ByID("frightened")
    require.True(t, c.TickDownPerRound)
}
```

- [ ] **Step 2: Author the seven YAML files** per spec NCA-4. Each file:

```yaml
# content/conditions/frightened.yaml
id: frightened
display_name: Frightened
description: |
  You are gripped by fear. Take a status penalty to checks and DCs equal to
  the condition value. The value decreases by 1 at the end of each round.
stacking: max_stacks
max_stacks: 4
tick_down_per_round: true
effects:
  - bonus_type: status
    target: checks_and_dcs
    per_stack: -1
```

- [ ] **Step 3: Loader** reads `content/conditions/*.yaml` at startup, validates the schema, and exposes a `Catalog.ByID(id)` lookup. Keep the existing in-code condition factories until the migration in Task 6 retires them.

- [ ] **Step 4:** Wire Task 1's loader to call `Catalog.ByID` for every `apply_condition.id` referenced in skill-action YAML.

---

### Task 3: `Resolve` — DoS computation + effect application

**Files:**
- Create: `internal/game/skillaction/resolver.go`
- Create: `internal/game/skillaction/apply.go`
- Create: `internal/game/skillaction/resolver_test.go`
- Create: `internal/game/skillaction/apply_test.go`

- [ ] **Step 1: Failing tests** for DoS computation (NCA-7) including natural-1/20 rules:

```go
func TestDoS_PlusTenIsCritSuccess(t *testing.T) {
    require.Equal(t, skillaction.CritSuccess, skillaction.DoS(roll(15), bonus(5), dc(10)))
}

func TestDoS_MinusTenIsCritFailure(t *testing.T) {
    require.Equal(t, skillaction.CritFailure, skillaction.DoS(roll(2), bonus(0), dc(15)))
}

func TestDoS_Nat20BumpsUpOneStep_FromFailure(t *testing.T) {
    // roll 20 + bonus 0 vs DC 30: raw is failure, nat 20 bumps to success.
    require.Equal(t, skillaction.Success, skillaction.DoS(roll(20), bonus(0), dc(30)))
}

func TestDoS_Nat20OnSuccessBumpsToCrit(t *testing.T) {
    // roll 20 + bonus 0 vs DC 15: raw success; nat 20 bumps to crit.
    require.Equal(t, skillaction.CritSuccess, skillaction.DoS(roll(20), bonus(0), dc(15)))
}

func TestDoS_Nat1OnCritFailureStaysCritFailure(t *testing.T) {
    require.Equal(t, skillaction.CritFailure, skillaction.DoS(roll(1), bonus(0), dc(15)))
}

func TestDoS_Nat1OnSuccessBumpsToFailure(t *testing.T) {
    // roll 1 + bonus 30 vs DC 10: raw crit success; nat 1 bumps down to success.
    require.Equal(t, skillaction.Success, skillaction.DoS(roll(1), bonus(30), dc(10)))
}
```

- [ ] **Step 2: Implement** `DoS`:

```go
func DoS(roll, bonus, dc int) DegreeOfSuccess {
    total := roll + bonus
    band := computeBand(total, dc) // -1 = critFail, 0 = fail, 1 = success, 2 = critSuccess
    if roll == 20 {
        if band < 2 { band++ }
    } else if roll == 1 {
        if band > -1 { band-- }
    }
    return bandToDoS(band)
}
```

- [ ] **Step 3: Failing tests** for `Resolve`:

```go
func TestResolve_AppliesOutcomeEffectsInOrder(t *testing.T) {
    def := loadDef(t, "demoralize")
    ctx := buildResolveContext(t)
    out, err := skillaction.Resolve(ctx, def, withRoll(18))
    require.NoError(t, err)
    require.Equal(t, skillaction.CritSuccess, out.DoS) // assuming bonus + 18 vs DC fits crit band
    appliedFrightened := ctx.AppliedConditions["frightened"]
    require.Equal(t, 2, appliedFrightened.Stacks)
}

func TestResolve_ReturnsGenericNarrativeWhenYAMLEmpty(t *testing.T) {
    def := defWithoutNarratives(t)
    out, err := skillaction.Resolve(buildResolveContext(t), def, withRoll(15))
    require.NoError(t, err)
    require.Contains(t, out.Narrative, "succeeds") // fallback template
}

func TestResolve_DoesNotConsumeAP(t *testing.T) {
    initial := actorAP(t)
    skillaction.Resolve(buildResolveContext(t), loadDef(t, "feint"), withRoll(15))
    require.Equal(t, initial, actorAP(t), "Resolve must not deduct AP (NCA-10)")
}

func TestResolve_NoMutationOfCombat(t *testing.T) {
    cbt := buildResolveContext(t).Combat
    snap := combatSnapshot(cbt)
    skillaction.Resolve(buildResolveContext(t), loadDef(t, "trip"), withRoll(20))
    require.Equal(t, snap, combatSnapshot(cbt), "Resolve must not mutate Combat directly (NCA-11)")
}
```

- [ ] **Step 4: Implement** `Resolve`:

```go
func Resolve(ctx ResolveContext, def *ActionDef, dice DiceRoller) (Outcome, error) {
    roll := dice.Roll("1d20")
    bonus := ctx.Actor.SkillBonus(def.Skill)
    dc, err := evaluateDC(ctx, def.DC)
    if err != nil { return Outcome{}, err }
    dos := DoS(roll, bonus, dc)
    out := Outcome{DoS: dos, Roll: roll, Bonus: bonus, DC: dc}
    od := def.Outcomes[dos]
    for _, eff := range od.Effects {
        ctx.Apply(ctx, eff)
    }
    out.Narrative = renderNarrative(def, dos, od)
    return out, nil
}
```

`ctx.Apply` is the side-effect callback that dispatches `ApplyCondition`, `Damage`, `Move`, `Narrative`. Keeps `Resolve` itself pure (NCA-11).

- [ ] **Step 5:** Apply tests for each effect kind in `apply_test.go`. `ApplyCondition` consults the condition catalog from Task 2; `Damage` parses the dice expression and routes through `combat.ResolveDamage` (when #246 is present) or a direct `ApplyDamage` call.

---

### Task 4: Target validation + line-of-fire seam

**Files:**
- Create: `internal/game/skillaction/target.go`
- Create: `internal/game/skillaction/target_test.go`

- [ ] **Step 1: Failing tests** per NCA-12 / NCA-16:

```go
func TestValidateTarget_MeleeReachRequiresAdjacent(t *testing.T) {
    err := skillaction.ValidateTarget(ctxWithChebyshev(2), defWithRange(skillaction.RangeMelee))
    require.Error(t, err)
    require.Contains(t, err.Error(), "out of melee reach")
}

func TestValidateTarget_RangedHonorsFeet(t *testing.T) {
    err := skillaction.ValidateTarget(ctxWithChebyshev(7) /* 35 ft */, defWithRangedFeet(30))
    require.Error(t, err)
}

func TestValidateTarget_TargetKindFilter(t *testing.T) {
    err := skillaction.ValidateTarget(ctxWithTargetKind("player"), defOnlyNPC())
    require.Error(t, err)
    require.Contains(t, err.Error(), "target kind")
}

func TestPostValidateTarget_NoOpV1(t *testing.T) {
    err := skillaction.ValidateTarget(buildLOFCtx(t), defAny())
    require.NoError(t, err, "LoF stage must be a no-op until #267 ships (NCA-15)")
}
```

- [ ] **Step 2: Implement** `ValidateTarget`:

```go
func ValidateTarget(ctx ResolveContext, def *ActionDef) error {
    if !inTargetKinds(ctx.Target.Kind, def.TargetKinds) {
        return &PreconditionError{Field: "target_kind", Detail: ...}
    }
    switch def.Range.Kind {
    case RangeMelee:
        if chebyshevCells(ctx.Actor, ctx.Target) > 1 {
            return &PreconditionError{Field: "range", Detail: "out of melee reach"}
        }
    case RangeRanged:
        if chebyshevCells(ctx.Actor, ctx.Target) > def.Range.Feet/5 {
            return &PreconditionError{Field: "range", Detail: fmt.Sprintf("out of range (max %d ft)", def.Range.Feet)}
        }
    case RangeSelf:
        if ctx.Actor != ctx.Target {
            return &PreconditionError{Field: "range", Detail: "must target self"}
        }
    }
    return PostValidateTarget(ctx, def) // NCA-15: no-op v1 hook
}

func PostValidateTarget(ctx ResolveContext, def *ActionDef) error {
    return nil
}
```

- [ ] **Step 3:** `PreconditionError` carries a structured `Field`/`Detail` so the handler can surface it to the client without consuming AP (NCA-17).

---

### Task 5: `ListCombatActions` discovery RPC

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Create: `internal/gameserver/list_combat_actions.go`
- Create: `internal/gameserver/list_combat_actions_test.go`

- [ ] **Step 1: Add proto messages** per NCA-18 / NCA-19:

```proto
message ListCombatActionsRequest {
  string combat_id = 1;
  string actor_uid = 2;   // optional — server infers from session
  string target_uid = 3;  // optional — when supplied, availability is computed against this target
}

message ListCombatActionsResponse {
  repeated CombatActionEntry entries = 1;
}

message CombatActionEntry {
  string id                 = 1;
  string display_name       = 2;
  string description        = 3;
  int32  ap_cost            = 4;
  string skill              = 5;
  string dc_summary         = 6;  // "vs Target Perception DC", never the value
  string range_summary      = 7;  // "melee reach" | "ranged 30 ft" | "self"
  bool   available          = 8;
  string unavailable_reason = 9;  // "out of range" | "insufficient AP" | "target is not an NPC" | ...
}
```

- [ ] **Step 2: Failing tests**:

```go
func TestListCombatActions_FiltersByTargetKindWhenTargetSupplied(t *testing.T) {
    res := s.ListCombatActions(ctx, &gamev1.ListCombatActionsRequest{
        CombatId: combatID, TargetUid: playerTargetUID,
    })
    for _, e := range res.Entries {
        // Demoralize is npc-only; with a player target it must surface as unavailable.
        if e.Id == "demoralize" {
            require.False(t, e.Available)
            require.Contains(t, e.UnavailableReason, "not an NPC")
        }
    }
}

func TestListCombatActions_EnumeratesAllWhenTargetEmpty(t *testing.T) {
    res := s.ListCombatActions(ctx, &gamev1.ListCombatActionsRequest{CombatId: combatID})
    require.Len(t, res.Entries, len(skillActionCatalog())) // NCA-20
}

func TestListCombatActions_DCSummaryHidesExactValue(t *testing.T) {
    res := s.ListCombatActions(ctx, &gamev1.ListCombatActionsRequest{CombatId: combatID, TargetUid: enemyUID})
    for _, e := range res.Entries {
        require.NotRegexp(t, `\d+`, e.DcSummary, "exact DC numbers must not leak (NCA-22)")
    }
}
```

- [ ] **Step 3: Implement** the handler. For each action in the catalog: build the entry, run `ValidateTarget` against the supplied target (if any) and the actor's AP, set `available` and `unavailable_reason` accordingly. Cache the action catalog at startup; do not reload per request.

---

### Task 6: Audit checkpoint + migration of existing handlers

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (existing nine handlers)
- Create: `content/skill_actions/demoralize.yaml`, `feint.yaml`, `grapple.yaml`, `trip.yaml`, `disarm.yaml`, `shove.yaml`, `first_aid.yaml`
- Modify: `internal/gameserver/grpc_service_test.go` (Feint suite + equivalents)

- [ ] **Step 1: Checkpoint (NCA-28).** STOP and confirm with the user the migration table from spec §4.6:
  - Demoralize → Frightened 1/2 ladder.
  - Feint → Off-Guard window vs single attack; full-turn on crit.
  - Grapple → grabbed/restrained/off_guard ladder.
  - Trip → prone + d6 fall on crit success; actor prone on crit failure.
  - Disarm → -2 to attack on success / weapon drop on crit success / actor off_guard on crit failure.
  - Shove → 5/10 ft + prone ladder.
  - FirstAid → DoS-graded healing.
  - RaiseShield, TakeCover → not migrated (not skill checks).

  Confirmation MUST cover the per-action behaviour AND the runtime compatibility step (NCA-29) for in-flight save data.

- [ ] **Step 2: Author** YAML for each migrated action. Example (`demoralize.yaml`):

```yaml
id: demoralize
display_name: Demoralize
description: Cow your foe with words. They become Frightened.
ap_cost: 1
skill: smooth_talk
dc: { kind: target_will }
range: { kind: ranged, feet: 30 }
target_kinds: [npc]
outcomes:
  crit_success:
    - apply_condition: { id: frightened, stacks: 2, duration_rounds: -1 }
    - narrative: { text: "{actor}'s words shatter {target}'s composure. ({target} is Frightened 2.)" }
  success:
    - apply_condition: { id: frightened, stacks: 1, duration_rounds: -1 }
    - narrative: { text: "{actor} unnerves {target}. ({target} is Frightened 1.)" }
  failure:
    - narrative: { text: "{actor}'s threats fall flat." }
  crit_failure:
    - narrative: { text: "{actor}'s posturing emboldens {target}." }
```

- [ ] **Step 3: Convert each handler** to a thin wrapper:

```go
func (s *Service) HandleDemoralize(ctx context.Context, req *gamev1.DemoralizeRequest) (*gamev1.DemoralizeResponse, error) {
    actor, target, def, err := s.bindSkillAction(ctx, req.CombatId, req.ActorUid, req.TargetUid, "demoralize")
    if err != nil { return nil, err }
    if err := skillaction.ValidateTarget(rctx(actor, target, def), def); err != nil {
        return s.skillActionPreconditionResponse(err)
    }
    if err := s.spendAP(actor, def.APCost); err != nil { return nil, err }
    out, err := skillaction.Resolve(rctx(actor, target, def), def, s.dice)
    if err != nil {
        s.refundAP(actor, def.APCost)
        return nil, err
    }
    s.combatLog.Append(out)
    return &gamev1.DemoralizeResponse{ /* fields built from out */ }, nil
}
```

`bindSkillAction` is the shared helper.

- [ ] **Step 4: Runtime compatibility** (NCA-29). On combat start (or on-load for existing save data), scan `Combatant.Conditions` for legacy entries (`-1 AC from demoralize`, `-2 AC from feint`) and replace with `frightened:1` / `off_guard:1-round`. Test:

```go
func TestLoad_LegacyDemoralizeCondition_MigratesToFrightened(t *testing.T) {
    cbt := combatWithLegacyDemoralizeOn("thug")
    s.OnCombatLoad(cbt)
    target := cbt.ByName("thug")
    require.NotContains(t, target.Conditions, legacyDemoralizeID)
    require.True(t, condition.HasActive(target, "frightened", 1))
}
```

- [ ] **Step 5: Update existing tests** that pin pre-migration behaviour. Each updated assertion gets a `// PF2E alignment per NCA-31` comment naming the spec.

---

### Task 7: Four new skill actions

**Files:**
- Create: `content/skill_actions/create_a_diversion.yaml`, `bon_mot.yaml`, `tumble_through.yaml`, `recall_knowledge.yaml`
- Modify: `internal/game/skillaction/def.go` (`OutcomeDef.APRefund` field per NCA-33)
- Create: `internal/gameserver/grpc_service_skillaction_test.go` end-to-end tests for each

- [ ] **Step 1: Author** the four YAML files per NCA-32:

```yaml
# content/skill_actions/recall_knowledge.yaml
id: recall_knowledge
display_name: Recall Knowledge
description: Pull what you know about the foe — their weaknesses, immunities, or notable tricks.
ap_cost: 1
skill: reasoning
dc: { kind: formula, expr: "14 + target.level" }
range: { kind: self }
target_kinds: [npc]
outcomes:
  crit_success:
    - reveal: { count: 2 }   # NEW Effect kind — see Step 2
    - narrative: { text: "{actor} pieces together two facts about {target}." }
  success:
    - reveal: { count: 1 }
    - narrative: { text: "{actor} recalls a useful detail about {target}." }
  failure:
    - narrative: { text: "{actor}'s memory comes up blank." }
  crit_failure:
    ap_refund: true     # NCA-33
    - narrative: { text: "{actor} confidently shares a detail — and gets it dead wrong." }
```

- [ ] **Step 2: Add the `Reveal` effect** to `apply.go`:

```go
type Reveal struct{ Count int }
```

`Reveal` writes one or two NPC-metadata facts to the per-session known-knowledge store. Per spec NCA-Q2: ONE of `[weaknesses, notable ability, immunity]` on success; up to TWO on crit success. Never `HP` or `AC`.

- [ ] **Step 3: Generalised `ap_refund` field** per NCA-Q5. The wrapper `bindSkillAction` in Task 6 calls `spendAP` before `Resolve`, then refunds if `out.Effects[dos].APRefund` is true. Tests:

```go
func TestResolve_RecallKnowledge_CritFailureRefundsAP(t *testing.T) {
    apBefore := actorAP(t)
    res := callRecallKnowledge(t, withRoll(1), bonusFor(t, target, dc-15))
    require.Equal(t, skillaction.CritFailure, res.DoS)
    require.Equal(t, apBefore, actorAP(t), "AP must be refunded on RecallKnowledge crit failure")
}
```

- [ ] **Step 4: End-to-end tests** in `grpc_service_skillaction_test.go` for each new action. Cover crit success, success, failure, crit failure paths.

---

### Task 8: Telnet `skills` command

**Files:**
- Create: `internal/frontend/handlers/skills_command.go`
- Create: `internal/frontend/handlers/skills_command_test.go`

- [ ] **Step 1: Failing tests** per NCA-23:

```go
func TestSkillsCommand_ListsAvailableActionsWithReasons(t *testing.T) {
    h := newHandlerInCombat(t, withTarget(npcUID))
    out := h.Run("skills")
    require.Contains(t, out, "demoralize  Demoralize (AP 1, ranged 30 ft, vs Target Will DC) — available")
    require.Contains(t, out, "feint  Feint (AP 1, melee reach, vs Target Perception DC) — unavail: out of melee reach")
}

func TestSkillsCommand_AliasS(t *testing.T) {
    require.Equal(t, runHandler("skills"), runHandler("s"))
}

func TestSkillsCommand_ExistingFeintCommandStillWorks(t *testing.T) {
    out := h.Run("feint thug")
    require.NotContains(t, out, "unknown command")
}
```

- [ ] **Step 2: Implement** the command. It calls `ListCombatActions` with the player's current combat + target, formats one line per entry. Width-aware rendering uses the existing telnet console-region helpers.

- [ ] **Step 3:** Verify the existing per-action commands (`feint`, `demoralize`, `grapple`, ...) continue to work unchanged (NCA-24).

---

### Task 9: Web Skill Actions submenu

**Files:**
- Create: `cmd/webclient/ui/src/combat/SkillActionsMenu.tsx`
- Create: `cmd/webclient/ui/src/combat/SkillActionsMenu.test.tsx`
- Modify: `cmd/webclient/ui/src/combat/CombatActionBar.tsx`

- [ ] **Step 1: Failing component tests** per NCA-25 / NCA-26 / NCA-27:

```ts
test("renders entries from ListCombatActions", async () => {
  mockListCombatActions([
    entry("demoralize", { available: true }),
    entry("feint", { available: false, unavailable_reason: "out of melee reach" }),
  ]);
  render(<SkillActionsMenu />);
  expect(await screen.findByText("Demoralize")).toBeInTheDocument();
  const feintBtn = screen.getByText("Feint").closest("button")!;
  expect(feintBtn).toBeDisabled();
  expect(feintBtn).toHaveAttribute("title", expect.stringContaining("out of melee reach"));
});

test("refreshes after round tick", async () => {
  const { rerender } = render(<SkillActionsMenu />);
  fireRoundTick();
  await waitFor(() => expect(mockListCombatActions).toHaveBeenCalledTimes(2));
});

test("clicking an available entry opens the existing target picker", async () => {
  mockListCombatActions([entry("demoralize", { available: true })]);
  const { onPickerOpened } = renderWithSpy();
  fireEvent.click(screen.getByText("Demoralize"));
  expect(onPickerOpened).toHaveBeenCalledWith({ actionId: "demoralize" });
});
```

- [ ] **Step 2: Implement** `SkillActionsMenu`. Reuses the existing AP-cost display + click-to-execute affordance from `CombatActionBar`. Greys out unavailable entries with the reason in a tooltip. Refreshes on each round-tick event from the existing combat slice.

- [ ] **Step 3: Wire** the submenu into `CombatActionBar` as a new "Skill Actions" entry; existing combat buttons (Attack, Stride, Reload, Use Tech) are unchanged (per NCA-Q3 — `ListCombatActions` stays skill-specific).

---

### Task 10: Telemetry + narrative

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (`bindSkillAction` post-resolve hook)
- Modify: `internal/gameserver/grpc_service_skillaction_test.go`

- [ ] **Step 1: Failing tests** for NCA-34 / NCA-35:

```go
func TestSkillAction_EmitsStructuredLog(t *testing.T) {
    sink := captureLog(t)
    h.Run("demoralize thug")
    e := sink.Last()
    require.Equal(t, "demoralize", e["action_id"])
    require.Equal(t, actorUID, e["actor_uid"])
    require.Equal(t, npcUID, e["target_uid"])
    require.Contains(t, e, "roll")
    require.Contains(t, e, "bonus")
    require.Contains(t, e, "dc")
    require.Contains(t, e, "dos")
}

func TestSkillAction_NarrativeIncludesDoSName(t *testing.T) {
    out := runDemoralizeAtRoll(20)
    require.Contains(t, out.Narrative, "succeeds critically")
}
```

- [ ] **Step 2: Implement** the structured log entry and the narrative DoS prefix injection. Narrative text comes from the YAML; DoS prefix is appended only when the YAML narrative does not already mention the outcome.

---

### Task 11: Architecture documentation

**Files:**
- Modify: `docs/architecture/combat.md`

- [ ] **Step 1: Add a "Skill Actions" section** documenting:
  - All 35 `NCA-N` requirements with one-line summaries.
  - The pipeline: `bindSkillAction → ValidateTarget → spendAP → Resolve → ApplyEffects → emit narrative + log → refund-on-AP-refund`.
  - The DoS computation rules (±10 band + natural-1/20).
  - The condition catalog and stacking rules (`independent`, `replace_if_higher`, `max_stacks`).
  - The legacy compatibility mapping for in-flight save data (NCA-29).
  - The per-outcome `ap_refund` field (NCA-33 generalisation).
  - The line-of-fire seam reserved for #267.
  - Open question resolutions (NCA-Q1..Q5).

- [ ] **Step 2: Cross-link** to `internal/game/skillaction/`, the spec, and to `content/conditions/` and `content/skill_actions/` directories.

- [ ] **Step 3:** Verify the doc renders correctly in GitHub markdown preview.

---

## Verification

Per SWENG-6, the full test suite MUST pass before commit / PR:

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
```

Additional sanity:

- `go vet ./...` clean.
- `make proto` re-runs cleanly with no diff.
- Telnet smoke test: enter combat with one NPC; run `skills`; pick `demoralize thug`, verify narrative includes "Frightened" on success; load a save file containing the legacy demoralize condition and verify the mapping replaces it on combat start.
- Web smoke test: skill-actions submenu populated; greyed-out entries show their reason on hover; clicking an available entry opens the existing target picker.

---

## Rollout / Open Questions Resolved at Plan Time

- **NCA-Q1**: Full PF2E migration to Frightened + NCA-29 compatibility mapping. Cleaner, fewer long-term code paths. Confirmed by user at the Task 6 checkpoint.
- **NCA-Q2**: Recall Knowledge surfaces ONE of `[weaknesses, notable ability, immunity]` on success; up to TWO on crit success. Never HP or AC.
- **NCA-Q3**: `ListCombatActions` stays skill-specific. Smaller blast radius; client merges with its existing action list.
- **NCA-Q4**: No pagination v1. 13 actions fit comfortably on one telnet screen.
- **NCA-Q5**: `ap_refund` is a generalised per-outcome field, not a Recall-Knowledge-only special case. Cost is low; future actions may want it.

## Non-Goals Reaffirmed

Per spec §2.2:

- No ability-score → skill remapping (Mud's existing mapping stays).
- No "use any skill on any target" sandbox.
- No NPC skill actions against players (separate ticket).
- No hotbar / macro bindings.
- No persistent skill cooldowns; AP economy gates use.
- No combat-map animations / VFX.

# Innate Technologies — Design Spec

**Date:** 2026-03-18

---

## Goal

Add per-tech daily use tracking for innate technologies, wire region innate grants into character creation, implement activation and rest restoration, expose innate slots on the character sheet, and populate all 11 regions with innate tech content derived from PF2E ancestry innate spell equivalents.

---

## Context

`InnateSlot{MaxUses int}` and `InnateTechRepo` (GetAll/Set/DeleteAll) already exist. `AssignTechnologies` assigns innate grants from archetypes; `LoadTechnologies` loads them at login. The `character_innate_technologies` table stores `tech_id` and `max_uses` but not `uses_remaining`. No archetypes currently define innate grants. Regions have modifiers, traits, and ability boosts but no `InnateGrants` field. `handleUse`, `handleRest`, and `handleChar` have no innate tech paths.

**Out of scope:** Effect resolution for innate techs; `chrome_reflex` reaction trigger; passive mechanics for `seismic_sense`/`moisture_reclaim`; level-up innate grants. These are tracked in `docs/requirements/FEATURES.md`.

---

## Feature 1: Region Innate Grants

### 1a. `Region` struct update

File: `internal/game/ruleset/region.go`

Add field to `Region`:

```go
// InnateTechnologies lists innate technology grants from this region.
InnateTechnologies []InnateGrant `yaml:"innate_technologies,omitempty"`
```

`InnateGrant` is already defined in `internal/game/ruleset/technology_grants.go`.

### 1b. Region YAML content

Each region YAML gains an `innate_technologies:` block with one entry. `uses_per_day: 0` means unlimited.

| Region file | tech id | uses_per_day |
|-------------|---------|-------------|
| `old_town.yaml` | `blackout_pulse` | 0 (unlimited) |
| `northeast.yaml` | `arc_lights` | 0 (unlimited) |
| `pearl_district.yaml` | `pressure_burst` | 1 |
| `southeast_portland.yaml` | `nanite_infusion` | 1 |
| `pacific_northwest.yaml` | `atmospheric_surge` | 1 |
| `south.yaml` | `viscous_spray` | 1 |
| `southern_california.yaml` | `chrome_reflex` | 1 |
| `mountain.yaml` | `seismic_sense` | 0 (unlimited) |
| `midwest.yaml` | `moisture_reclaim` | 0 (unlimited) |
| `north_portland.yaml` | `terror_broadcast` | 1 |
| `gresham_outskirts.yaml` | `acid_spit` | 1 |

### 1c. Innate technology YAML files

New directory: `content/technologies/innate/`

Each file follows the same YAML schema as `mind_spike.yaml`. All are `usage_type: innate`. Effect resolution is out of scope — `effects:` block is omitted or left empty.

**`blackout_pulse.yaml`**
```yaml
id: blackout_pulse
name: Blackout Pulse
description: Emits a localized EM burst that kills electronic lighting and blinds optical sensors in a small radius.
tradition: technical
level: 1
usage_type: innate
action_cost: 1
range: emanation
targets: area
duration: rounds:1
```

**`arc_lights.yaml`**
```yaml
id: arc_lights
name: Arc Lights
description: Projects three hovering electromagnetic arc-light drones that illuminate and disorient.
tradition: technical
level: 1
usage_type: innate
action_cost: 1
range: close
targets: area
duration: minutes:1
```

**`pressure_burst.yaml`**
```yaml
id: pressure_burst
name: Pressure Burst
description: A pneumatic compression rig vents a focused blast that shoves targets and shatters brittle obstacles.
tradition: technical
level: 1
usage_type: innate
action_cost: 2
range: ranged
targets: single
duration: instant
```

**`nanite_infusion.yaml`**
```yaml
id: nanite_infusion
name: Nanite Infusion
description: Releases a cloud of salvaged medical nanites that accelerate tissue repair in a touched target.
tradition: bio_synthetic
level: 1
usage_type: innate
action_cost: 2
range: touch
targets: single
duration: instant
```

**`atmospheric_surge.yaml`**
```yaml
id: atmospheric_surge
name: Atmospheric Surge
description: A wrist-mounted atmospheric compressor discharges a powerful wind blast that scatters enemies.
tradition: technical
level: 1
usage_type: innate
action_cost: 2
range: ranged
targets: area
duration: instant
```

**`viscous_spray.yaml`**
```yaml
id: viscous_spray
name: Viscous Spray
description: A bio-synthetic adhesive secretion coats a target's joints and limbs, restraining movement.
tradition: bio_synthetic
level: 1
usage_type: innate
action_cost: 2
range: ranged
targets: single
duration: rounds:1
```

**`chrome_reflex.yaml`**
```yaml
id: chrome_reflex
name: Chrome Reflex
description: A neural-augmented reflex burst that overrides the nervous system and forces a second attempt at a failed saving throw.
tradition: neural
level: 1
usage_type: innate
action_cost: 1
range: self
targets: self
duration: instant
```

**`seismic_sense.yaml`**
```yaml
id: seismic_sense
name: Seismic Sense
description: Bone-conduction implants detect ground vibrations, revealing the movement of creatures through floors and walls.
tradition: technical
level: 1
usage_type: innate
action_cost: 1
range: emanation
targets: area
duration: rounds:1
```

**`moisture_reclaim.yaml`**
```yaml
id: moisture_reclaim
name: Moisture Reclaim
description: Atmospheric condensation filters extract potable water from ambient humidity.
tradition: bio_synthetic
level: 1
usage_type: innate
action_cost: 1
range: self
targets: self
duration: instant
```

**`terror_broadcast.yaml`**
```yaml
id: terror_broadcast
name: Terror Broadcast
description: A subdermal transmitter floods nearby targets with a fear-inducing neural frequency.
tradition: neural
level: 1
usage_type: innate
action_cost: 2
range: emanation
targets: area
duration: rounds:1
```

**`acid_spit.yaml`**
```yaml
id: acid_spit
name: Acid Spit
description: A bio-synthetic gland secretes pressurized corrosive fluid at a single target.
tradition: bio_synthetic
level: 1
usage_type: innate
action_cost: 2
range: ranged
targets: single
duration: instant
```

---

## Feature 2: Session and DB — UsesRemaining

### 2a. `InnateSlot` update

File: `internal/game/session/technology.go`

```go
// InnateSlot tracks an innate technology granted by a region or archetype.
// MaxUses == 0 means unlimited.
type InnateSlot struct {
    MaxUses       int
    UsesRemaining int
}
```

### 2b. DB migration 029

File: `migrations/029_innate_uses_remaining.up.sql`

```sql
ALTER TABLE character_innate_technologies
    ADD COLUMN uses_remaining INT NOT NULL DEFAULT 0;
```

File: `migrations/029_innate_uses_remaining.down.sql`

```sql
ALTER TABLE character_innate_technologies
    DROP COLUMN uses_remaining;
```

---

## Feature 3: `InnateTechRepo` extension

File: `internal/gameserver/technology_assignment.go`

The full updated `InnateTechRepo` interface (existing + new methods):

```go
type InnateTechRepo interface {
    // GetAll returns all innate slots for the character.
    GetAll(ctx context.Context, characterID int64) (map[string]*session.InnateSlot, error)

    // Set initializes or overwrites an innate slot entry.
    // Postcondition: row (characterID, techID) has max_uses=maxUses, uses_remaining=maxUses.
    // Precondition: only called at character creation or full re-assignment, never at login load.
    Set(ctx context.Context, characterID int64, techID string, maxUses int) error

    // DeleteAll removes all innate tech rows for the character.
    DeleteAll(ctx context.Context, characterID int64) error

    // Decrement atomically decrements uses_remaining by 1 if > 0.
    // Precondition: caller has verified UsesRemaining > 0 in session before calling.
    // Postcondition: uses_remaining = max(0, uses_remaining - 1).
    Decrement(ctx context.Context, characterID int64, techID string) error

    // RestoreAll sets uses_remaining = max_uses for all rows of this character.
    // Postcondition: all innate slots are at maximum uses.
    RestoreAll(ctx context.Context, characterID int64) error
}
```

File: `internal/storage/postgres/character_innate_tech.go`

```go
func (r *CharacterInnateTechRepository) Decrement(ctx context.Context, characterID int64, techID string) error {
    _, err := r.db.Exec(ctx,
        `UPDATE character_innate_technologies
            SET uses_remaining = GREATEST(0, uses_remaining - 1)
          WHERE character_id = $1 AND tech_id = $2`,
        characterID, techID)
    return err
}

func (r *CharacterInnateTechRepository) RestoreAll(ctx context.Context, characterID int64) error {
    _, err := r.db.Exec(ctx,
        `UPDATE character_innate_technologies
            SET uses_remaining = max_uses
          WHERE character_id = $1`,
        characterID)
    return err
}
```

`GetAll` must also scan `uses_remaining`:

```go
rows, err := r.db.Query(ctx,
    `SELECT tech_id, max_uses, uses_remaining
       FROM character_innate_technologies
      WHERE character_id = $1`,
    characterID)
// ...
var techID string
var maxUses, usesRemaining int
if err := rows.Scan(&techID, &maxUses, &usesRemaining); err != nil { ... }
slots[techID] = &session.InnateSlot{MaxUses: maxUses, UsesRemaining: usesRemaining}
```

`Set` must also write `uses_remaining`:

```go
_, err := r.db.Exec(ctx,
    `INSERT INTO character_innate_technologies (character_id, tech_id, max_uses, uses_remaining)
     VALUES ($1, $2, $3, $3)
     ON CONFLICT (character_id, tech_id)
     DO UPDATE SET max_uses = EXCLUDED.max_uses, uses_remaining = EXCLUDED.uses_remaining`,
    characterID, techID, maxUses)
```

Note: `uses_remaining` is initialized to `max_uses` on Set (new character / re-assignment). The second `$3` covers both columns. **Precondition:** `Set` is only called at character creation or full re-assignment — never at login load. Calling `Set` on an existing row will reset `uses_remaining` to `max_uses`, which is correct behavior at creation but would be wrong at login (use `GetAll` at login, not `Set`).

---

## Feature 4: Character Creation — Region Innate Grants

File: `internal/gameserver/technology_assignment.go` — `AssignTechnologies`

After the existing archetype innate grant block, add a region innate grant block:

```go
if region != nil {
    for _, grant := range region.InnateTechnologies {
        sess.InnateTechs[grant.ID] = &session.InnateSlot{
            MaxUses:       grant.UsesPerDay,
            UsesRemaining: grant.UsesPerDay,
        }
        if err := innateRepo.Set(ctx, characterID, grant.ID, grant.UsesPerDay); err != nil {
            return fmt.Errorf("AssignTechnologies: set region innate tech %s: %w", grant.ID, err)
        }
    }
}
```

`AssignTechnologies` gains one parameter: `region *ruleset.Region`. All call sites updated.

The call site in `grpc_service.go` uses `s.regions[dbChar.Region]` (a `map[string]*ruleset.Region` field on `GameServiceServer`) to obtain `*ruleset.Region` and pass it in.

`sess.InnateTechs` must be initialized once before both the archetype and region innate blocks. Place the nil guard immediately before the existing archetype innate block **and remove the existing `sess.InnateTechs = make(...)` line from inside the archetype conditional** — it is replaced by this single guard:

```go
// Initialize once before both archetype and region innate blocks
if sess.InnateTechs == nil {
    sess.InnateTechs = make(map[string]*session.InnateSlot)
}
// existing archetype innate loop follows here (with the inner make() removed)
// ...
// then the new region innate loop
```

---

## Feature 5: `handleUse` — Innate Tech Path

File: `internal/gameserver/grpc_service.go`

### No-arg (list) mode

After the spontaneous tech listing block, append innate tech entries. For each tech ID in `sess.InnateTechs` (sorted ascending for determinism):

- If `MaxUses == 0`: `<techID> (unlimited)`
- Else if `UsesRemaining > 0`: `<techID> (<N> uses remaining)`
- If `UsesRemaining == 0`: omit (no uses left — not actionable)

### Activation path

After the spontaneous tech activation block, when `abilityID` matched no prepared or spontaneous tech:

1. Look up `techID` in `sess.InnateTechs`
2. If not found: return `"You don't have innate tech <techID>."`
3. If `MaxUses != 0 && slot.UsesRemaining <= 0`: return `"No uses of <techID> remaining."`
4. Otherwise:
   - If `MaxUses != 0`: call `s.innateTechRepo.Decrement(ctx, sess.CharacterID, techID)`; decrement session: `slot.UsesRemaining--; sess.InnateTechs[techID] = slot`
   - Return `"You activate <techID>."` (unlimited) or `"You activate <techID>. (<N> uses remaining.)"` (limited)

---

## Feature 6: `handleRest` — Innate Restoration

File: `internal/gameserver/grpc_service.go`

After the spontaneous use pool restoration block:

```go
if err := s.innateTechRepo.RestoreAll(ctx, sess.CharacterID); err != nil {
    return fmt.Errorf("handleRest: restore innate use pools: %w", err)
}
innates, err := s.innateTechRepo.GetAll(ctx, sess.CharacterID)
if err != nil {
    return fmt.Errorf("handleRest: reload innate slots: %w", err)
}
sess.InnateTechs = innates
```

---

## Feature 7: Character Sheet

### 7a. Proto

File: `api/proto/game/v1/game.proto`

```protobuf
message InnateSlotView {
    string tech_id        = 1;
    int32  uses_remaining = 2;
    int32  max_uses       = 3;
}
```

Add to `CharacterSheetView` (next available field number after 45):

```protobuf
repeated InnateSlotView innate_slots = 46;
```

Run `make proto` to regenerate.

### 7b. `handleChar` population

File: `internal/gameserver/grpc_service.go`

```go
for techID, slot := range sess.InnateTechs {
    view.InnateSlots = append(view.InnateSlots, &gamev1.InnateSlotView{
        TechId:        techID,
        UsesRemaining: int32(slot.UsesRemaining),
        MaxUses:       int32(slot.MaxUses),
    })
}
```

### 7c. `main_test.go` migration update

File: `internal/storage/postgres/main_test.go`

Add `uses_remaining INT NOT NULL DEFAULT 0` to the `character_innate_technologies` CREATE TABLE in `applyAllMigrations`.

---

## Testing

All tests follow TDD + property-based testing (SWENG-5, SWENG-5a).

### `internal/gameserver/grpc_service_innate_test.go` — **package `gameserver`**

Same package as other grpc service tests (e.g., `grpc_service_selecttech_test.go`) for access to unexported helpers `testMinimalService` and `fakeSessionStream`. Use an in-memory fake `innateTechRepo` (implementing `InnateTechRepo`) and pre-populate `sess.InnateTechs` directly in test setup, following the pattern from `grpc_service_selecttech_test.go` lines 87–92.

- **REQ-INN1**: Session has `{"acid_spit": {MaxUses:1, UsesRemaining:1}}`; call `handleUse(uid, "req1", "acid_spit", stream)`; assert response contains `"You activate acid_spit."` and `UsesRemaining` decremented to 0 in session and fake repo.
- **REQ-INN2**: Session has `{"acid_spit": {MaxUses:1, UsesRemaining:0}}`; call `handleUse`; assert response contains `"No uses of acid_spit remaining."`.
- **REQ-INN3**: Session has empty `InnateTechs`; call `handleUse` with `"acid_spit"`; assert response contains `"You don't have innate tech acid_spit."`.
- **REQ-INN4**: Session has `{"blackout_pulse": {MaxUses:0, UsesRemaining:0}}`; call `handleUse`; assert response contains `"You activate blackout_pulse."`; fake repo Decrement NOT called.
- **REQ-INN5**: Session has mixed innate slots; call `handleUse` with no arg; assert response lists unlimited tech as `(unlimited)` and omits exhausted limited techs.
- **REQ-INN6**: Session has `{"acid_spit": {MaxUses:1, UsesRemaining:0}}`; call `svc.SetInnateTechRepo(fakeInnateTechRepo)` after `testMinimalService` and before `handleRest`; call `handleRest`; assert `sess.InnateTechs["acid_spit"].UsesRemaining == 1`; fake repo `RestoreAll` called once.

**Compilation note:** Adding `Decrement` and `RestoreAll` to `InnateTechRepo` will break any existing fakes that implement the interface. The existing `innateRepoInternal` fake in `grpc_service_levelup_tech_test.go` (implements only `GetAll`, `Set`, `DeleteAll`) must have stub `Decrement` and `RestoreAll` methods added (returning `nil`) to restore compilation.

### `internal/gameserver/technology_assignment_test.go` — **package `gameserver_test`**

- **REQ-INN7**: Call `AssignTechnologies` with a region containing one innate grant (`{ID:"acid_spit", UsesPerDay:1}`); assert `sess.InnateTechs["acid_spit"] == &InnateSlot{MaxUses:1, UsesRemaining:1}`; assert fake repo `Set` called with correct args.
- **REQ-INN8** (property, `pgregory.net/rapid`): Generate N ∈ [1,5] (MaxUses); call `handleUse` N+1 times; assert first N calls return activation message; assert (N+1)th call returns "no remaining"; assert `UsesRemaining` never goes below 0.

### `internal/storage/postgres/character_innate_tech_test.go` — **package `postgres_test`**

Uses `testDB` fixture from `main_test.go` (same pattern as other postgres test files). Requires a real test DB via `TestMain`.

- **REQ-INN9**: Call `Set(characterID, "acid_spit", 3)`; verify `GetAll` returns `{MaxUses:3, UsesRemaining:3}`; call `Decrement`; verify `UsesRemaining==2`; call `RestoreAll`; verify `UsesRemaining==3`.

### `internal/game/ruleset/loader_test.go` — **extend existing file**

- **REQ-CONTENT1**: Load all files in `content/technologies/innate/`; assert 11 loaded without error, each has `usage_type: innate`.
- **REQ-CONTENT2**: Load all region YAMLs; assert each has exactly one `innate_technologies` entry.

---

## Files Changed

| Action | Path | Notes |
|--------|------|-------|
| Modify | `internal/game/ruleset/region.go` | Add `InnateTechnologies []InnateGrant` |
| Modify | `internal/game/session/technology.go` | Add `UsesRemaining int` to `InnateSlot` |
| Create | `migrations/029_innate_uses_remaining.up.sql` | Add `uses_remaining` column |
| Create | `migrations/029_innate_uses_remaining.down.sql` | Drop column |
| Modify | `internal/gameserver/technology_assignment.go` | Extend `InnateTechRepo`; `AssignTechnologies` gains `region` param; all call sites in `grpc_service.go` and `technology_assignment_test.go` updated |
| Modify | `internal/storage/postgres/character_innate_tech.go` | Add Decrement, RestoreAll; update GetAll/Set |
| Modify | `internal/gameserver/grpc_service.go` | `handleUse` innate path; `handleRest` innate restore; `handleChar` innate slots |
| Modify | `api/proto/game/v1/game.proto` | Add `InnateSlotView`; add field 46 to `CharacterSheetView` |
| Modify | `internal/storage/postgres/main_test.go` | Add `uses_remaining` to `applyAllMigrations` |
| Create | `content/technologies/innate/blackout_pulse.yaml` | Old Town innate tech |
| Create | `content/technologies/innate/arc_lights.yaml` | Northeast innate tech |
| Create | `content/technologies/innate/pressure_burst.yaml` | Pearl District innate tech |
| Create | `content/technologies/innate/nanite_infusion.yaml` | SE Portland innate tech |
| Create | `content/technologies/innate/atmospheric_surge.yaml` | Pacific Northwest innate tech |
| Create | `content/technologies/innate/viscous_spray.yaml` | South innate tech |
| Create | `content/technologies/innate/chrome_reflex.yaml` | Southern California innate tech |
| Create | `content/technologies/innate/seismic_sense.yaml` | Mountain innate tech |
| Create | `content/technologies/innate/moisture_reclaim.yaml` | Midwest innate tech |
| Create | `content/technologies/innate/terror_broadcast.yaml` | North Portland innate tech |
| Create | `content/technologies/innate/acid_spit.yaml` | Gresham Outskirts innate tech |
| Modify | `content/regions/*.yaml` (all 11) | Add `innate_technologies:` block |
| Create | `internal/gameserver/grpc_service_innate_test.go` | REQ-INN1–INN6 grpc-level tests |
| Create | `internal/storage/postgres/character_innate_tech_test.go` | REQ-INN9 DB round-trip tests |
| Modify | `internal/gameserver/technology_assignment_test.go` | REQ-INN7, REQ-INN8 property test |
| Modify | `docs/requirements/FEATURES.md` | Mark Innate Technologies complete |

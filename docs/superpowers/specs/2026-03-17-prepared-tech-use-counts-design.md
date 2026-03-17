# Prepared Technology Use Counts — Design Spec

**Date:** 2026-03-17

---

## Goal

Each prepared technology slot is a single use, following PF2E spell-slot mechanics. Using a prepared tech expends that slot. Slots are restored when the player rests. Expended state is persisted so server restarts do not reset uses.

---

## Context

`PreparedSlot` in `internal/game/session/technology.go` currently holds only `TechID string`. There is no use tracking. `handleUse` in `internal/gameserver/grpc_service.go` handles feat and class-feature activation but ignores prepared techs. The `rest` command calls `RearrangePreparedTechs`, which clears and re-fills all slots — this naturally resets expended state if slots carry an `Expended` flag.

**Out of scope:** prepared tech effect resolution (damage, conditions); spontaneous tech use counts.

---

## Feature 1: `PreparedSlot.Expended` field

Add to `internal/game/session/technology.go`:

```go
type PreparedSlot struct {
    TechID   string
    Expended bool
}
```

---

## Feature 2: DB migration

New migration `migrations/027_prepared_tech_expended.up.sql`:

```sql
ALTER TABLE character_prepared_technologies
    ADD COLUMN expended BOOLEAN NOT NULL DEFAULT FALSE;
```

Down migration `migrations/027_prepared_tech_expended.down.sql`:

```sql
ALTER TABLE character_prepared_technologies
    DROP COLUMN expended;
```

---

## Feature 3: `PreparedTechRepo` extension

Add to the `PreparedTechRepo` interface in `internal/gameserver/technology_assignment.go`:

```go
// SetExpended marks or unmarks a single prepared slot as expended.
//
// Precondition: characterID > 0; level >= 1; index >= 0.
// Postcondition: character_prepared_technologies row has expended = value.
SetExpended(ctx context.Context, characterID int64, level, index int, expended bool) error
```

Add to `CharacterPreparedTechRepository` in `internal/storage/postgres/character_prepared_tech.go`:

```go
func (r *CharacterPreparedTechRepository) SetExpended(ctx context.Context, characterID int64, level, index int, expended bool) error {
    if characterID <= 0 {
        return fmt.Errorf("characterID must be > 0, got %d", characterID)
    }
    _, err := r.db.Exec(ctx,
        `UPDATE character_prepared_technologies
            SET expended = $1
          WHERE character_id = $2 AND slot_level = $3 AND slot_index = $4`,
        expended, characterID, level, index,
    )
    return err
}
```

`GetAll` must be updated to select and scan the `expended` column. The SELECT query becomes:

```sql
SELECT slot_level, slot_index, tech_id, expended
FROM character_prepared_technologies
WHERE character_id = $1
ORDER BY slot_level, slot_index
```

And the Scan call gains a fourth destination:

```go
if err := rows.Scan(&level, &index, &techID, &expended); err != nil { ... }
slot := &session.PreparedSlot{TechID: techID, Expended: expended}
```

`Set` (used during `RearrangePreparedTechs`) always writes `expended = false` — freshly prepared slots are ready to use:

```go
`INSERT INTO character_prepared_technologies (character_id, slot_level, slot_index, tech_id, expended)
 VALUES ($1, $2, $3, $4, FALSE)
 ON CONFLICT (character_id, slot_level, slot_index)
 DO UPDATE SET tech_id = EXCLUDED.tech_id, expended = FALSE`
```

---

## Feature 4: `handleUse` extension

In `internal/gameserver/grpc_service.go`, extend `handleUse` to handle prepared tech activation after the existing feat/class-feature path.

### No-arg path (list mode)

When `abilityID == ""`, append prepared tech entries to the choices list. For each distinct `TechID` in `sess.PreparedTechs`, count non-expended slots. Include only techs with at least one non-expended slot:

```
Shock Grenade (2 uses remaining)
Neural Disruptor (1 use remaining)
```

### Activation path

When `abilityID != ""` and no feat/class-feature matched:

1. Scan `sess.PreparedTechs` (ascending level, ascending index) for the first slot where `TechID == abilityID && !Expended`
2. If found:
   - Call `s.preparedTechRepo.SetExpended(ctx, sess.CharacterID, level, index, true)`
   - Set `sess.PreparedTechs[level][index].Expended = true`
   - Return message: `"You activate <techID>."`
3. If not found:
   - Return message: `"No prepared uses of <techID> remaining."`

---

## Feature 5: Character sheet

`PreparedSlotView` does not yet exist and `CharacterSheetView` has no prepared technology fields. This feature creates both.

Add new proto message to `api/proto/game/v1/game.proto`:

```protobuf
message PreparedSlotView {
    string tech_id  = 1;
    bool   expended = 2;
}
```

Add to `CharacterSheetView` (using next available field number after 43):

```protobuf
repeated PreparedSlotView prepared_slots = 44;
```

Run `make proto` to regenerate.

In the character sheet construction in `grpc_service.go` (in `handleChar` or equivalent), populate prepared slots:

```go
for level, slots := range sess.PreparedTechs {
    for _, slot := range slots {
        if slot != nil {
            view.PreparedSlots = append(view.PreparedSlots, &gamev1.PreparedSlotView{
                TechId:   slot.TechID,
                Expended: slot.Expended,
            })
        }
    }
}
```

---

## Feature 6: `rest` — no changes needed

`RearrangePreparedTechs` calls `preparedTechRepo.DeleteAll` then `Set` for each slot. `Set` always writes `expended = FALSE`. Expended state is reset automatically on rest.

---

## Testing

All tests use TDD + property-based testing (SWENG-5, SWENG-5a).

- **REQ-UC1**: `use <tech>` with a non-expended prepared slot expends it and returns activation message
- **REQ-UC2**: `use <tech>` with all slots for that tech expended returns "No prepared uses remaining"
- **REQ-UC3**: `use <tech>` with no slot for that tech returns "No prepared uses remaining"
- **REQ-UC4**: `use` (no arg) includes prepared techs with remaining use counts in the choices list
- **REQ-UC5**: After `rest`, previously expended slots are restored (Expended = false in session and DB)
- **REQ-UC6**: Expended state round-trips through DB (GetAll returns Expended = true after SetExpended)
- **REQ-UC7** (property): For any N non-expended slots of the same tech, exactly min(calls, N) slots become expended after that many `use` calls; the (N+1)th call returns "no remaining"
- **REQ-UC8**: Character sheet `PreparedSlotView.expended` reflects session state

---

## Constraints

- One new DB migration (027) — `ALTER TABLE` to add `expended` column
- One new repo method (`SetExpended`) — consistent with existing `PreparedTechRepo` pattern
- `handleUse` extended in-place — no new command needed
- One new proto message (`PreparedSlotView`) and one new field on `CharacterSheetView` (`prepared_slots`)
- Effect resolution is out of scope
- Spontaneous tech use counts are out of scope

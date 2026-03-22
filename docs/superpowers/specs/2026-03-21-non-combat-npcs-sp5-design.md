# Non-Combat NPCs — Sub-Project 5: Quest Giver, Crafter, and Fixer

## Overview

SP5 completes the remaining non-combat NPC types from the original design spec. It adds the `talk` command for Quest Givers, a named Crafter NPC (no new code — behavior deferred to the `crafting` feature), and the `FixerConfig` data model with validation for the `fixer` NPC type (commands deferred to the `wanted-clearing` feature). It also updates the `non-combat-npcs.md` feature doc to mark SP4 items complete.

References:
- Design spec: `docs/superpowers/specs/2026-03-20-non-combat-npcs-design.md`
- Wanted-clearing spec: `docs/superpowers/specs/2026-03-20-wanted-clearing-design.md`

---

## Requirements

### Quest Giver

- REQ-NPC-QG-1: `talk <npc_name>` MUST send a `TalkRequest` to the gameserver. If no `quest_giver` NPC matching `<npc_name>` (case-insensitive prefix) is present in the player's current room, the server MUST respond with `MessageEvent{Content: "No one named '<npc_name>' here."}`.
- REQ-NPC-QG-2: On a successful `talk`, the server MUST respond with one randomly selected line from the NPC's `PlaceholderDialog` as a `MessageEvent`.
- REQ-NPC-QG-3: Named NPC `gail_grinder_graves` MUST be defined as `npc_type: quest_giver` and placed in Scrapshack 23 (Rustbucket Ridge) with at least 3 placeholder dialog lines in lore-appropriate voice.

### Crafter

- REQ-NPC-CR-1: Named NPC `sparks` MUST be defined as `npc_type: crafter` with an explicit `crafter: {}` config block and placed in a lore-appropriate room in Rustbucket Ridge.

### Fixer

- REQ-WC-1: `FixerConfig.NPCVariance` MUST be > 0; fatal load error otherwise. *(from wanted-clearing spec)*
- REQ-WC-2: `FixerConfig.MaxWantedLevel` MUST be in range 1–4; fatal load error otherwise. *(from wanted-clearing spec)*
- REQ-WC-2a: `FixerConfig.BaseCosts` MUST contain all keys 1–4 with positive values; fatal load error otherwise. *(from wanted-clearing spec)*
- REQ-WC-3: Fixers MUST default to `flee` on combat start; MUST NOT enter initiative order. *(from wanted-clearing spec)*
- REQ-NPC-FX-1: `"fixer"` MUST be a recognized `npc_type` value; a missing or nil `fixer:` config block MUST be a fatal load error.
- REQ-NPC-FX-2: Named NPC `dex` MUST be defined as `npc_type: fixer` with valid `FixerConfig` and placed in a lore-appropriate room in Rustbucket Ridge.
- REQ-NPC-FX-3: The `fix` and `bribe` commands for Fixers are deferred to the `wanted-clearing` feature and MUST NOT be implemented in this sub-project.
- REQ-NPC-FX-4: `talk <fixer_name>` on a Fixer NPC is deferred to the `wanted-clearing` feature. In SP5, `handleTalk` MUST only match `npc_type == "quest_giver"`. Typing `talk <fixer>` MUST return `"No one named '<name>' here."` — this is intentional placeholder behavior until `wanted-clearing` extends `handleTalk` to support fixers.

### Feature Doc

- REQ-NPC-DOC-1: `docs/features/non-combat-npcs.md` MUST be updated to mark SP4 (Guard + Hireling) requirements as complete.

---

## Architecture

### Quest Giver Command

New proto message `TalkRequest` at field 101 in `ClientMessage` oneof (verify 101 is unused before use; use next available if taken):

```protobuf
message TalkRequest {
  string npc_name = 1;  // case-insensitive prefix matched against NPC name in room
}
```

New handler file `internal/gameserver/grpc_service_quest_giver.go` with function `handleTalk(sess *session.PlayerSession, req *gamev1.TalkRequest) []*gamev1.ServerMessage`:
- Looks up NPCs in `sess.RoomID` via `s.npcManager.InstancesInRoom(roomID)`
- Filters for `npc_type == "quest_giver"` and name prefix matches `req.NpcName` (case-insensitive)
- If no match: returns `MessageEvent{Content: "No one named '<name>' here."}`
- If match: picks `PlaceholderDialog[rand.Intn(len(dialog))]` and returns as `MessageEvent`

Wired in `grpc_service.go` dispatch switch on `*gamev1.TalkRequest`.

### Fixer Data Model

Add `FixerConfig` to `internal/game/npc/noncombat.go`:

```go
// FixerConfig holds the static configuration for a fixer NPC.
// Full bribe/fix command behavior is deferred to the wanted-clearing feature.
type FixerConfig struct {
    BaseCosts          map[int]int `yaml:"base_costs"`           // keys 1–4, all positive
    NPCVariance        float64     `yaml:"npc_variance"`         // must be > 0
    MaxWantedLevel     int         `yaml:"max_wanted_level"`     // 1–4
    ClearRecordQuestID string      `yaml:"clear_record_quest_id,omitempty"`
}

func (f FixerConfig) Validate() error // enforces REQ-WC-1, REQ-WC-2, REQ-WC-2a
```

Add `Fixer *FixerConfig \`yaml:"fixer,omitempty"\`` to the `Template` struct in `internal/game/npc/template.go` alongside the existing type-specific config fields (`Merchant`, `Guard`, `Healer`, etc.).

Register in `internal/game/npc/template.go`:
- Add `"fixer"` to the valid-types map
- Add switch case: nil `Fixer` field → fatal load error; non-nil → call `t.Fixer.Validate()`
- HTN personality default: add `"fixer": "flee"` to the personality defaults table (REQ-WC-3)

### Named NPCs

Three new YAML files under `content/npcs/`:

| File | NPC | Type | Location |
|------|-----|------|----------|
| `content/npcs/gail_grinder_graves.yaml` | Gail "Grinder" Graves | quest_giver | Scrapshack 23, Rustbucket Ridge |
| `content/npcs/sparks.yaml` | Sparks | crafter | The Tinker's Den, Rustbucket Ridge |
| `content/npcs/dex.yaml` | Dex | fixer | Back Alley, Rustbucket Ridge |

---

## File Map

| File | Change |
|------|--------|
| `api/proto/game/v1/game.proto` | Add `TalkRequest` (field 101, verify unused) to `ClientMessage` oneof |
| `internal/game/npc/noncombat.go` | Add `FixerConfig` struct + `Validate()` |
| `internal/game/npc/noncombat_test.go` | Tests for `FixerConfig.Validate()` |
| `internal/game/npc/template.go` | Register `"fixer"` type; add to personality defaults; add switch case |
| `internal/game/npc/template_test.go` | Tests for fixer template validation |
| `internal/gameserver/grpc_service_quest_giver.go` | New: `handleTalk` |
| `internal/gameserver/grpc_service_quest_giver_test.go` | New: tests for `talk` command |
| `internal/gameserver/grpc_service.go` | Wire `TalkRequest` dispatch case |
| `content/npcs/gail_grinder_graves.yaml` | Named quest giver NPC |
| `content/npcs/sparks.yaml` | Named crafter NPC |
| `content/npcs/dex.yaml` | Named fixer NPC |
| `docs/features/non-combat-npcs.md` | Mark SP4 guard/hireling requirements complete |

---

## Test Strategy

- REQ-NPC-TS-1: `grpc_service_quest_giver_test.go` MUST cover: talk to matching quest giver (returns random dialog line), talk to NPC not in room (returns "No one named" message), talk to non-quest-giver NPC (returns "No one named" message), case-insensitive name match.
- REQ-NPC-TS-2: Property-based tests (`pgregory.net/rapid`) in `grpc_service_quest_giver_test.go` MUST cover: for any non-empty `PlaceholderDialog`, `handleTalk` MUST always return a line that is one of the dialog entries (never out-of-bounds).
- REQ-NPC-TS-3: `noncombat_test.go` MUST cover: `FixerConfig.Validate()` rejects NPCVariance ≤ 0, rejects MaxWantedLevel 0 and 5, rejects BaseCosts missing any key 1–4, rejects BaseCosts with non-positive value, accepts valid config.
- REQ-NPC-TS-4: `template_test.go` MUST cover: fixer template without `fixer:` block errors, fixer template with invalid `FixerConfig` errors, valid fixer template loads.

---

## Non-Goals

- No `fix` command — deferred to `wanted-clearing` feature.
- No `bribe` command for fixers — deferred to `wanted-clearing` feature.
- No quest state tracking — deferred to `quests` feature.
- No crafter commands — deferred to `crafting` feature.
- No HTN flee/cower behavior at runtime — behavior is a personality default label; runtime flee execution is implemented as part of the HTN system (separate feature).

# Editor Commands

Formalizes the Editor role in the command system with a dedicated `CategoryEditor`, role enforcement helpers, NPC spawn, world-editing commands (addroom/addlink/removelink/setroom), and atomic YAML persistence to a PersistentVolumeClaim. See `docs/superpowers/specs/2026-03-21-editor-commands-design.md` for full design spec.

## Requirements

### Role Enforcement

- [ ] REQ-EC-1: `grant`, `summon_item`, `roomequip` recategorized to `CategoryEditor`
- [ ] REQ-EC-2: `setrole`, `teleport` remain in `CategoryAdmin`
- [ ] REQ-EC-3: Existing inline role string comparisons replaced with `requireEditor`/`requireAdmin`
- [ ] REQ-EC-4: `handleRoomEquip` gains a `requireEditor` call as the first validation step
- [ ] REQ-EC-5: New editor command handlers call `requireEditor` first
- [ ] REQ-EC-6: New admin command handlers call `requireAdmin` first

### NPC Spawn

- [ ] REQ-EC-7: `RespawnManager.GetTemplate` accessor added
- [ ] REQ-EC-8: `spawnnpc` creates runtime-only NPC instances; no YAML write
- [ ] REQ-EC-9: `spawnnpc` categorized as `CategoryEditor`

### Infrastructure

- [ ] REQ-EC-10: `deployments/k8s/mud/values.yaml` gains `content.persistentVolume` section
- [ ] REQ-EC-11: Deployment template conditionally mounts PVC at `/app/content`
- [ ] REQ-EC-12: Init container seeds PVC from image on first deploy (no-op if YAML files exist)
- [ ] REQ-EC-13: Gameserver verifies `content/` writability at startup
- [ ] REQ-EC-14: Startup warning logged when world-editing disabled; other functionality unaffected
- [ ] REQ-EC-15: All YAML writes atomic via `os.CreateTemp` + sync + `os.Rename`; temp cleaned on error
- [ ] REQ-EC-16: Successful YAML write triggers `worldMgr.ReloadZone(zone)` with freshly-parsed `*Zone`

### World Editing Commands

- [ ] REQ-EC-17: `addroom` writes atomically and hot-reloads
- [ ] REQ-EC-18: `addroom` categorized as `CategoryEditor`
- [ ] REQ-EC-19: `addlink` writes affected zone YAML(s) atomically and hot-reloads each
- [ ] REQ-EC-20: Same-zone `addlink` performs one YAML write and one hot-reload
- [ ] REQ-EC-21: `addlink` categorized as `CategoryEditor`
- [ ] REQ-EC-22: `removelink` writes atomically and hot-reloads
- [ ] REQ-EC-23: `removelink` categorized as `CategoryEditor`
- [ ] REQ-EC-24: `setroom` writes atomically and hot-reloads
- [ ] REQ-EC-25: `setroom` categorized as `CategoryEditor`
- [ ] REQ-EC-26: `setroom title`/`description` triggers updated room display for all players in room

### Editor Listing

- [ ] REQ-EC-27: `ecmds` categorized as `CategoryEditor`
- [ ] REQ-EC-28: `ecmds` lists all `CategoryEditor` commands sorted alphabetically
- [ ] REQ-EC-29: `help` includes `CategoryEditor` section for editor/admin roles only

### Architecture

- [ ] REQ-EC-30: `WorldEditor` is nil when `content/` not writable; handlers return disabled message
- [ ] REQ-EC-31: `ReloadZone` holds write lock for full replacement; callers must re-fetch `*Room` pointers
- [ ] REQ-EC-32: `HandlePlayerMessage` dispatch switch includes cases for all six new proto oneof fields

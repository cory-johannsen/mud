# Downtime

15 downtime activities mapped to Gunchete setting. Players declare an activity while in a Safe room; their character enters a busy state while the in-game clock advances. After the activity's duration elapses (all activities complete within 10 real minutes / 10 in-game hours), a single skill check resolves the outcome. Activity state persists across disconnects. See `docs/superpowers/specs/2026-03-20-downtime-design.md` for full design spec.

## Requirements

### Data Model

- [ ] REQ-DT-6: `DowntimeActivityID string`, `DowntimeCompletesAt time.Time`, `DowntimeBusy bool`, `DowntimeMetadata string` added to `PlayerSession`
- [ ] REQ-DT-6: `character_downtime` table: `character_id bigint PRIMARY KEY`, `activity_id text`, `completes_at timestamptz`, `activity_metadata jsonb`
- [ ] REQ-DT-6: On reconnect, load `character_downtime` row; if `completes_at` elapsed, resolve activity before returning control; else restore `DowntimeBusy = true`
- [ ] REQ-DT-14: `PlayerSession.ZoneCircumstanceBonus map[string]int` added (transient, not persisted)

### Busy State

- [ ] REQ-DT-5: Movement and combat commands blocked while `DowntimeBusy` is true; enforced at command dispatch in `grpc_service.go` (not via condition system)
- [ ] REQ-DT-9: Non-combat skills, technologies, feats, `chat`, `say`, `look`, `inventory`, `character`, `downtime` (status), `downtime cancel` remain available while busy

### Commands

- [ ] `downtime` — show active activity, time remaining, busy status
- [ ] `downtime list` — list all 15 activities with duration, skill, and location requirement
- [ ] `downtime cancel` — cancel active activity; REQ-DT-4: no material refund for Craft
- [ ] `downtime earn` — Earn Creds
- [ ] `downtime craft <recipe>` — Craft (requires `workshop` tag)
- [ ] `downtime retrain` — Retrain
- [ ] `downtime sickness` — Fight the Sickness (requires `clinic` tag)
- [ ] `downtime subsist` — Subsist
- [ ] `downtime forge` — Forge Papers
- [ ] `downtime recalibrate` — Recalibrate
- [ ] `downtime patchup` — Patch Up (requires `clinic` tag)
- [ ] `downtime flushit` — Flush It (requires `clinic` tag)
- [ ] `downtime intel <target>` — Run Intel
- [ ] `downtime analyze <item>` — Analyze Tech (requires `workshop` or `archive` tag)
- [ ] `downtime repair <item>` — Field Repair (requires `workshop` tag)
- [ ] `downtime decode` — Crack the Code
- [ ] `downtime cover` — Run a Cover
- [ ] `downtime pressure <npc>` — Apply Pressure

### Validation

- [ ] REQ-DT-1: Fail if not in a Safe room (room tags include `"safe"`)
- [ ] REQ-DT-2: Fail if activity-specific room tag requirement not met
- [ ] REQ-DT-3: Fail if another activity already active
- [ ] REQ-DT-8: Room tags validated at activity start time only; tag changes mid-activity do not cancel activity

### Activity Definitions

- [ ] 15 activity definitions with ID, duration, location requirement, skill, DC formula
- [ ] Duration table: Recalibrate=30min, Patch Up/Flush It=1h, Subsist/Analyze Tech/Field Repair=2h, Crack the Code=3h, Fight the Sickness/Run Intel/Apply Pressure/Craft(complexity 2)=4h, Forge Papers/Run a Cover/Earn Creds/Craft(complexity 3)=6h, Retrain/Craft(complexity 4)=8h

### Resolution

- [ ] REQ-DT-10: All skill checks use standard skill check engine; resolvers do not manually compute modifiers
- [ ] Earn Creds: Rigging/Intel/Rep (highest) vs zone `settlement_dc` (default 15); REQ-DT-12: zones define `settlement_dc int`
- [ ] Craft: Rigging vs recipe DC; produced items added to character inventory via crafting feature's item delivery logic
- [ ] Fight the Sickness: Patch Job vs disease DC; REQ-DT-11: disease conditions expose `Severity int` and `MaxSeverity int`
- [ ] Subsist: Scavenging or Factions vs zone DC; REQ-DT-13: `fatigued` condition must exist in registry
- [ ] Forge Papers: Hustle vs DC 15; produces forgery document item
- [ ] Recalibrate: 1d20 no modifier (20=crit success, 11–19=success, 2–10=failure, 1=crit fail); restores `FocusPoints` to `MaxFocusPoints`
- [ ] Patch Up: Patch Job vs DC 15; heals 4×/2×/1×/0 level HP by tier
- [ ] Flush It: Patch Job vs poison/drug DC; REQ-DT-11: poison/drug conditions expose `Stage int` and `MaxStage int`
- [ ] Run Intel: Smooth Talk vs target DC (default 15); facts are authored content strings keyed by target name/NPC ID
- [ ] Analyze Tech: Tech Lore vs item DC; items may define `hidden_properties []string` revealed only on critical success
- [ ] Field Repair: Rigging vs item DC; 100%/100%/50%/−1 durability by tier
- [ ] Crack the Code: Intel or Tech Lore vs document DC; document decoded or partial on lower tiers
- [ ] Run a Cover: Hustle vs DC 15; cover duration tracked as game-time state; critical success grants transient `ZoneCircumstanceBonus` +1 Hustle
- [ ] Apply Pressure: Hard Look vs `10 + npc.Awareness`; NPC compliance tracked per-NPC (depends on `npc-awareness` feature)
- [ ] Retrain: Always succeeds; 1d20 at start determines duration (20=6h, else=8h); change applied on completion

### Architecture

- [ ] `internal/game/downtime/activity.go` — Activity interface, 15 activity definitions
- [ ] `internal/game/downtime/engine.go` — DowntimeEngine: `Start()`, `Cancel()`, `CheckCompletion(sess, gameClock)`
- [ ] `internal/game/downtime/resolver.go` — per-activity resolution logic
- [ ] REQ-DT-7: Game clock tick handler calls `CheckCompletion()` for all sessions with `DowntimeActivityID != ""`
- [ ] `CharacterDowntimeRepository` in `internal/storage/postgres/`: `Save()`, `Load()`, `Clear()`
- [ ] `HandlerDowntime` constant, `DowntimeRequest{subcommand, args}` proto message in `ClientMessage` oneof
- [ ] Bridge handler + `handleDowntime` case in `grpc_service.go`
- [ ] Room `workshop`, `clinic`, `archive` tags added to content YAML for appropriate rooms

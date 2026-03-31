# Gunchete MUD

A dystopian sci-fi text adventure set in post-collapse Portland, OR.
Three-tier architecture: Telnet frontend + Web client + Pitaya game backend connected via gRPC.

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.26+ | [go.dev](https://go.dev/dl/) |
| Docker + Docker Compose | v2+ | [docker.com](https://www.docker.com/) |
| mise | latest | `curl https://mise.run \| sh` |
| Node.js + npm | 20+ | [nodejs.org](https://nodejs.org/) |
| protoc + plugins | latest | See [Proto Setup](#proto-setup) |

## Quick Start (Docker)

```bash
# Start PostgreSQL, run migrations, launch gameserver + frontend
docker compose -f deployments/docker/docker-compose.yml up

# Connect via Telnet
telnet localhost 4000

# Or open the web client
open http://localhost:8080
```

## Local Development

### 1. Start the Database

```bash
docker compose -f deployments/docker/docker-compose.yml up -d postgres
```

### 2. Run Migrations

```bash
make build-migrate
./bin/migrate -config configs/dev.yaml
```

### 3. Start the Game Server

```bash
make run-gameserver
# Listening on :50051 (gRPC)
```

### 4. Start the Telnet Frontend

In a separate terminal:

```bash
make run-frontend
# Listening on :4000 (Telnet)
```

### 5. Connect

```bash
telnet localhost 4000
```

### Web Client (optional)

Build and run the browser-based client:

```bash
# Install JS dependencies and build the React app
make ui-install
make ui-build

# Build and run the web server
make build-webclient
./bin/webclient -config configs/dev.yaml
# Listening on :8080
```

## Build

```bash
# Build all binaries to bin/
make build

# Individual targets
make build-frontend          # Telnet server
make build-gameserver        # gRPC game server
make build-webclient         # Web client server (also runs ui-build)
make build-devserver         # Development server variant
make build-migrate           # Database migration runner
make build-import-content    # Bulk content importer
make build-setrole           # Admin role management CLI
make build-seed-claude-accounts  # E2E test account seeding
```

## Testing

```bash
# Full test suite (unit + integration + postgres, requires Docker)
make test

# Fast tests only — no Docker required (~5s)
make test-fast

# Postgres integration tests only
make test-postgres

# End-to-end integration tests (spins up full stack)
make test-e2e

# Coverage report → coverage.html
make test-cover
```

> **Note:** `test-fast` and `test-postgres` run as parallel sub-targets under `make -j`. Postgres tests spin up Docker containers and run bcrypt property tests under the race detector; they have a 10-minute timeout. `test-e2e` spins up ephemeral Postgres, gameserver, and frontend subprocesses and drives them via a headless telnet client.

## Kubernetes Deployment

The canonical deployment targets a Kubernetes cluster. The registry is `registry.johannsen.cloud:5000`.

```bash
# Build images, push to registry, and upgrade the Helm release
make k8s-redeploy

# Individual steps
make docker-push      # Build + push all images (gameserver, frontend, webclient)
make helm-install     # First-time install
make helm-upgrade     # Upgrade existing release
```

> The Helm release is named `mud` in namespace `mud`. If a release is stuck in `pending-upgrade`, run: `helm rollback mud <last-good-revision> -n mud`

## Proto Setup

If you modify `api/proto/game/v1/game.proto`, regenerate Go code:

```bash
# Install code generators (one-time)
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Regenerate Go bindings
make proto

# Regenerate TypeScript bindings (for web client)
make proto-ts
```

## Wire (Dependency Injection)

The project uses [Wire](https://github.com/google/wire) for compile-time dependency injection:

```bash
# Regenerate wire_gen.go files
make wire

# Verify wire output is up to date
make wire-check
```

## Project Layout

```
cmd/
  frontend/             Telnet acceptor + auth + character flow + gRPC bridge
  gameserver/           Pitaya game server + gRPC service
  webclient/            HTTP/WebSocket web client server
    ui/                 React frontend (TypeScript + Vite)
  devserver/            Development server variant (wire-generated)
  migrate/              Database migration runner
  import-content/       Bulk content importer CLI
  setrole/              Admin role management CLI
  seed-claude-accounts/ E2E test account seeding tool

api/proto/              Protobuf definitions (game/v1/game.proto)

content/
  archetypes/           Archetype YAML files (aggressor, criminal, drifter, …)
  conditions/           Condition YAML files (dying, prone, stunned, frightened, …)
  jobs/                 Job YAML files (90+ playable classes)
  npcs/                 NPC template YAML files
  recipes/              Crafting recipe YAML files
  regions/              Home region YAML files
  scripts/
    conditions/         Global Lua condition scripts (__global__ VM)
    zones/              Per-zone Lua scripts (combat hooks, room hooks)
  teams/                Team YAML files (gun, machete)
  technologies/         Technology YAML files (feats/abilities)
  weather/              Weather type definitions
  zones/                World zone + room YAML files (16 zones)

internal/
  client/               Shared client library (auth, feed, history, render, session)
  config/               Configuration loading
  frontend/
    handlers/           Auth handler, character flow, game bridge, text renderer
    telnet/             Raw Telnet connection handling
  game/
    character/          Character model + builder
    combat/             PF2E-inspired combat engine (rounds, actions, conditions,
                        initiative, Lua hook integration)
    command/            Command registry + parser
    condition/          Condition definitions, active condition tracking, modifiers
    crafting/           Recipe data model, material inventory, crafting checks
    dice/               Dice expression parser + roller (CryptoSource, LoggedRoller)
    downtime/           Downtime activity engine, real-time clock, queue
    faction/            Faction data model, reputation/allegiance tracking
    inventory/          Item and equipment management
    npc/                NPC templates, instance spawning, manager, HTN behaviors
    quest/              Quest system, objectives, progress tracking, NPC wiring
    reaction/           Ready action / reaction trigger engine
    ruleset/            Region, team, archetype, job, technology YAML loaders
    session/            Player session + room presence (Pitaya bridge)
    substance/          Drug/alcohol/medicine/poison effect system
    technology/         Technology data model, pools, use counts
    trap/               Trap types, trigger/detect/disarm mechanics
    world/              Zone/room model, YAML loader, navigation manager,
                        exploration modes, world map
  gameserver/           gRPC service, Pitaya setup, world/chat/combat handlers,
                        weather manager, roving NPC manager
  observability/        Structured logging setup (zap)
  scripting/            GopherLua sandbox, scripting.Manager, engine.* API modules
  server/               Service lifecycle management
  storage/
    postgres/           Account + character repositories

migrations/             SQL migration files (042 migrations)
deployments/            Docker Compose + Dockerfiles + Kubernetes Helm chart
configs/                Configuration files (dev.yaml)
docs/
  architecture/         Architecture diagrams and design documents
  features/             Feature specifications and status index
  requirements/         Product requirements documents
```

## Gameplay

After connecting via Telnet (or via the web client), use these commands in-game:

### Movement

| Command | Aliases | Description |
|---------|---------|-------------|
| `north` | `n` | Move north |
| `south` | `s` | Move south |
| `east` | `e` | Move east |
| `west` | `w` | Move west |
| `northeast` | `ne` | Move northeast |
| `northwest` | `nw` | Move northwest |
| `southeast` | `se` | Move southeast |
| `southwest` | `sw` | Move southwest |
| `up` | `u` | Move up |
| `down` | `d` | Move down |
| `climb` | `cl` | Climb a climbable surface |
| `swim` | `sm` | Swim through water |
| `travel` | | Fast travel to a discovered zone |

### World

| Command | Aliases | Description |
|---------|---------|-------------|
| `look` | `l` | Describe current room and occupants |
| `exits` | | List available exits |
| `examine <target>` | `ex` | Inspect an NPC or object in the room |
| `who` | | List players in the room |
| `map` | | Display automap for the current zone |
| `inventory` | `inv`, `i` | Show backpack contents and currency |
| `get <item>` | `take` | Pick up item from room floor |
| `drop <item>` | | Drop an item from your backpack |
| `inspect <item>` | | Inspect an item in your backpack |
| `balance` | `bal` | Show your currency (Rounds/Clips/Crates) |
| `char` | `sheet` | Display your character sheet |
| `skills` | `sk` | Display your skill proficiencies |
| `feats` | `ft` | Display your feats |
| `use` | | Activate an active feat or item |
| `interact` | `int` | Interact with an item in the room |

### NPC Interaction

| Command | Aliases | Description |
|---------|---------|-------------|
| `talk <npc>` | | Talk to a quest giver NPC |
| `heal <npc>` | | Ask a healer to restore your HP |
| `browse <npc>` | | Browse a merchant's inventory |
| `buy <npc> <item>` | | Buy an item from a merchant |
| `sell <npc> <item>` | | Sell an item to a merchant |
| `negotiate <npc>` | `neg` | Negotiate prices with a merchant |
| `deposit <npc> <amount>` | | Deposit credits with a banker |
| `withdraw <npc> <amount>` | | Withdraw credits from a banker |
| `stash <npc>` | `stashbal` | Check your stash balance |
| `hire <npc>` | | Hire a hireling NPC |
| `dismiss` | | Dismiss your current hireling |
| `train <npc> <job>` | | Train a job with a job trainer NPC |
| `bribe` | | Bribe law enforcement to reduce wanted level |
| `surrender` | | Surrender to law enforcement |
| `uncurse <npc>` | | Ask a chip doc to remove a cursed item |

### Factions

| Command | Aliases | Description |
|---------|---------|-------------|
| `faction` | | Show your current faction, tier, rep, and perks |
| `faction_info <id>` | | Show public information about a faction |
| `faction_standing` | | Show your standing in all tracked factions |
| `change_rep <id>` | | Pay a Fixer to improve your faction standing |

### Combat

| Command | Aliases | Description |
|---------|---------|-------------|
| `attack <target>` | `att`, `kill` | Attack a target (1 AP) |
| `strike <target>` | `st` | Full attack routine (2 AP, two strikes with MAP) |
| `burst <target>` | `bf` | Burst fire (2 AP, 2 attacks) |
| `auto` | `af` | Automatic fire at all enemies (3 AP) |
| `throw` | `gr` | Throw an explosive at the current room |
| `reload` | `rl` | Reload equipped weapon (1 AP) |
| `raise` | `rs` | Raise shield (+2 AC until start of next turn) |
| `cover` | `tc` | Take cover (+2 AC for the encounter; 1 AP in combat) |
| `action [name] [target]` | `act` | Activate an archetype or job action |
| `stride` | `str`, `close`, `move`, `approach` | Close distance toward target (1 AP) |
| `step` | | Step 5 ft without triggering Reactive Strikes |
| `tumble` | | Tumble through an enemy's space (Acrobatics vs Level+10) |
| `hide` | | Attempt to hide (Stealth; 1 AP) |
| `sneak` | | Move while hidden (1 AP) |
| `divert` | `div` | Create a diversion to gain hidden (1 AP) |
| `seek` | | Scan for hidden enemies (Perception; 1 AP) |
| `motive <target>` | `mot` | Read an NPC's intentions (1 AP) |
| `feint <target>` | | Apply flat-footed to target (grift; 1 AP) |
| `demoralize <target>` | `dem` | Reduce target AC/attack (smooth_talk; 1 AP) |
| `seduce <target>` | `sed` | Charm an NPC (flair; 1 AP) |
| `grapple <target>` | `grp` | Apply grabbed condition to target (1 AP) |
| `trip <target>` | `trp` | Apply prone to target (1 AP) |
| `disarm <target>` | `dsm` | Remove NPC weapon (1 AP) |
| `shove <target>` | | Push target back 5–10 ft (1 AP) |
| `escape` | `esc` | Escape from grabbed condition (1 AP) |
| `aid <ally>` | `fa` | Aid an ally's attack roll (2 AP) |
| `ready <action> when <trigger>` | `rdy` | Ready a reaction for a trigger (2 AP) |
| `delay` | `dl` | Bank remaining AP (up to 2) for next round |
| `calm` | | Attempt to calm your worst mental state |
| `pass` | `p` | Forfeit remaining action points |
| `flee` | `run` | Attempt to flee combat |
| `join` | | Join active combat in the room |
| `decline` | | Decline to join active combat |
| `status` | `cond` | Show your active conditions |
| `deploy_trap <item>` | `deploy` | Arm a trap item at your current position (1 AP) |
| `disarm_trap <name>` | `dt` | Disarm a detected trap (Thievery) |
| `combat_default` | `cd` | Set your default combat action |

### Equipment

| Command | Aliases | Description |
|---------|---------|-------------|
| `equip <item> [slot]` | `eq` | Equip a weapon |
| `unequip <slot>` | `ueq` | Unequip an item from a slot |
| `equipment` | `gear` | Show all equipped items |
| `loadout [1\|2]` | `lo`, `prep`, `kit` | Display or swap weapon presets |
| `wear <item> <slot>` | | Equip a piece of armor |
| `remove <slot>` | `rem` | Remove armor and return to inventory |
| `repair <item>` | | Repair a damaged item |
| `affix <material> <item>` | | Affix a precious material to equipped gear |

### Crafting

| Command | Aliases | Description |
|---------|---------|-------------|
| `materials [category]` | `mats` | List your material inventory |
| `craft list\|<recipe>\|confirm` | `cr` | Craft an item |
| `scavenge` | | Scavenge the current area for materials |

### Exploration

| Command | Aliases | Description |
|---------|---------|-------------|
| `explore [mode\|off]` | `exp` | Set or query your exploration mode |

Exploration modes: `lay_low`, `hold_ground`, `active_sensors`, `case_it`, `run_point`, `shadow <ally>`, `poke_around`

### Downtime

| Command | Aliases | Description |
|---------|---------|-------------|
| `downtime [list\|<activity>\|cancel]` | `dtime` | Manage downtime activities |

### Character

| Command | Aliases | Description |
|---------|---------|-------------|
| `jobs` | | List your current jobs |
| `setjob <job>` | | Set your active job |
| `levelup <ability>` | `lu` | Assign a pending ability boost |
| `trainskill <skill>` | `ts` | Advance a skill proficiency rank |
| `selecttech` | | Select pending technology upgrades |
| `rest` | | Rest to rearrange prepared technology slots |
| `heropoint reroll\|stabilize` | `hp` | Spend a hero point |
| `proficiencies` | `prof` | Display armor and weapon proficiencies |
| `class_features` | `cf` | List your class features |

### Groups

| Command | Aliases | Description |
|---------|---------|-------------|
| `group` | | Create a group or show group info |
| `invite <player>` | | Invite a player to your group |
| `accept` | | Accept a pending group invitation |
| `gdecline` | | Decline a pending group invitation |
| `kick <player>` | | Kick a player from your group (leader only) |
| `ungroup` | | Leave your group |

### Communication

| Command | Aliases | Description |
|---------|---------|-------------|
| `say <message>` | | Speak to players in the room |
| `emote <action>` | `em` | Perform an emote |

### System

| Command | Aliases | Description |
|---------|---------|-------------|
| `help` | `?` | Show available commands |
| `hotbar [<slot> <text>]` | | Manage hotbar slots (0–9) |
| `switch` | | Switch to a different character |
| `quit` | `exit` | Disconnect |

## Character Creation

1. Register an account: `register <username> <password>`
2. Log in: `login <username> <password>`
3. Choose your **home region** (affects starting ability scores)
4. Choose your **team**: Gun or Machete (your faction in post-collapse Portland)
5. Choose your **job** (general jobs available to all + team-exclusive jobs)
6. Confirm and enter the world

## Combat System

Combat uses a PF2E-inspired action economy:

- Each round grants **3 action points (AP)**; the round has a fixed duration timer
- **Attack** costs 1 AP; **Strike** costs 2 AP and makes two attacks (the second at −5 MAP)
- Attack rolls are `d20 + level` vs. target AC; outcomes are critical success, success, or miss
- Critical success applies double damage and extra conditions (e.g., flat-footed)
- Conditions (dying, wounded, prone, stunned, frightened, flat-footed, grabbed, hidden) modify attack rolls, AC, and available actions
- Dying condition triggers death saves; wounded stacks increase dying severity on re-down
- Hero points can be spent to reroll or auto-stabilize
- Multiplayer combat: multiple players can join the same combat; groups share initiative

## Technology System

Characters gain Technology — the game's equivalent of spells and special abilities:

- **Prepared technologies** are arranged into slots during a short rest; use counts restore on long rest
- **Spontaneous technologies** are selected at level-up and use a pool of charges
- **Innate technologies** are always available, tied to archetype or job
- **Passive technologies** apply automatic effects (stat bonuses, condition immunities, etc.)
- Focus technologies are powered by a Focus Point pool restored via the `downtime` Recalibrate activity

## Lua Scripting

The gameserver embeds a [GopherLua](https://github.com/yuin/gopher-lua) scripting engine.
Scripts run in sandboxed VMs — one per zone plus a shared `__global__` VM for condition scripts.

### Hook Points

| Hook | Arguments | Return | Where called |
|------|-----------|--------|--------------|
| `lua_on_apply` | `uid, cond_id, stacks, duration` | — | When a condition is applied |
| `lua_on_remove` | `uid, cond_id` | — | When a condition is removed |
| `lua_on_tick` | `uid, cond_id, stacks, duration_remaining` | — | Each combat round tick |
| `on_attack_roll` | `attacker_uid, target_uid, roll_total, ac` | modified total or `nil` | Before hit/miss determination |
| `on_damage_roll` | `attacker_uid, target_uid, damage` | modified damage or `nil` | Before damage is applied |
| `on_condition_apply` | `target_uid, condition_id, stacks` | `false` to cancel or `nil` | Before a condition is applied |
| `on_enter` | `uid, room_id, from_room_id` | — | When a player enters a room |
| `on_exit` | `uid, room_id, to_room_id` | — | When a player exits a room |
| `on_look` | `uid, room_id` | — | When a player looks at a room |

### Engine API

Scripts access game state via the `engine` global:

```lua
engine.log.info("message")           -- structured logging (debug/info/warn/error)
engine.dice.roll("2d6+3")            -- returns {total, dice, modifier}
engine.entity.get_hp(uid)            -- entity HP
engine.entity.get_ac(uid)            -- entity AC
engine.entity.get_name(uid)          -- entity name
engine.entity.get_conditions(uid)    -- list of active condition IDs
engine.combat.apply_condition(uid, cond_id, stacks, duration)
engine.combat.apply_damage(uid, hp)
engine.combat.query_combatant(uid)   -- returns {uid, name, hp, max_hp, ac, conditions}
engine.world.broadcast(room_id, msg)
engine.world.query_room(room_id)     -- returns {id, title}
```

### Configuration

```yaml
# content/zones/downtown.yaml
zone:
  script_dir: content/scripts/zones/downtown
  script_instruction_limit: 50000   # optional; defaults to 100,000
```

```bash
# gameserver flags
--script-root content/scripts          # empty string disables scripting
--condition-scripts content/scripts/conditions
```

## Linting

```bash
make lint
# Requires golangci-lint: https://golangci-lint.run/usage/install/
```

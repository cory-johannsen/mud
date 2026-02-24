# Gunchete MUD

A dystopian sci-fi text adventure set in post-collapse Portland, OR.
Two-tier architecture: Telnet frontend + Pitaya game backend connected via gRPC.

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.26+ | [go.dev](https://go.dev/dl/) |
| Docker + Docker Compose | v2+ | [docker.com](https://www.docker.com/) |
| mise | latest | `curl https://mise.run \| sh` |
| protoc + plugins | latest | See [Proto Setup](#proto-setup) |

## Quick Start (Docker)

```bash
# Start PostgreSQL, run migrations, launch gameserver + frontend
docker compose -f deployments/docker/docker-compose.yml up

# Connect via Telnet
telnet localhost 4000
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

### 4. Start the Frontend

In a separate terminal:

```bash
make run-frontend
# Listening on :4000 (Telnet)
```

### 5. Connect

```bash
telnet localhost 4000
```

## Build

```bash
# Build all binaries to bin/
make build

# Individual targets
make build-frontend
make build-gameserver
make build-migrate
make build-import-content
```

## Testing

```bash
# Full test suite (unit + integration + postgres, requires Docker)
make test

# Fast tests only — no Docker required (~5s)
make test-fast

# Postgres integration tests only
make test-postgres

# Coverage report → coverage.html
make test-cover
```

> **Note:** `test-fast` and `test-postgres` run as parallel sub-targets under `make -j`. Postgres tests spin up Docker containers and run bcrypt property tests under the race detector; they have a 10-minute timeout.

## Proto Setup

If you modify `api/proto/game/v1/game.proto`, regenerate Go code:

```bash
# Install code generators (one-time)
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Regenerate
make proto
```

## Project Layout

```
cmd/
  frontend/         Telnet acceptor + auth + character flow + gRPC bridge
  gameserver/       Pitaya game server + gRPC service
  migrate/          Database migration runner
  import-content/   Bulk content importer CLI

api/proto/          Protobuf definitions (game/v1/game.proto)

content/
  archetypes/       Archetype YAML files (aggressor, criminal, drifter, …)
  conditions/       Condition YAML files (dying, prone, stunned, frightened, …)
  jobs/             Job YAML files (90+ playable classes)
  npcs/             NPC template YAML files (ganger, scavenger, lieutenant)
  regions/          Home region YAML files
  scripts/
    conditions/     Global Lua condition scripts (__global__ VM)
    zones/          Per-zone Lua scripts (combat hooks, room hooks)
  teams/            Team YAML files (gun, machete)
  zones/            World zone + room YAML files

internal/
  config/           Configuration loading
  frontend/
    handlers/       Auth handler, character flow, game bridge, text renderer
    telnet/         Raw Telnet connection handling
  game/
    character/      Character model + builder
    combat/         PF2E-inspired combat engine (rounds, actions, conditions,
                    initiative, Lua hook integration)
    command/        Command registry + parser
    condition/      Condition definitions, active condition tracking, modifiers
    dice/           Dice expression parser + roller (CryptoSource, LoggedRoller)
    npc/            NPC templates, instance spawning, manager
    ruleset/        Region, team, archetype, job YAML loaders
    session/        Player session + room presence (Pitaya bridge)
    world/          Zone/room model, YAML loader, navigation manager
  gameserver/       gRPC service, Pitaya setup, world/chat/combat handlers
  observability/    Structured logging setup (zap)
  scripting/        GopherLua sandbox, scripting.Manager, engine.* API modules
  server/           Service lifecycle management
  storage/
    postgres/       Account + character repositories

migrations/         SQL migration files
deployments/        Docker Compose + Dockerfiles
configs/            Configuration files (dev.yaml)
docs/plans/         Design documents and implementation plans
```

## Gameplay

After connecting via Telnet, use these commands in-game:

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

### World

| Command | Aliases | Description |
|---------|---------|-------------|
| `look` | `l` | Describe current room and occupants |
| `exits` | | List available exits |
| `examine <target>` | | Inspect an NPC or object in the room |
| `who` | | List players in the room |

### Combat

| Command | Aliases | Description |
|---------|---------|-------------|
| `attack <target>` | | Attack a target (1 AP, single strike) |
| `strike <target>` | | Full attack routine (2 AP, two strikes with MAP) |
| `flee` | | Attempt to flee combat |
| `pass` | | Forfeit remaining action points for the round |
| `status` | `cond` | Show your active conditions |

### Communication

| Command | Aliases | Description |
|---------|---------|-------------|
| `say <message>` | | Speak to players in the room |
| `emote <action>` | `em` | Perform an emote |

### System

| Command | Aliases | Description |
|---------|---------|-------------|
| `help` | | Show available commands |
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
- Conditions (dying, wounded, prone, stunned, frightened, flat-footed) modify attack rolls, AC, and available actions
- Dying condition triggers death saves; wounded stacks increase dying severity on re-down

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

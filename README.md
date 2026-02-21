# Gunchete MUD

A dystopian sci-fi text adventure set in post-collapse Portland, OR.
Two-tier architecture: Telnet frontend + Pitaya game backend connected via gRPC.

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.22+ | [go.dev](https://go.dev/dl/) |
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
```

## Testing

```bash
# Full test suite (requires Docker for container-based integration tests)
make test

# With coverage report
make test-cover
# Opens coverage.html
```

> **Note:** The postgres integration tests spin up Docker containers and run bcrypt property tests under the race detector. The full suite takes 5–10 minutes.

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
  frontend/       Telnet acceptor + auth + character flow + gRPC bridge
  gameserver/     Pitaya game server + gRPC service
  migrate/        Database migration runner

api/proto/        Protobuf definitions (game/v1/game.proto)

content/
  regions/        Home region YAML files
  teams/          Team YAML files (gun, machete)
  archetypes/     Archetype YAML files (broad character categories)
  jobs/           Job YAML files (concrete playable classes)
  zones/          World zone + room YAML files

internal/
  config/         Configuration loading
  frontend/
    handlers/     Auth handler, character flow, game bridge, text renderer
    telnet/       Raw Telnet connection handling
  game/
    character/    Character model + builder
    command/      Command registry + parser
    ruleset/      Region, team, archetype, job loaders
    session/      Player session + room presence (Pitaya bridge)
    world/        Zone/room model, YAML loader, navigation manager
  gameserver/     gRPC service, Pitaya setup, world/chat handlers
  observability/  Structured logging setup
  server/         Service lifecycle management
  storage/
    postgres/     Account + character repositories

migrations/       SQL migration files
deployments/      Docker Compose + Dockerfiles
configs/          Configuration files (dev.yaml)
```

## Gameplay

After connecting via Telnet, use these commands in-game:

| Command | Aliases | Description |
|---------|---------|-------------|
| `look` | `l` | Describe current room |
| `exits` | | List available exits |
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
| `say <message>` | | Speak to players in the room |
| `emote <action>` | `em` | Perform an emote |
| `who` | | List players in the room |
| `help` | | Show available commands |
| `quit` | `exit` | Disconnect |

## Character Creation

1. Register an account: `register <username> <password>`
2. Log in: `login <username> <password>`
3. Choose your **home region** (affects starting ability scores)
4. Choose your **team**: Gun or Machete (your faction in post-collapse Portland)
5. Choose your **job** (general jobs available to all + team-exclusive jobs)
6. Confirm and enter the world

## Linting

```bash
make lint
# Requires golangci-lint: https://golangci-lint.run/usage/install/
```

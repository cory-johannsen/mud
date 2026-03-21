# Wire Dependency Injection Refactor

Replace manual dependency wiring in all three binaries with Google Wire code-generated injectors. `NewGameServiceServer` is refactored from ~30 individual parameters to grouped dependency structs. See `docs/superpowers/specs/2026-03-20-wire-refactor-design.md` for the full design spec.

## Requirements

- [ ] Provider sets
  - REQ-WIRE-5: Provider functions MUST live in `providers.go` files within the packages that own the types, not in `cmd/` directories.
  - REQ-WIRE-10: `StorageProviders` MUST include `wire.Bind` calls for every interface/concrete pair consumed by `NewGameServiceServer`.
  - [ ] `internal/storage/postgres/providers.go` — `StorageProviders` with all repo constructors and `wire.Bind` mappings
  - [ ] `internal/gameserver/providers.go` — `HandlerProviders` and `ServerProviders`
  - [ ] `internal/frontend/providers.go` — `FrontendProviders`
  - [ ] Per-domain `providers.go` files in `internal/game/world`, `npc`, `condition`, `inventory`, `ruleset`, `technology`, `ai`, `dice`, `combat`, `mentalstate`
  - [ ] `internal/scripting/providers.go`
- [ ] `NewGameServiceServer` refactor
  - REQ-WIRE-4: MUST accept `StorageDeps`, `ContentDeps`, and `HandlerDeps` structs in place of individual parameters.
- [ ] Wire injectors
  - REQ-WIRE-2: `wire_gen.go` MUST be committed to each binary's directory with the `!wireinject` build tag.
  - REQ-WIRE-3: `make wire` MUST regenerate all three `wire_gen.go` files cleanly.
  - REQ-WIRE-6: Flag parsing MUST remain in each binary's `main.go`; wire MUST NOT be responsible for CLI flag binding.
  - REQ-WIRE-9: The XP service setter injection block MUST remain in `main.go` post-`Initialize()`. It MUST NOT be forced into a wire provider.
  - [ ] `cmd/gameserver/wire.go` + `wire_gen.go`
  - [ ] `cmd/devserver/wire.go` + `wire_gen.go`
  - [ ] `cmd/frontend/wire.go` + `wire_gen.go`
- [ ] Build integration
  - REQ-WIRE-7: `wire` MUST be pinned via `tools/tools.go` and available in the `mise`-managed toolchain.
  - REQ-WIRE-11: `make wire-check` MUST diff regenerated `wire_gen.go` files against committed versions and fail if they differ.
  - [ ] `tools/tools.go` — wire tool pin
  - [ ] `mise.toml` updated with wire
  - [ ] `make wire` target added to Makefile
  - [ ] `make wire-check` target added to Makefile
- [ ] Tests
  - REQ-WIRE-1: All tests passing before the refactor MUST pass after. No new skips permitted.
  - REQ-WIRE-8: This refactor MUST introduce no behavior changes.

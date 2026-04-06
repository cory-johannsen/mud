# Technology ID Refactor — Gunchete Names

Renames all technology IDs from PF2E-sourced names to snake_case Gunchete names via a two-phase CLI tool, with YAML file renames, job/archetype reference updates, Go source updates, and a DB migration.

## Requirements

See `docs/superpowers/specs/2026-04-03-technology-id-refactor-design.md` for the full spec.

- [ ] `cmd/rename-tech-ids --generate` reads all tech YAMLs, derives `new_id = snake_case(name)`, flags PF2E-sourced names and collisions, writes `tools/rename_map.yaml`
- [ ] Human reviews and edits `tools/rename_map.yaml` (resolve flags, fix remaining PF2E names in source YAMLs)
- [ ] `cmd/rename-tech-ids --apply` reads approved map and:
  - Renames YAML files to match new IDs
  - Rewrites `id:` fields in all tech YAMLs
  - Updates all tech ID references in job/archetype YAMLs
  - Updates hardcoded tech ID string literals in Go source files
  - Emits `migrations/059_rename_tech_ids.up.sql` and `.down.sql`
  - Runs validation pass via `Registry.Load()` — exits non-zero on any error

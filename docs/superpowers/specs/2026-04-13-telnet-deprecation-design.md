# Spec: Telnet Interface Deprecation

**GitHub Issue:** cory-johannsen/mud#65
**Date:** 2026-04-13

---

## Overview

The web client (`web-client`) is now the primary player-facing interface. The telnet frontend (`internal/frontend/`) served as the original UI but is no longer needed for players. This feature formalises its deprecation: the telnet port is retained exclusively for automated testing (HeadlessClient, interactive test suite) and direct debug access via the `claude-gameserver-skill` Skill. No player-facing telnet login path is supported after this change.

---

## Requirements

### REQ-TD-1: Telnet port retention for testing and debugging

- REQ-TD-1a: The headless telnet port used by `HeadlessClient` and `seed-claude-accounts` MUST continue to function unchanged.
- REQ-TD-1b: The `claude-gameserver-skill` Skill MUST document the telnet port as the only supported direct-access method for agents and test automation.
- REQ-TD-1c: The telnet port MUST be bound to `127.0.0.1` only (loopback) in all environments — it MUST NOT be exposed externally.

### REQ-TD-2: Player-facing telnet login removal

- REQ-TD-2a: The telnet character-select and login flow MUST be removed from `internal/frontend/`.
- REQ-TD-2b: Any telnet connection on the standard telnet port that is not a headless/seeded session MUST be rejected with an informational message directing the user to the web client URL.
- REQ-TD-2c: The Helm chart / Kubernetes deployment MUST NOT expose the telnet port as a LoadBalancer or NodePort service.

### REQ-TD-3: Headless port isolation

- REQ-TD-3a: A dedicated headless-only telnet port (distinct from the deprecated player port) MUST be used for test/debug access.
- REQ-TD-3b: The headless port MUST only accept connections pre-authorized by the `seed-claude-accounts` seeding mechanism.
- REQ-TD-3c: The headless port MUST be documented in `docs/features/claude-gameserver-skill.md` and in the `claude-gameserver-skill` Skill file.

### REQ-TD-4: Documentation and dependency cleanup

- REQ-TD-4a: `docs/architecture/` diagrams that reference telnet as a player-facing interface MUST be updated to reflect web-only player access.
- REQ-TD-4b: Any player-facing references to telnet commands or telnet configuration in `docs/` MUST be removed or redirected to the web client.
- REQ-TD-4c: The `internal/frontend/` package summary comment MUST state: "Telnet frontend — retained for headless test/debug access only."

### REQ-TD-5: No regression to interactive test suite

- REQ-TD-5a: All `interactive-test-suite` tests MUST pass after this change.
- REQ-TD-5b: The `HeadlessClient` connection setup MUST require no modification beyond any port/address config changes mandated by REQ-TD-3.

---

## Files to Modify

- `internal/frontend/` — remove player login/character-select handlers; add rejection message for non-headless connections; update package doc comment
- `internal/frontend/headless.go` (or equivalent) — bind headless port to loopback only; enforce seed-authorized connections only
- `cmd/webclient/` — no changes expected
- `docs/features/claude-gameserver-skill.md` — document headless port as sole telnet access method
- `docs/architecture/` — update diagrams removing player-facing telnet
- `deploy/helm/` or `k8s/` — ensure telnet port not exposed as external service

# Telnet Interface Deprecation

**Slug:** telnet-deprecation
**Status:** backlog
**Priority:** 490
**Category:** meta
**Effort:** M

## Overview

The web client is the primary player-facing interface. This feature formalises the deprecation of the telnet frontend: the telnet port is retained exclusively for automated testing (HeadlessClient, interactive test suite) and direct debug access via the `claude-gameserver-skill` Skill. No player-facing telnet login path is supported after this change.

## Dependencies

- web-client (primary player interface must be complete)
- interactive-test-suite (headless port must remain functional)
- claude-gameserver-skill (Skill document must be updated)

## Spec

See [docs/superpowers/specs/2026-04-13-telnet-deprecation-design.md](../superpowers/specs/2026-04-13-telnet-deprecation-design.md)

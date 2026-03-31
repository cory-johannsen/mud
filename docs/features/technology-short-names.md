# Technology Short Names

**Slug:** technology-short-names
**Status:** backlog
**Priority:** 483
**Category:** ui
**Effort:** S

## Overview

Add a `short_name` field to technology definitions so the `use` command accepts a human-readable alias (e.g. `use kinetic_volley` instead of `use force_barrage_technical`), and the web UI Technologies tab populates hotbar slots with the short name rather than the raw ID.

## Dependencies

- `technology` — technology data model and `use` command
- `web-client` — Technologies drawer hotbar slot assignment

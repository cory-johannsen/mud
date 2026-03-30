# Character Switch

**Slug:** character-switch
**Status:** planned
**Priority:** 440
**Category:** ui
**Effort:** S

## Overview

Players can switch characters without fully logging out. The Logout button in the `GamePage` toolbar becomes a dropdown offering **Switch Character** (navigates to `/characters`, tears down the current session naturally) or **Logout** (clears auth, redirects to `/login`). The `CharactersPage` Logout button is unchanged.

## Dependencies

- `web-client` — GamePage toolbar and CharactersPage

## Spec

`docs/superpowers/specs/2026-03-30-character-switch-design.md`

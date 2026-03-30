# Seasonal Weather Effects

**Slug:** seasonal-weather-effects
**Status:** planned
**Priority:** 475
**Category:** world
**Effort:** L

## Overview

Random severe weather events occur during appropriate seasons, applying conditions to all outdoor players simultaneously for the duration of the event. Events are season-weighted, have random durations (2 game hours–7 game days), and are separated by mandatory cooldowns (24–72 game hours). Active weather is displayed on-screen in both the telnet and web clients. Events persist across server restarts via DB.

## Dependencies

- `persistent-calendar` — GameCalendar subscription and monotonic tick counter
- `exploration` — `applyRoomEffectsOnEntry` hook for weather condition application
- `web-client` — web UI weather badge in game toolbar

## Spec

`docs/superpowers/specs/2026-03-30-seasonal-weather-effects-design.md`

# Project Overview

## Vision

A multiplayer online text-based game (MUD) built in Go, designed with a decoupled architecture that separates the core engine from game-specific rulesets and settings. The engine serves as a general-purpose MUD platform capable of hosting different games through pluggable modules.

## Technology Stack

- OVR-1: The server MUST be implemented in Go using Go modules.
- OVR-2: The server MUST use the Pitaya game server framework as its networking and clustering foundation.
- OVR-3: The server MUST use PostgreSQL for persistent data storage.
- OVR-4: The server MUST use GopherLua as the embedded scripting runtime.
- OVR-5: The server MUST use YAML as the data definition format for world content, rulesets, and configuration.
- OVR-6: The server MUST be deployed via Docker containers.
- OVR-7: The Go toolchain MUST be managed via `mise`.

## Client Connectivity

- OVR-8: The server MUST accept client connections via Telnet.
- OVR-9: The server MUST accept client connections via WebSocket.
- OVR-10: The server MUST expose a gRPC API for programmatic client access.

## Scale

- OVR-11: The server MUST support 500 or more concurrent player connections.

## Default Game Implementation

- OVR-12: The project MUST ship with a default ruleset adapted from the Pathfinder Second Edition system.
- OVR-13: The project MUST ship with a default setting: a dystopian sci-fi future set on a post-collapse Earth.
- OVR-14: The default ruleset and setting MUST serve as a reference implementation demonstrating the engine's plugin architecture.

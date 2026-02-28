## 1. Requirements Format

- REQ-1: Requirement identifiers MUST follow the pattern "{prefix}-{ordinal}".
- REQ-2: Requirement prefixes MUST concisely indicate the concern domain.
- REQ-3: Requirement text MUST use RFC 2119 modal verbs.
- REQ-4: Requirements MUST be short, concise, and unambiguous.
- REQ-5: Bulleted lists without requirement identifiers MUST NOT be used.
- REQ-6: These meta‑requirements themselves MUST follow all requirements defined in this section.

## 2. LLM Agentic Behavior & Anti‑Patterns

- AGENT-1: Agents MUST NOT emit TODOs, placeholders, or incomplete code.
- AGENT-2: All generated code MUST be production quality and free of missing logic.
- AGENT-3: Agents MUST NOT make assumptions; all gaps MUST be surfaced explicitly.
- AGENT-4: Conflicts or ambiguities in requirements MUST be escalated and resolved prior to execution.
- AGENT-5: Agents MUST adhere to deterministic, spec‑driven behavior.
- AGENT-6: Agents MUST record timing information for all operations and display them to the user.
- AGENT-7: Agents MUST NOT allow any single operation to exceed 3 minutes in duration.
- AGENT-8: Agents MUST output the command and its progress immediately when running commands.
- AGENT-9: Agents MUST subdivide large tasks into smaller, manageable steps to prevent loss of work.
    - AGENT-9: Agents MUST record all work for each task step to allow work to resume.

## 3. Software Engineering Best Practices

- SWENG-1: Systems MUST follow the Single Responsibility Principle.
- SWENG-2: Systems MUST apply Design by Contract, with explicit preconditions and postconditions.
- SWENG-3: Systems SHOULD use a Functional Core with an Imperative Shell.
- SWENG-4: State mutation MUST be isolated at the system boundaries.
- SWENG-5: Test driven development (TDD) MUST be used for all new code and all regressions.
    - SWENG-5a: Test driven development (TDD) MUST use Property-Based Testing (https://en.wikipedia.org/wiki/Property_testing)

## 4. Golang Development Standards
- GO-1: Go development MUST use adhere to industry standard best practices and The One True Way.
- GO-2: Go development MUST use the `mise` installed toolchain.
- GO-3: Go development MUST use go modules.

## 5. Adding a New Game Command

Adding a new player command requires ALL of the following steps. Omitting any step is a defect.

- CMD-1: A `Handler<Name>` constant MUST be added to `internal/game/command/commands.go`.
- CMD-2: A `Command{...}` entry referencing the new constant MUST be appended to `BuiltinCommands()` in `internal/game/command/commands.go`.
- CMD-3: A `Handle<Name>` function MUST be implemented in `internal/game/command/<name>.go` with full TDD coverage (SWENG-5, SWENG-5a).
- CMD-4: A proto request message MUST be added to `api/proto/game/v1/game.proto` and the message MUST be added to the `ClientMessage` oneof. `make proto` MUST be run to regenerate.
- CMD-5: A `bridge<Name>` function MUST be added to `internal/frontend/handlers/bridge_handlers.go` and registered in `bridgeHandlerMap`. The test `TestAllCommandHandlersAreWired` MUST pass.
- CMD-6: A `handleName` function MUST be implemented in `internal/gameserver/grpc_service.go` and wired into the `dispatch` type switch.
- CMD-7: All steps MUST be completed and all tests MUST pass before the command is considered done. A command that is registered in `BuiltinCommands()` but not wired end-to-end is a defect.
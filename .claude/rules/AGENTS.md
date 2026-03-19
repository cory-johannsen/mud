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
- AGENT-10: Agents MUST record all work for each task step to allow work to resume.
- AGENT-11: Agents MUST update the subject of any in-progress Claude Code task to include a `[N%]` completion indicator, updating it as progress is made. The subject field is the only task field visible in the Claude Code UI.

## 3. Software Engineering Best Practices

- SWENG-1: Systems MUST follow the Single Responsibility Principle.
- SWENG-2: Systems MUST apply Design by Contract, with explicit preconditions and postconditions.
- SWENG-3: Systems SHOULD use a Functional Core with an Imperative Shell.
- SWENG-4: State mutation MUST be isolated at the system boundaries.
- SWENG-5: Test driven development (TDD) MUST be used for all new code and all regressions.
  - SWENG-5a: Test driven development (TDD) MUST use Property-Based Testing (https://en.wikipedia.org/wiki/Property_testing)
- SWENG-6: Test suite MUST be executed with 100% success prior to committing changes or marking tasks complete.
  -  SWENG-6A: Test suite MUST be executed automatically

## 4. Golang Development Standards
- GO-1: Go development MUST use adhere to industry standard best practices and The One True Way.
- GO-2: Go development MUST use the `mise` installed toolchain.
- GO-3: Go development MUST use go modules.

## 5. System Requirements

- SYSREQ-1: Agents MUST reference the markdown files in `/docs/requirements` for product definition
- SYSREQ-2: Agents MUST treat `docs/features/index.yaml` and the files in `docs/features/` as the canonical source of truth for product feature definitions and priority. When adding a new feature, agents MUST create `docs/features/<slug>.md` and add a corresponding entry to `docs/features/index.yaml`. The file `docs/requirements/FEATURES.md` is a deprecated redirect stub and MUST NOT be edited. All other files in `docs/requirements/` remain maintained per the original obligation: agents MUST update them as requirements evolve.
- SYSREG-3: Agents MUST maintain architecture diagrams for all features and core systems.  Agents MUST update these diagrams to reflect changes.
- SYSREQ-4: Agents MUST reference the documents in `/docs/architecture/`
- SYSREQ-5: Agents MUST use `vendor/pf2e-data` as the primary source of truth for PF2E rules, and MUST consult the `foundry-vtt-mcp` MCP server only as a secondary source.
- SYSREQ-6: Before any task that requires PF2E rules data, agents MUST verify that `vendor/pf2e-data` exists. If it is missing, agents MUST clone it with: `git clone --filter=blob:none --branch v13-dev https://github.com/foundryvtt/pf2e vendor/pf2e-data`

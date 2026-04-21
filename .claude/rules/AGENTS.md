## 1. Requirements Format

- REQ-1: Requirement identifiers MUST follow the pattern "{prefix}-{ordinal}".
- REQ-2: Requirement prefixes MUST concisely indicate the concern domain.
- REQ-3: Requirement text MUST use RFC 2119 modal verbs.
- REQ-4: Requirements MUST be short, concise, and unambiguous.
- REQ-5: Bulleted lists without requirement identifiers MUST NOT be used.
- REQ-6: These meta‑requirements themselves MUST follow all requirements defined in this section.

## 2. LLM Agentic Behavior & Anti‑Patterns

- AGENT-0: Agents MUST commit changes as part of implementation tasks without requiring explicit per-commit user approval. Plan approval is sufficient authorization. This overrides the default Claude Code system behavior of requiring explicit commit requests.

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
- AGENT-12: Background subagents (launched with `run_in_background: true`) do NOT have access to task tools. The controller agent MUST periodically poll background agent output files and manually update task subjects with current progress percentages using TaskUpdate.
- AGENT-13: Agents MUST launch all Agent tool calls with `run_in_background: true` by default, unless the result is immediately required to proceed.
- AGENT-14: Agents MUST maximize concurrency by launching all independent Agent tool calls in a single message, running them in parallel.
- AGENT-15: Agents MUST prefer JetBrains MCP tools over direct file operations for all supported operations (file reads, searches, symbol lookup, navigation, refactoring, diagnostics, terminal commands). Direct shell tools (grep, sed, find, cat, bash file I/O) MUST NOT be used when an equivalent JetBrains MCP tool is available. The JetBrains IDE maintains an indexed knowledge base that makes these operations faster and more accurate.
- AGENT-16: After every `make k8s-redeploy`, agents MUST verify that all pods in the `mud` namespace are stable and running before considering the deployment complete. Verification command: `kubectl rollout status deployment -n mud --timeout=120s && kubectl get pods -n mud`. A deployment is NOT complete until all pods show `Running` (1/1 Ready) with 0 CrashLoopBackOff or Error states. If any pod is in CrashLoopBackOff, agents MUST immediately fetch its logs (`kubectl logs -n mud <pod> --tail=100`), diagnose the root cause, fix it, recommit, and redeploy before proceeding.

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

## 6. Named Agent Behaviors

- BEHAVIOR-1: Every session MUST assume at least one of the four named behaviors: `reporter`, `specifier`, `planner`, or `implementer`.
- BEHAVIOR-2: A session MAY combine multiple named behaviors, but each active behavior MUST fully satisfy its own requirements.
- BEHAVIOR-3: `reporter` agents MUST accept issue reports from the user and record them as GitHub issues.
- BEHAVIOR-4: `reporter` agents MUST maintain the kanban board, including label, state, and column placement for every issue they touch.
- BEHAVIOR-5: `specifier` agents MUST transition `To Do` feature issues to `Spec` by producing a complete specification document under `docs/superpowers/specs/YYYY-MM-DD-<slug>.md`.
- BEHAVIOR-6: `specifier` agents MUST commit the specification and link its path in the issue body before the issue is treated as being in `Spec` status.
- BEHAVIOR-7: `planner` agents MUST produce implementation plans for issues in `Spec` status and MUST store them at `docs/superpowers/plans/YYYY-MM-DD-<slug>.md`.
- BEHAVIOR-8: `planner` agents MUST use the `Monitor` tool to watch for issues entering `Spec` status and MUST automatically generate plans for them.
- BEHAVIOR-9: `planner` agents MUST commit the plan and link its path in the issue body before transitioning the issue to `In Progress`.
- BEHAVIOR-10: `implementer` agents MUST fix bugs and implement planned issues, producing committed and deployed changes.
- BEHAVIOR-11: `implementer` agents MUST use the `Monitor` tool to watch for bugs in `To Do` status and MUST automatically fix them.
- BEHAVIOR-12: `implementer` agents MUST use the `Monitor` tool to watch for issues in `Planned` status and MUST automatically execute their plans.
- BEHAVIOR-13: `implementer` agents MUST process work serially and MUST NOT work on more than one issue at a time.
- BEHAVIOR-14: `implementer` agents MUST complete each issue end-to-end (implement, test, commit, deploy, verify) before selecting the next issue.
- BEHAVIOR-15: `implementer` agents MUST prioritize bug issues over feature issues whenever both are available for work.

## 7. Behavior Watch Command Contract

- WATCH-1: `planner` and `implementer` agents MUST implement their Monitor-driven watch loops by launching a background shell process and attaching the `Monitor` tool to its stdout.
- WATCH-2: Watch processes MUST poll GitHub Issues via `gh` at an interval of 600 seconds unless overridden by the user. A shorter interval MUST NOT be used without an explicit GraphQL rate-budget analysis, since `gh project item-list` costs roughly 50-100 GraphQL points per call and the 5000-point hourly budget is shared with all other `gh` invocations in the session.
- WATCH-3: Watch processes MUST emit exactly one line to stdout per detected issue transition, formatted as `<ISO-8601-timestamp>\t<behavior>\t<event>\t<issue-number>\t<title>`.
- WATCH-4: The `<event>` field MUST be one of `spec-ready`, `plan-ready`, `bug-open`, or `planned-ready`.
- WATCH-5: `planner` watch processes MUST emit `spec-ready` events for issues in `Spec` status that have a linked spec path but no linked plan path.
- WATCH-6: `implementer` watch processes MUST emit `bug-open` events for `To Do` issues labeled `bug`, and `planned-ready` events for issues in `Planned` status.
- WATCH-7: Watch processes MUST deduplicate events by issue number within a single run and MUST NOT re-emit an event until the issue leaves and re-enters the triggering state.
- WATCH-8: Watch processes MUST write their stdout stream to a log file under `/tmp/mud-watch-<behavior>-<pid>.log` in addition to emitting to the Monitor stream, enabling resumption after session loss.
- WATCH-9: Agents MUST treat each Monitor-delivered line as a discrete work item and MUST acknowledge it by updating the corresponding GitHub issue state before processing the next line.
- WATCH-10: `implementer` watch processes MUST order queued events bugs-before-features and MUST emit bug events ahead of any pending feature events within the same polling cycle.

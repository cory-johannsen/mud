# Wire Dependency Injection Refactor

Replace manual dependency wiring in `GameServiceServer` and related types with Google's `wire` code generation tool.

## Requirements

- [ ] Refactor `GameServiceServer` construction to use `wire`-generated providers
- [ ] Replace all manual constructor calls and field assignments with injected dependencies
- [ ] Ensure all existing tests continue to pass after the refactor

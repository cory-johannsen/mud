package gameserver

// Integration tests for WeaponDamageType and NPC Resistances/Weaknesses wiring
// in combat_handler.go.
//
// Full integration tests for startCombatLocked require a complete server setup
// (session manager, NPC manager, combat engine, inventory registry, etc.) that
// exceeds reasonable unit-test scope for this wiring step.
//
// The core resistance/weakness logic is covered by:
//   - internal/game/combat/round_resistance_test.go  (ResolveRound applies resistances/weaknesses)
//   - internal/game/combat/resolver_damage_type_test.go (WeaponDamageType flows into AttackResult)
//   - internal/game/npc/template_test.go (Instance.Resistances/Weaknesses copied from template)
//
// TODO: integration test verifying that startCombatLocked populates
// npcCbt.Resistances, npcCbt.Weaknesses, and playerCbt.WeaponDamageType from
// live session/NPC data — core logic covered by round_resistance_test.go.

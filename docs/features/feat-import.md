# Feat Import

Import all PF2E general, skill, and adaptable miscellaneous feats into Gunchete, adapted to the setting. See `docs/superpowers/specs/2026-03-20-feat-import-design.md` for the full design spec and complete mapping table.

## Requirements

- [ ] Feat import — 211 new feats added to `content/feats.yaml`
  - REQ-FI-1: All Gap feats from the spec MUST be added to `content/feats.yaml` in the correct skill section.
  - REQ-FI-2: Each new feat MUST include a `pf2e` field matching the source PF2E feat name.
  - REQ-FI-3: Each new feat MUST have a Gunchete-flavored `name` and `description`; PF2E flavor text MUST NOT appear verbatim.
  - REQ-FI-4: Skill feats MUST include a `skill` field matching the Gunchete skill ID from the PF2E→Gunchete skill mapping.
  - REQ-FI-5: Covered feats MUST NOT be duplicated; existing feats satisfying a PF2E mapping MUST be left unchanged.
  - REQ-FI-6: All Skip feats MUST NOT be added to the system.
  - REQ-FI-7: `docs/architecture/character.md` MUST be updated with a Feat System section documenting the three-category structure, the `pf2e` field convention, `SkillFeatsForTrainedSkills` as the runtime access path, and the extension point for adding new feats.
  - REQ-FI-8: The feat registry (`internal/game/ruleset/feat.go`) MUST NOT require code changes; all new feats are pure YAML additions.
- [ ] Audit summary: 73 Covered (no changes needed), 211 Gap (new YAML entries), 10 Skipped
  - General feats: 11 covered, 35 new, 1 skipped
  - Skill feats (17 pools): 62 covered, 160 new
  - Miscellaneous/adaptable: 0 covered, 15 new, 9 skipped

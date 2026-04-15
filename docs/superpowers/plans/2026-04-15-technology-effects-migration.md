# Technology Effects Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 1,679 technology YAML files (issues #98 and #100) where descriptions contain Foundry VTT markup and/or describe mechanics not implemented in the `effects` section, by building a Python migration script that parses Foundry annotations, maps them to Gunchete equivalents, rewrites descriptions in plain English, and populates effects sections with the correct structure.

**Architecture:** A standalone Python script (`scripts/migrate_tech_effects.py`) with three phases: (1) parse Foundry markup from descriptions to extract mechanical data, (2) generate/update the effects section using that data and the existing `resolution` field, (3) rewrite descriptions as plain English. The script runs per-tradition with dry-run and apply modes. No Go code is changed — this is a content migration only.

**Tech Stack:** Python 3, PyYAML (`pip install pyyaml`), regex, pathlib. All changes are to YAML files under `content/technologies/`.

---

## Reference: Valid Field Values

**Effect types:** `damage`, `heal`, `condition`, `skill_check`, `movement`, `zone`, `summon`, `utility`, `drain`, `tremorsense`

**Save type mappings (PF2E → Gunchete):**
| PF2E | Gunchete |
|---|---|
| `fortitude` | `toughness` |
| `reflex` | `reflex` |
| `will` | `cool` |
| `flat` | `toughness` (flat check → toughness) |

**Damage types:** `acid`, `bleed`, `bludgeoning`, `cold`, `electricity`, `fire`, `force`, `mental`, `neural`, `piercing`, `poison`, `slashing`, `sonic`, `spirit`, `untyped`, `vitality`, `void`

**Resolution types:** `save`, `attack`, `none`
- `save` → populate `on_crit_success`, `on_success`, `on_failure`, `on_crit_failure`
- `attack` → populate `on_miss`, `on_hit`, `on_crit_hit`
- `none` → populate `on_apply`

**Condition ID mappings (PF2E → Gunchete condition files):**
| PF2E name | Gunchete condition_id |
|---|---|
| Blinded | `blinded` |
| Frightened | `frightened` |
| Fleeing | `fleeing` |
| Flat-footed | `flat_footed` |
| Grabbed | `grabbed` |
| Hidden | `hidden` |
| Immobilized | `immobilized` |
| Prone | `prone` |
| Restrained | `immobilized` |
| Sickened | `nausea` |
| Slowed | `slowed` |
| Stunned | `stunned` |
| Unconscious | `unconscious` |
| Dazzled | `dazzled` |
| Dying | `dying` |
| Wounded | `wounded` |

---

## File Structure

**Create:**
- `scripts/migrate_tech_effects.py` — main migration runner
- `scripts/tech_migration/parser.py` — Foundry annotation parser
- `scripts/tech_migration/mapper.py` — PF2E→Gunchete mappings
- `scripts/tech_migration/effects_builder.py` — builds/merges effects YAML structure
- `scripts/tech_migration/description_rewriter.py` — strips Foundry markup, writes plain English
- `scripts/tech_migration/__init__.py` — empty
- `scripts/tech_migration/test_parser.py` — parser unit tests
- `scripts/tech_migration/test_mapper.py` — mapper unit tests
- `scripts/tech_migration/test_effects_builder.py` — effects builder unit tests
- `scripts/tech_migration/test_description_rewriter.py` — rewriter unit tests

**Modify (content only, no Go):**
- `content/technologies/bio_synthetic/*.yaml` — 180 affected files
- `content/technologies/fanatic_doctrine/*.yaml` — 139 affected files
- `content/technologies/neural/*.yaml` — 223 affected files
- `content/technologies/technical/*.yaml` — 194 affected files

---

## Task 1: Foundry Annotation Parser

**Files:**
- Create: `scripts/tech_migration/__init__.py`
- Create: `scripts/tech_migration/parser.py`
- Create: `scripts/tech_migration/test_parser.py`

- [ ] **Step 1: Create the package stub**

```bash
mkdir -p scripts/tech_migration
touch scripts/tech_migration/__init__.py
```

- [ ] **Step 2: Write the failing parser tests**

Create `scripts/tech_migration/test_parser.py`:

```python
import pytest
from tech_migration.parser import (
    parse_damage_annotations,
    parse_condition_annotations,
    parse_check_annotations,
    ParsedDamage,
    ParsedCondition,
    ParsedCheck,
)


class TestParseDamageAnnotations:
    def test_basic_damage(self):
        desc = "deals @Damage[3d8[acid]] damage on a hit"
        result = parse_damage_annotations(desc)
        assert result == [ParsedDamage(dice="3d8", damage_type="acid", persistent=False)]

    def test_persistent_damage(self):
        desc = "plus @Damage[(floor(@item.rank/2))d6[persistent,acid]] damage"
        result = parse_damage_annotations(desc)
        assert result == [ParsedDamage(dice="1d6", damage_type="acid", persistent=True)]

    def test_persistent_fire(self):
        desc = "takes @Damage[2d8[persistent,fire]] damage"
        result = parse_damage_annotations(desc)
        assert result == [ParsedDamage(dice="2d8", damage_type="fire", persistent=True)]

    def test_plain_dice_in_description(self):
        desc = "deals 4d10 slashing damage to the target"
        result = parse_damage_annotations(desc)
        assert result == [ParsedDamage(dice="4d10", damage_type="slashing", persistent=False)]

    def test_multiple_damage_types(self):
        desc = "dealing 4d8 slashing damage and 4d8 fire damage"
        result = parse_damage_annotations(desc)
        assert ParsedDamage(dice="4d8", damage_type="slashing", persistent=False) in result
        assert ParsedDamage(dice="4d8", damage_type="fire", persistent=False) in result

    def test_no_damage(self):
        desc = "you create a distraction and hide"
        result = parse_damage_annotations(desc)
        assert result == []

    def test_floor_expression_resolves_to_base_dice(self):
        # floor(@item.rank/2) for rank 2 = 1d6; we default to 1 die when expr is complex
        desc = "@Damage[(floor(@item.rank/2))d6[acid]]"
        result = parse_damage_annotations(desc)
        assert result[0].dice == "1d6"
        assert result[0].damage_type == "acid"


class TestParseConditionAnnotations:
    def test_uuid_condition(self):
        desc = "target is @UUID[Compendium.pf2e.conditionitems.Item.Blinded] until end of turn"
        result = parse_condition_annotations(desc)
        assert result == [ParsedCondition(pf2e_name="Blinded", value=1)]

    def test_uuid_frightened_with_value(self):
        desc = "target becomes @UUID[Compendium.pf2e.conditionitems.Item.Frightened] 2"
        result = parse_condition_annotations(desc)
        assert result == [ParsedCondition(pf2e_name="Frightened", value=2)]

    def test_plain_condition_keyword(self):
        desc = "the target is grabbed and cannot move"
        result = parse_condition_annotations(desc)
        assert ParsedCondition(pf2e_name="Grabbed", value=1) in result

    def test_multiple_conditions(self):
        desc = "target is Grabbed and also Restrained"
        result = parse_condition_annotations(desc)
        names = {c.pf2e_name for c in result}
        assert "Grabbed" in names
        assert "Restrained" in names

    def test_no_conditions(self):
        desc = "deals 3d8 acid damage"
        result = parse_condition_annotations(desc)
        assert result == []


class TestParseCheckAnnotations:
    def test_basic_reflex_save(self):
        desc = "@Check[reflex|basic] DC 15"
        result = parse_check_annotations(desc)
        assert result == [ParsedCheck(save_type="reflex", dc=15, basic=True)]

    def test_fortitude_with_dc(self):
        desc = "@Check[fortitude|dc:20]"
        result = parse_check_annotations(desc)
        assert result == [ParsedCheck(save_type="fortitude", dc=20, basic=False)]

    def test_will_save(self):
        desc = "@Check[will|basic]"
        result = parse_check_annotations(desc)
        assert result == [ParsedCheck(save_type="will", dc=0, basic=True)]

    def test_flat_check(self):
        desc = "@Check[flat|dc:11]"
        result = parse_check_annotations(desc)
        assert result == [ParsedCheck(save_type="flat", dc=11, basic=False)]

    def test_no_check(self):
        desc = "deals 3d8 acid damage on a hit"
        result = parse_check_annotations(desc)
        assert result == []
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd scripts && python -m pytest tech_migration/test_parser.py -v 2>&1 | head -30
```
Expected: ModuleNotFoundError or ImportError (parser.py doesn't exist yet)

- [ ] **Step 4: Implement parser.py**

Create `scripts/tech_migration/parser.py`:

```python
"""
Foundry VTT annotation parser for technology description migration.

Extracts damage dice, conditions, and save checks from Foundry markup.
"""
import re
from dataclasses import dataclass, field
from typing import List


@dataclass
class ParsedDamage:
    dice: str
    damage_type: str
    persistent: bool = False


@dataclass
class ParsedCondition:
    pf2e_name: str
    value: int = 1


@dataclass
class ParsedCheck:
    save_type: str
    dc: int
    basic: bool = False


# Damage types recognized in plain text descriptions
_PLAIN_DAMAGE_TYPES = {
    "acid", "bleed", "bludgeoning", "cold", "electricity", "fire",
    "force", "mental", "neural", "piercing", "poison", "slashing",
    "sonic", "spirit", "untyped", "vitality", "void",
}

# Conditions recognized in plain text (case-insensitive)
_PLAIN_CONDITIONS = {
    "blinded", "frightened", "fleeing", "flat-footed", "grabbed",
    "hidden", "immobilized", "prone", "restrained", "sickened",
    "slowed", "stunned", "unconscious", "dazzled", "dying", "wounded",
}

# Pattern: @Damage[(expr)dN[flags]] or @Damage[NdN[flags]]
_DAMAGE_ANNOTATION_RE = re.compile(
    r'@Damage\[\(?([^)\]]*)\)?(d\d+)\[([^\]]+)\]\]',
    re.IGNORECASE,
)
# Pattern: plain dice like "3d8 acid" or "4d10 slashing damage"
_PLAIN_DICE_RE = re.compile(
    r'\b(\d+d\d+)\s+(' + '|'.join(_PLAIN_DAMAGE_TYPES) + r')\b',
    re.IGNORECASE,
)
# Pattern: @UUID[...Item.ConditionName]
_UUID_CONDITION_RE = re.compile(
    r'@UUID\[Compendium\.pf2e\.conditionitems\.Item\.(\w+)\](?:\s+(\d+))?',
    re.IGNORECASE,
)
# Pattern: @Check[save_type|opts]
_CHECK_RE = re.compile(
    r'@Check\[([^\]]+)\]',
    re.IGNORECASE,
)


def _resolve_dice_prefix(prefix: str, die: str) -> str:
    """Resolve a dice count expression to a concrete dice string.

    For complex expressions like 'floor(@item.rank/2)', default to 1 die.
    For plain integers, use as-is.
    """
    prefix = prefix.strip()
    if not prefix or re.search(r'[a-zA-Z@]', prefix):
        return f"1{die}"
    try:
        count = int(eval(prefix))  # only simple arithmetic reaches here
        return f"{count}{die}"
    except Exception:
        return f"1{die}"


def parse_damage_annotations(description: str) -> List[ParsedDamage]:
    """Extract all damage entries from a technology description."""
    results = []
    seen = set()

    # @Damage[...] annotations (highest confidence)
    for match in _DAMAGE_ANNOTATION_RE.finditer(description):
        prefix, die, flags = match.group(1), match.group(2), match.group(3)
        dice = _resolve_dice_prefix(prefix, die)
        flag_parts = [f.strip().lower() for f in flags.split(",")]
        persistent = "persistent" in flag_parts
        damage_type = next(
            (f for f in flag_parts if f in _PLAIN_DAMAGE_TYPES),
            "untyped",
        )
        key = (dice, damage_type, persistent)
        if key not in seen:
            seen.add(key)
            results.append(ParsedDamage(dice=dice, damage_type=damage_type, persistent=persistent))

    # Plain text dice (only if no @Damage annotations found for this type)
    annotated_types = {r.damage_type for r in results}
    for match in _PLAIN_DICE_RE.finditer(description):
        dice, damage_type = match.group(1), match.group(2).lower()
        if damage_type not in annotated_types:
            key = (dice, damage_type, False)
            if key not in seen:
                seen.add(key)
                results.append(ParsedDamage(dice=dice, damage_type=damage_type, persistent=False))

    return results


def parse_condition_annotations(description: str) -> List[ParsedCondition]:
    """Extract all condition references from a technology description."""
    results = []
    seen = set()

    # @UUID conditions (highest confidence)
    for match in _UUID_CONDITION_RE.finditer(description):
        name = match.group(1)
        value = int(match.group(2)) if match.group(2) else 1
        if name not in seen:
            seen.add(name)
            results.append(ParsedCondition(pf2e_name=name, value=value))

    # Plain text condition keywords (title case or sentence context)
    for cond in _PLAIN_CONDITIONS:
        pattern = re.compile(r'\b' + re.escape(cond) + r'\b', re.IGNORECASE)
        if pattern.search(description):
            title = cond.title().replace("-", "_")
            if title not in seen:
                seen.add(title)
                # Look for trailing number
                m = re.search(r'\b' + re.escape(cond) + r'\s+(\d+)\b', description, re.IGNORECASE)
                value = int(m.group(1)) if m else 1
                results.append(ParsedCondition(pf2e_name=title, value=value))

    return results


def parse_check_annotations(description: str) -> List[ParsedCheck]:
    """Extract save/check references from a technology description."""
    results = []
    for match in _CHECK_RE.finditer(description):
        parts = match.group(1).split("|")
        save_type = parts[0].strip().lower()
        basic = any(p.strip().lower() == "basic" for p in parts)
        dc = 0
        for part in parts:
            m = re.match(r'dc:(\d+)', part.strip(), re.IGNORECASE)
            if m:
                dc = int(m.group(1))
                break
        results.append(ParsedCheck(save_type=save_type, dc=dc, basic=basic))
    return results
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd scripts && python -m pytest tech_migration/test_parser.py -v
```
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add scripts/tech_migration/__init__.py scripts/tech_migration/parser.py scripts/tech_migration/test_parser.py
git commit -m "feat(migration): add Foundry annotation parser for tech effects migration"
```

---

## Task 2: PF2E→Gunchete Mapper

**Files:**
- Create: `scripts/tech_migration/mapper.py`
- Create: `scripts/tech_migration/test_mapper.py`

- [ ] **Step 1: Write failing mapper tests**

Create `scripts/tech_migration/test_mapper.py`:

```python
import pytest
from tech_migration.mapper import map_save_type, map_condition_id, map_damage_type


class TestMapSaveType:
    def test_fortitude_maps_to_toughness(self):
        assert map_save_type("fortitude") == "toughness"

    def test_reflex_maps_to_reflex(self):
        assert map_save_type("reflex") == "reflex"

    def test_will_maps_to_cool(self):
        assert map_save_type("will") == "cool"

    def test_flat_maps_to_toughness(self):
        assert map_save_type("flat") == "toughness"

    def test_unknown_save_returns_none(self):
        assert map_save_type("athletics") is None

    def test_case_insensitive(self):
        assert map_save_type("Fortitude") == "toughness"
        assert map_save_type("REFLEX") == "reflex"


class TestMapConditionId:
    def test_blinded(self):
        assert map_condition_id("Blinded") == "blinded"

    def test_frightened(self):
        assert map_condition_id("Frightened") == "frightened"

    def test_restrained_maps_to_immobilized(self):
        assert map_condition_id("Restrained") == "immobilized"

    def test_sickened_maps_to_nausea(self):
        assert map_condition_id("Sickened") == "nausea"

    def test_grabbed(self):
        assert map_condition_id("Grabbed") == "grabbed"

    def test_prone(self):
        assert map_condition_id("Prone") == "prone"

    def test_flat_footed(self):
        assert map_condition_id("Flat-Footed") == "flat_footed"
        assert map_condition_id("Flat_Footed") == "flat_footed"

    def test_unknown_condition_returns_none(self):
        assert map_condition_id("Enraged") is None

    def test_case_insensitive(self):
        assert map_condition_id("BLINDED") == "blinded"
        assert map_condition_id("prone") == "prone"


class TestMapDamageType:
    def test_known_types_pass_through(self):
        for t in ["acid", "fire", "slashing", "cold", "electricity"]:
            assert map_damage_type(t) == t

    def test_unknown_maps_to_untyped(self):
        assert map_damage_type("radiant") == "untyped"

    def test_case_insensitive(self):
        assert map_damage_type("ACID") == "acid"
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd scripts && python -m pytest tech_migration/test_mapper.py -v 2>&1 | head -20
```
Expected: ImportError (mapper.py doesn't exist)

- [ ] **Step 3: Implement mapper.py**

Create `scripts/tech_migration/mapper.py`:

```python
"""
PF2E → Gunchete system mappings for the tech effects migration.
"""
from typing import Optional

_SAVE_TYPE_MAP = {
    "fortitude": "toughness",
    "reflex": "reflex",
    "will": "cool",
    "flat": "toughness",
}

_CONDITION_MAP = {
    "blinded": "blinded",
    "dazzled": "dazzled",
    "dying": "dying",
    "flat-footed": "flat_footed",
    "flat_footed": "flat_footed",
    "fleeing": "fleeing",
    "frightened": "frightened",
    "grabbed": "grabbed",
    "hidden": "hidden",
    "immobilized": "immobilized",
    "prone": "prone",
    "restrained": "immobilized",
    "sickened": "nausea",
    "slowed": "slowed",
    "stunned": "stunned",
    "unconscious": "unconscious",
    "wounded": "wounded",
}

_VALID_DAMAGE_TYPES = {
    "acid", "bleed", "bludgeoning", "cold", "electricity", "fire",
    "force", "mental", "neural", "piercing", "poison", "slashing",
    "sonic", "spirit", "untyped", "vitality", "void",
}


def map_save_type(pf2e_save: str) -> Optional[str]:
    """Map a PF2E save type string to the Gunchete equivalent.

    Returns None if the save type has no mapping (e.g. skill checks).
    """
    return _SAVE_TYPE_MAP.get(pf2e_save.lower())


def map_condition_id(pf2e_condition: str) -> Optional[str]:
    """Map a PF2E condition name to a Gunchete condition_id.

    Returns None if there is no known mapping.
    """
    normalized = pf2e_condition.lower().replace("-", "_").replace(" ", "_")
    return _CONDITION_MAP.get(normalized)


def map_damage_type(damage_type: str) -> str:
    """Map a damage type to a valid Gunchete damage type.

    Returns 'untyped' for unknown types.
    """
    normalized = damage_type.lower()
    if normalized in _VALID_DAMAGE_TYPES:
        return normalized
    return "untyped"
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd scripts && python -m pytest tech_migration/test_mapper.py -v
```
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add scripts/tech_migration/mapper.py scripts/tech_migration/test_mapper.py
git commit -m "feat(migration): add PF2E→Gunchete mapper for save types, conditions, damage types"
```

---

## Task 3: Effects Builder

**Files:**
- Create: `scripts/tech_migration/effects_builder.py`
- Create: `scripts/tech_migration/test_effects_builder.py`

- [ ] **Step 1: Write failing effects builder tests**

Create `scripts/tech_migration/test_effects_builder.py`:

```python
import pytest
from tech_migration.parser import ParsedDamage, ParsedCondition, ParsedCheck
from tech_migration.effects_builder import build_effects, EffectsBuildResult


class TestBuildEffectsAttackResolution:
    """Technologies with resolution: attack get on_hit/on_crit_hit effects."""

    def test_single_damage_on_hit(self):
        result = build_effects(
            resolution="attack",
            existing_effects={},
            damages=[ParsedDamage(dice="3d8", damage_type="acid", persistent=False)],
            conditions=[],
            checks=[],
        )
        assert result.effects["on_hit"] == [{"type": "damage", "dice": "3d8", "damage_type": "acid"}]
        assert "on_crit_hit" in result.effects

    def test_crit_doubles_damage_dice(self):
        result = build_effects(
            resolution="attack",
            existing_effects={},
            damages=[ParsedDamage(dice="3d8", damage_type="acid", persistent=False)],
            conditions=[],
            checks=[],
        )
        assert result.effects["on_crit_hit"] == [{"type": "damage", "dice": "6d8", "damage_type": "acid"}]

    def test_persistent_damage_not_doubled_on_crit(self):
        result = build_effects(
            resolution="attack",
            existing_effects={},
            damages=[
                ParsedDamage(dice="3d8", damage_type="acid", persistent=False),
                ParsedDamage(dice="1d6", damage_type="acid", persistent=True),
            ],
            conditions=[],
            checks=[],
        )
        on_crit = result.effects["on_crit_hit"]
        # Regular damage doubled: 6d8
        assert {"type": "damage", "dice": "6d8", "damage_type": "acid"} in on_crit
        # Persistent damage NOT doubled: still 1d6
        persistent = [e for e in on_crit if e.get("persistent")]
        assert persistent[0]["dice"] == "1d6"

    def test_condition_on_hit(self):
        result = build_effects(
            resolution="attack",
            existing_effects={},
            damages=[],
            conditions=[ParsedCondition(pf2e_name="Grabbed", value=1)],
            checks=[],
        )
        on_hit = result.effects["on_hit"]
        assert {"type": "condition", "condition_id": "grabbed", "value": 1} in on_hit

    def test_skips_unknown_conditions(self):
        result = build_effects(
            resolution="attack",
            existing_effects={},
            damages=[],
            conditions=[ParsedCondition(pf2e_name="Enraged", value=1)],
            checks=[],
        )
        assert result.skipped_conditions == ["Enraged"]


class TestBuildEffectsSaveResolution:
    """Technologies with resolution: save get on_failure/on_crit_failure effects."""

    def test_damage_on_failure(self):
        result = build_effects(
            resolution="save",
            existing_effects={},
            damages=[ParsedDamage(dice="4d10", damage_type="slashing", persistent=False)],
            conditions=[],
            checks=[ParsedCheck(save_type="reflex", dc=18, basic=True)],
        )
        assert result.save_type == "reflex"
        assert result.save_dc == 18
        assert result.effects["on_failure"] == [
            {"type": "damage", "dice": "4d10", "damage_type": "slashing"}
        ]
        # Basic save: half on success
        assert result.effects["on_success"] == [
            {"type": "damage", "dice": "4d10", "damage_type": "slashing", "multiplier": 0.5}
        ]

    def test_non_basic_save_no_success_damage(self):
        result = build_effects(
            resolution="save",
            existing_effects={},
            damages=[ParsedDamage(dice="4d10", damage_type="slashing", persistent=False)],
            conditions=[],
            checks=[ParsedCheck(save_type="reflex", dc=18, basic=False)],
        )
        assert "on_success" not in result.effects or result.effects.get("on_success") == []

    def test_condition_on_failure(self):
        result = build_effects(
            resolution="save",
            existing_effects={},
            damages=[],
            conditions=[ParsedCondition(pf2e_name="Prone", value=1)],
            checks=[ParsedCheck(save_type="will", dc=15, basic=False)],
        )
        assert result.save_type == "cool"
        on_fail = result.effects["on_failure"]
        assert {"type": "condition", "condition_id": "prone", "value": 1} in on_fail


class TestBuildEffectsExistingPreserved:
    """Existing non-placeholder effects are not overwritten."""

    def test_existing_real_damage_not_overwritten(self):
        existing = {
            "on_hit": [{"type": "damage", "dice": "3d8", "damage_type": "fire"}]
        }
        result = build_effects(
            resolution="attack",
            existing_effects=existing,
            damages=[ParsedDamage(dice="2d6", damage_type="acid", persistent=False)],
            conditions=[],
            checks=[],
        )
        # Existing fire damage preserved; acid NOT added (already has real effects)
        assert result.effects["on_hit"] == [{"type": "damage", "dice": "3d8", "damage_type": "fire"}]

    def test_placeholder_utility_is_replaced(self):
        existing = {
            "on_apply": [{"type": "utility"}]
        }
        result = build_effects(
            resolution="attack",
            existing_effects=existing,
            damages=[ParsedDamage(dice="3d8", damage_type="acid", persistent=False)],
            conditions=[],
            checks=[],
        )
        assert "on_hit" in result.effects
        assert result.effects.get("on_apply") is None or result.effects.get("on_apply") == []
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd scripts && python -m pytest tech_migration/test_effects_builder.py -v 2>&1 | head -20
```
Expected: ImportError

- [ ] **Step 3: Implement effects_builder.py**

Create `scripts/tech_migration/effects_builder.py`:

```python
"""
Builds Gunchete TieredEffects YAML structures from parsed Foundry annotation data.
"""
import re
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Any

from tech_migration.parser import ParsedDamage, ParsedCondition, ParsedCheck
from tech_migration.mapper import map_save_type, map_condition_id


@dataclass
class EffectsBuildResult:
    effects: Dict[str, List[Dict]]
    save_type: Optional[str] = None
    save_dc: int = 0
    skipped_conditions: List[str] = field(default_factory=list)
    needs_review: bool = False
    notes: List[str] = field(default_factory=list)


def _is_placeholder(effects: Dict) -> bool:
    """Return True if the effects dict contains only utility placeholders."""
    for slot_effects in effects.values():
        for e in slot_effects:
            if e.get("type") not in ("utility", None):
                return False
    return True


def _double_dice(dice: str) -> str:
    """Double the number of dice in an expression like '3d8' → '6d8'."""
    m = re.match(r'^(\d+)(d\d+)$', dice)
    if m:
        return f"{int(m.group(1)) * 2}{m.group(2)}"
    return dice


def _make_damage_entry(dmg: ParsedDamage) -> Dict:
    entry = {"type": "damage", "dice": dmg.dice, "damage_type": dmg.damage_type}
    if dmg.persistent:
        entry["persistent"] = True
    return entry


def _make_crit_damage_entry(dmg: ParsedDamage) -> Dict:
    """For crit hits, double non-persistent damage dice."""
    if dmg.persistent:
        return _make_damage_entry(dmg)
    entry = {"type": "damage", "dice": _double_dice(dmg.dice), "damage_type": dmg.damage_type}
    return entry


def _make_half_damage_entry(dmg: ParsedDamage) -> Dict:
    """For basic saves: success = half damage."""
    return {"type": "damage", "dice": dmg.dice, "damage_type": dmg.damage_type, "multiplier": 0.5}


def build_effects(
    resolution: str,
    existing_effects: Dict,
    damages: List[ParsedDamage],
    conditions: List[ParsedCondition],
    checks: List[ParsedCheck],
) -> EffectsBuildResult:
    """Build a TieredEffects dict from parsed annotation data.

    Precondition: resolution is one of 'attack', 'save', 'none'.
    Postcondition: result.effects contains only valid Gunchete effect entries.
    If existing_effects has real (non-placeholder) effects, they are preserved unchanged.
    """
    result = EffectsBuildResult(effects={})

    # If existing effects are real (non-placeholder), preserve them
    if existing_effects and not _is_placeholder(existing_effects):
        result.effects = dict(existing_effects)
        result.notes.append("existing non-placeholder effects preserved")
        return result

    if not damages and not conditions:
        result.needs_review = True
        result.notes.append("no damage or conditions parseable from description")
        return result

    # Resolve conditions
    mapped_conditions = []
    for cond in conditions:
        cond_id = map_condition_id(cond.pf2e_name)
        if cond_id:
            mapped_conditions.append({"type": "condition", "condition_id": cond_id, "value": cond.value})
        else:
            result.skipped_conditions.append(cond.pf2e_name)
            result.needs_review = True

    # Determine save type from checks
    primary_check = checks[0] if checks else None
    if primary_check:
        gunchete_save = map_save_type(primary_check.save_type)
        if gunchete_save:
            result.save_type = gunchete_save
            result.save_dc = primary_check.dc
        else:
            result.needs_review = True
            result.notes.append(f"unmapped save type: {primary_check.save_type}")

    if resolution == "attack":
        on_hit = [_make_damage_entry(d) for d in damages] + mapped_conditions
        on_crit_hit = [_make_crit_damage_entry(d) for d in damages] + mapped_conditions
        on_miss: List[Dict] = []
        if on_hit:
            result.effects["on_hit"] = on_hit
        if on_crit_hit:
            result.effects["on_crit_hit"] = on_crit_hit
        if on_miss:
            result.effects["on_miss"] = on_miss

    elif resolution == "save":
        on_failure = [_make_damage_entry(d) for d in damages] + mapped_conditions
        on_crit_failure = [_make_crit_damage_entry(d) for d in damages] + mapped_conditions
        on_success: List[Dict] = []
        on_crit_success: List[Dict] = []

        if primary_check and primary_check.basic and damages:
            # Basic save: half damage on success
            on_success = [_make_half_damage_entry(d) for d in damages if not d.persistent]

        if on_failure:
            result.effects["on_failure"] = on_failure
        if on_crit_failure:
            result.effects["on_crit_failure"] = on_crit_failure
        if on_success:
            result.effects["on_success"] = on_success
        if on_crit_success:
            result.effects["on_crit_success"] = on_crit_success

    else:  # none / on_apply
        on_apply = [_make_damage_entry(d) for d in damages] + mapped_conditions
        if on_apply:
            result.effects["on_apply"] = on_apply

    return result
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd scripts && python -m pytest tech_migration/test_effects_builder.py -v
```
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud
git add scripts/tech_migration/effects_builder.py scripts/tech_migration/test_effects_builder.py
git commit -m "feat(migration): add effects builder for Gunchete TieredEffects generation"
```

---

## Task 4: Description Rewriter

**Files:**
- Create: `scripts/tech_migration/description_rewriter.py`
- Create: `scripts/tech_migration/test_description_rewriter.py`

- [ ] **Step 1: Write failing description rewriter tests**

Create `scripts/tech_migration/test_description_rewriter.py`:

```python
from tech_migration.description_rewriter import rewrite_description


class TestRewriteDescription:
    def test_replaces_damage_annotation(self):
        desc = "deals @Damage[3d8[acid]] damage on a hit"
        result = rewrite_description(desc)
        assert "@Damage" not in result
        assert "3d8 acid" in result

    def test_replaces_persistent_damage_annotation(self):
        desc = "plus @Damage[2d8[persistent,fire]] damage"
        result = rewrite_description(desc)
        assert "@Damage" not in result
        assert "2d8 persistent fire" in result

    def test_replaces_floor_expression(self):
        desc = "@Damage[(floor(@item.rank/2))d6[persistent,acid]]"
        result = rewrite_description(desc)
        assert "@Damage" not in result
        assert "1d6 persistent acid" in result

    def test_replaces_uuid_condition(self):
        desc = "target becomes @UUID[Compendium.pf2e.conditionitems.Item.Blinded]"
        result = rewrite_description(desc)
        assert "@UUID" not in result
        assert "Blinded" in result

    def test_replaces_uuid_condition_with_value(self):
        desc = "target is @UUID[Compendium.pf2e.conditionitems.Item.Frightened] 2"
        result = rewrite_description(desc)
        assert "@UUID" not in result
        assert "Frightened 2" in result

    def test_replaces_check_annotation(self):
        desc = "make a @Check[reflex|basic] save"
        result = rewrite_description(desc)
        assert "@Check" not in result
        assert "Reflex" in result

    def test_replaces_check_with_dc(self):
        desc = "@Check[fortitude|dc:18]"
        result = rewrite_description(desc)
        assert "@Check" not in result
        assert "DC 18" in result
        assert "Toughness" in result

    def test_replaces_item_rank_reference(self):
        desc = "at spell rank @item.rank"
        result = rewrite_description(desc)
        assert "@item" not in result

    def test_strips_foundry_spell_uuid(self):
        desc = "cast @UUID[Compendium.pf2e.spells-srd.Item.Ignition]"
        result = rewrite_description(desc)
        assert "@UUID" not in result
        assert "Ignition" in result

    def test_no_change_to_clean_description(self):
        desc = "You fire a dart that deals 3d8 acid damage on a hit."
        result = rewrite_description(desc)
        assert result == desc
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd scripts && python -m pytest tech_migration/test_description_rewriter.py -v 2>&1 | head -20
```
Expected: ImportError

- [ ] **Step 3: Implement description_rewriter.py**

Create `scripts/tech_migration/description_rewriter.py`:

```python
"""
Rewrites Foundry VTT markup in technology descriptions to plain English.
Addresses GitHub issue #98.
"""
import re
from tech_migration.mapper import map_save_type

_VALID_DAMAGE_TYPES = {
    "acid", "bleed", "bludgeoning", "cold", "electricity", "fire",
    "force", "mental", "neural", "piercing", "poison", "slashing",
    "sonic", "spirit", "untyped", "vitality", "void",
}

# @Damage[(expr)dN[flags]] or @Damage[NdN[flags]]
_DAMAGE_RE = re.compile(r'@Damage\[\(?([^)\]]*)\)?(d\d+)\[([^\]]+)\]\]', re.IGNORECASE)
# @UUID[...conditionitems.Item.Name] optionally followed by a severity number
_CONDITION_UUID_RE = re.compile(
    r'@UUID\[Compendium\.pf2e\.conditionitems\.Item\.(\w+)\](?:\s*(\d+))?', re.IGNORECASE)
# @UUID[...spells-srd.Item.Name] or any other compendium link
_GENERIC_UUID_RE = re.compile(r'@UUID\[[^\]]+?\.(\w+)\]', re.IGNORECASE)
# @Check[save_type|opts]
_CHECK_RE = re.compile(r'@Check\[([^\]]+)\]', re.IGNORECASE)
# @item.anything or @actor.anything
_ITEM_ACTOR_RE = re.compile(r'@(?:item|actor)\.\w+', re.IGNORECASE)


def _resolve_dice_prefix(prefix: str, die: str) -> str:
    prefix = prefix.strip()
    if not prefix or re.search(r'[a-zA-Z@]', prefix):
        return f"1{die}"
    try:
        return f"{int(eval(prefix))}{die}"
    except Exception:
        return f"1{die}"


def _damage_replacement(match: re.Match) -> str:
    prefix, die, flags = match.group(1), match.group(2), match.group(3)
    dice = _resolve_dice_prefix(prefix, die)
    flag_parts = [f.strip().lower() for f in flags.split(",")]
    persistent = "persistent" in flag_parts
    damage_type = next((f for f in flag_parts if f in _VALID_DAMAGE_TYPES), "")
    parts = [dice]
    if persistent:
        parts.append("persistent")
    if damage_type:
        parts.append(damage_type)
    return " ".join(parts)


def _condition_uuid_replacement(match: re.Match) -> str:
    name = match.group(1)
    value = match.group(2)
    if value:
        return f"{name} {value}"
    return name


def _check_replacement(match: re.Match) -> str:
    parts = match.group(1).split("|")
    save_type_raw = parts[0].strip().lower()
    gunchete = map_save_type(save_type_raw)
    display = gunchete.title() if gunchete else save_type_raw.title()
    basic = any(p.strip().lower() == "basic" for p in parts)
    dc = 0
    for part in parts:
        m = re.match(r'dc:(\d+)', part.strip(), re.IGNORECASE)
        if m:
            dc = int(m.group(1))
            break
    result = f"{display} save"
    if dc:
        result += f" (DC {dc})"
    if basic:
        result += " (basic)"
    return result


def rewrite_description(description: str) -> str:
    """Replace all Foundry VTT markup in a description with plain English.

    Postcondition: returned string contains no @Damage, @UUID, @Check, or @item references.
    """
    result = description
    result = _DAMAGE_RE.sub(_damage_replacement, result)
    result = _CONDITION_UUID_RE.sub(_condition_uuid_replacement, result)
    result = _GENERIC_UUID_RE.sub(lambda m: m.group(1), result)
    result = _CHECK_RE.sub(_check_replacement, result)
    result = _ITEM_ACTOR_RE.sub("", result)
    # Clean up double spaces left by removals
    result = re.sub(r'  +', ' ', result).strip()
    return result
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd scripts && python -m pytest tech_migration/test_description_rewriter.py -v
```
Expected: All tests PASS

- [ ] **Step 5: Run the full test suite**

```bash
cd scripts && python -m pytest tech_migration/ -v
```
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add scripts/tech_migration/description_rewriter.py scripts/tech_migration/test_description_rewriter.py
git commit -m "feat(migration): add description rewriter to strip Foundry VTT markup"
```

---

## Task 5: Batch Migration Runner

**Files:**
- Create: `scripts/migrate_tech_effects.py`

- [ ] **Step 1: Implement the migration runner**

Create `scripts/migrate_tech_effects.py`:

```python
#!/usr/bin/env python3
"""
Technology effects migration runner.

Addresses GitHub issues #98 (Foundry markdown in descriptions) and
#100 (missing effects implementations).

Usage:
    cd scripts
    # Dry run — show what would change, write nothing:
    python migrate_tech_effects.py --tradition bio_synthetic --dry-run

    # Apply changes:
    python migrate_tech_effects.py --tradition bio_synthetic --apply

    # All traditions:
    python migrate_tech_effects.py --all --apply

    # Generate review report only:
    python migrate_tech_effects.py --tradition technical --report
"""
import argparse
import sys
from pathlib import Path
import yaml

# Ensure scripts/ is on sys.path
sys.path.insert(0, str(Path(__file__).parent))

from tech_migration.parser import parse_damage_annotations, parse_condition_annotations, parse_check_annotations
from tech_migration.effects_builder import build_effects
from tech_migration.description_rewriter import rewrite_description

TECH_ROOT = Path(__file__).parent.parent / "content" / "technologies"
TRADITIONS = ["bio_synthetic", "fanatic_doctrine", "neural", "technical", "innate"]


def has_foundry_markup(text: str) -> bool:
    return any(marker in text for marker in ["@Damage", "@UUID", "@Check", "@item.", "@actor."])


def is_placeholder_effects(effects: dict) -> bool:
    if not effects:
        return True
    for slot_effects in effects.values():
        for e in slot_effects:
            if e.get("type") not in ("utility", None):
                return False
    return True


def process_file(path: Path, apply: bool) -> dict:
    """Process a single technology YAML file.

    Returns a report dict with keys: name, path, changed, needs_review, notes.
    """
    report = {
        "name": path.stem,
        "path": str(path.relative_to(TECH_ROOT.parent.parent)),
        "changed": False,
        "needs_review": False,
        "notes": [],
        "desc_changed": False,
        "effects_changed": False,
    }

    with open(path) as f:
        # Load preserving order
        data = yaml.safe_load(f)

    if not isinstance(data, dict):
        report["notes"].append("not a dict — skipped")
        return report

    description = data.get("description", "")
    existing_effects = data.get("effects", {}) or {}
    resolution = data.get("resolution", "none")

    # --- Description cleanup ---
    if has_foundry_markup(description):
        new_description = rewrite_description(description)
        if new_description != description:
            report["desc_changed"] = True
            report["notes"].append("description: Foundry markup stripped")
            if apply:
                data["description"] = new_description
            description = new_description  # use cleaned description for effects parsing

    # --- Effects population ---
    if is_placeholder_effects(existing_effects):
        # Use original description (before cleaning) for parsing — markup is informative
        raw_description = data.get("description", description)
        damages = parse_damage_annotations(raw_description)
        conditions = parse_condition_annotations(raw_description)
        checks = parse_check_annotations(raw_description)

        if damages or conditions or checks:
            result = build_effects(
                resolution=resolution,
                existing_effects=existing_effects,
                damages=damages,
                conditions=conditions,
                checks=checks,
            )

            if result.effects and result.effects != existing_effects:
                report["effects_changed"] = True
                report["notes"].append(f"effects: populated {list(result.effects.keys())}")
                if result.skipped_conditions:
                    report["notes"].append(f"skipped conditions (no mapping): {result.skipped_conditions}")
                if result.needs_review:
                    report["needs_review"] = True
                    report["notes"].extend(result.notes)

                if apply:
                    data["effects"] = result.effects
                    if result.save_type and not data.get("save_type"):
                        data["save_type"] = result.save_type
                    if result.save_dc and not data.get("save_dc"):
                        data["save_dc"] = result.save_dc
            elif result.needs_review:
                report["needs_review"] = True
                report["notes"].extend(result.notes)
        else:
            report["needs_review"] = True
            report["notes"].append("no parseable mechanical data in description")

    report["changed"] = report["desc_changed"] or report["effects_changed"]

    if apply and report["changed"]:
        with open(path, "w") as f:
            yaml.dump(data, f, allow_unicode=True, default_flow_style=False, sort_keys=False)

    return report


def run(traditions: list, apply: bool, report_only: bool):
    total = changed = needs_review = 0
    review_list = []

    for tradition in traditions:
        tradition_path = TECH_ROOT / tradition
        if not tradition_path.exists():
            print(f"WARNING: tradition path not found: {tradition_path}")
            continue

        files = sorted(tradition_path.glob("*.yaml"))
        tradition_changed = 0

        print(f"\n{'='*60}")
        print(f"Tradition: {tradition} ({len(files)} files)")
        print(f"{'='*60}")

        for path in files:
            report = process_file(path, apply=apply and not report_only)
            total += 1

            if report["changed"]:
                tradition_changed += 1
                changed += 1
                action = "APPLY" if apply else "DRY-RUN"
                print(f"  [{action}] {report['name']}")
                for note in report["notes"]:
                    print(f"         {note}")

            if report["needs_review"]:
                needs_review += 1
                review_list.append(report)

        print(f"  → {tradition_changed} files changed in {tradition}")

    print(f"\n{'='*60}")
    print(f"SUMMARY")
    print(f"{'='*60}")
    print(f"  Total files processed: {total}")
    print(f"  Files changed:         {changed}")
    print(f"  Files needing review:  {needs_review}")

    if review_list:
        print(f"\nFILES NEEDING MANUAL REVIEW:")
        for r in review_list:
            print(f"  {r['path']}")
            for note in r["notes"]:
                print(f"    - {note}")


def main():
    parser = argparse.ArgumentParser(description="Technology effects migration tool")
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--tradition", choices=TRADITIONS, help="Process a single tradition")
    group.add_argument("--all", action="store_true", help="Process all traditions")
    parser.add_argument("--dry-run", action="store_true", help="Show changes without writing")
    parser.add_argument("--apply", action="store_true", help="Write changes to files")
    parser.add_argument("--report", action="store_true", help="Generate review report only")
    args = parser.parse_args()

    if not args.dry_run and not args.apply and not args.report:
        parser.error("specify one of --dry-run, --apply, or --report")

    traditions = TRADITIONS if args.all else [args.tradition]
    run(traditions=traditions, apply=args.apply, report_only=args.report)


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Install dependencies**

```bash
pip install pyyaml
```

- [ ] **Step 3: Verify the script runs on a small sample (dry run)**

```bash
cd /home/cjohannsen/src/mud/scripts
python migrate_tech_effects.py --tradition innate --dry-run
```
Expected: Output showing 0 files changed (innate is clean), script completes without errors.

- [ ] **Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud
git add scripts/migrate_tech_effects.py
git commit -m "feat(migration): add batch tech effects migration runner (dry-run + apply modes)"
```

---

## Task 6: Run Migration — bio_synthetic (180 affected files)

- [ ] **Step 1: Dry run — review proposed changes**

```bash
cd /home/cjohannsen/src/mud/scripts
python migrate_tech_effects.py --tradition bio_synthetic --dry-run 2>&1 | tee /tmp/bio_synthetic_dryrun.txt
```
Review output. Check that:
- Damage dice match description text
- Condition IDs are recognized Gunchete conditions
- Save types are `toughness`, `reflex`, or `cool` (not PF2E names)
- Files marked `needs_review` are noted but not failing

- [ ] **Step 2: Apply changes**

```bash
python migrate_tech_effects.py --tradition bio_synthetic --apply
```

- [ ] **Step 3: Validate YAML parses correctly**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/technology/... -run TestRegistry -v 2>&1 | tail -20
```
Expected: PASS — registry loads all bio_synthetic files without parse errors.

- [ ] **Step 4: Run full Go test suite**

```bash
cd /home/cjohannsen/src/mud
go test ./... 2>&1 | tail -30
```
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add content/technologies/bio_synthetic/
git commit -m "fix(content): populate tech effects and clean descriptions in bio_synthetic tradition (#98 #100)"
```

---

## Task 7: Run Migration — fanatic_doctrine (139 affected files)

- [ ] **Step 1: Dry run**

```bash
cd /home/cjohannsen/src/mud/scripts
python migrate_tech_effects.py --tradition fanatic_doctrine --dry-run 2>&1 | tee /tmp/fanatic_dryrun.txt
```

- [ ] **Step 2: Apply**

```bash
python migrate_tech_effects.py --tradition fanatic_doctrine --apply
```

- [ ] **Step 3: Validate YAML parses**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/technology/... -v 2>&1 | tail -20
```

- [ ] **Step 4: Run full test suite**

```bash
go test ./... 2>&1 | tail -30
```

- [ ] **Step 5: Commit**

```bash
git add content/technologies/fanatic_doctrine/
git commit -m "fix(content): populate tech effects and clean descriptions in fanatic_doctrine tradition (#98 #100)"
```

---

## Task 8: Run Migration — neural (223 affected files)

- [ ] **Step 1: Dry run**

```bash
cd /home/cjohannsen/src/mud/scripts
python migrate_tech_effects.py --tradition neural --dry-run 2>&1 | tee /tmp/neural_dryrun.txt
```

- [ ] **Step 2: Apply**

```bash
python migrate_tech_effects.py --tradition neural --apply
```

- [ ] **Step 3: Validate YAML parses**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/technology/... -v 2>&1 | tail -20
```

- [ ] **Step 4: Run full test suite**

```bash
go test ./... 2>&1 | tail -30
```

- [ ] **Step 5: Commit**

```bash
git add content/technologies/neural/
git commit -m "fix(content): populate tech effects and clean descriptions in neural tradition (#98 #100)"
```

---

## Task 9: Run Migration — technical (194 affected files)

- [ ] **Step 1: Dry run**

```bash
cd /home/cjohannsen/src/mud/scripts
python migrate_tech_effects.py --tradition technical --dry-run 2>&1 | tee /tmp/technical_dryrun.txt
```

- [ ] **Step 2: Apply**

```bash
python migrate_tech_effects.py --tradition technical --apply
```

- [ ] **Step 3: Validate YAML parses**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/technology/... -v 2>&1 | tail -20
```

- [ ] **Step 4: Run full test suite**

```bash
go test ./... 2>&1 | tail -30
```

- [ ] **Step 5: Commit**

```bash
git add content/technologies/technical/
git commit -m "fix(content): populate tech effects and clean descriptions in technical tradition (#98 #100)"
```

---

## Task 10: Manual Review Pass

After the automated migration, the runner will have produced a list of files marked `needs_review`. These require human judgment — descriptions where the mechanical intent is unclear or conditions couldn't be mapped.

- [ ] **Step 1: Generate the full review report**

```bash
cd /home/cjohannsen/src/mud/scripts
python migrate_tech_effects.py --all --report 2>&1 | tee /tmp/needs_review.txt
cat /tmp/needs_review.txt | grep "MANUAL REVIEW" -A 1000
```

- [ ] **Step 2: For each flagged file, open it and manually add effects**

For each file listed in the review output, examine the description and add the appropriate effects following the schema in the Reference section above. Example structure for a save-based tech with conditions:

```yaml
resolution: save
save_type: toughness
save_dc: 18
effects:
  on_failure:
    - type: condition
      condition_id: stunned
      value: 1
  on_crit_failure:
    - type: condition
      condition_id: stunned
      value: 2
    - type: condition
      condition_id: prone
      value: 1
  on_success:
    - type: utility
```

- [ ] **Step 3: Validate all files parse**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/technology/... -v 2>&1 | tail -30
```

- [ ] **Step 4: Run full test suite**

```bash
go test ./... 2>&1 | tail -30
```
Expected: All PASS

- [ ] **Step 5: Commit manual fixes**

```bash
git add content/technologies/
git commit -m "fix(content): manually fix remaining tech effects that required human review (#98 #100)"
```

- [ ] **Step 6: Update GitHub issues**

```bash
gh issue close 98 --comment "Fixed: all Foundry VTT markup stripped from technology descriptions by automated migration script."
gh issue close 100 --comment "Fixed: effects sections populated for all 1,679 affected technology files via automated migration + manual review pass."
```

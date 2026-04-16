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

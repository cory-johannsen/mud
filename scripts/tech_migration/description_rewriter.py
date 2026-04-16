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

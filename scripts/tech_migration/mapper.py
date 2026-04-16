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

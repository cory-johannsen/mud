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

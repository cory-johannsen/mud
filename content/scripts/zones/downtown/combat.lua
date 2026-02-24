-- Combat hooks for downtown zone.
-- Return nil to use original values; return false from on_condition_apply to cancel.

function on_attack_roll(attacker_uid, target_uid, roll_total, ac)
    return nil
end

function on_damage_roll(attacker_uid, target_uid, damage)
    return nil
end

function on_condition_apply(target_uid, condition_id, stacks)
    return nil
end

-- Clear archetype ability boost choices for characters whose job moved to a
-- different archetype. These choices were made under the old archetype's boost
-- pool and are now invalid. Players will be re-prompted at next login.
DELETE FROM character_ability_boosts
WHERE source = 'archetype'
AND character_id IN (
    SELECT id FROM characters
    WHERE class IN (
        'pastor', 'believer', 'street_preacher', 'trainee', 'guard', 'follower', 'vigilante',
        'hippie', 'freegan', 'tracker', 'rancher', 'hobo', 'laborer', 'fallen_trustafarian',
        'narcomancer', 'illusionist', 'grifter', 'dealer', 'mall_ninja', 'shit_stirrer',
        'goon', 'muscle', 'pirate', 'free_spirit', 'contract_killer', 'specialist',
        'cult_leader', 'hired_help', 'medic', 'maker', 'salesman', 'exterminator',
        'driver', 'pilot', 'hanger_on'
    )
);

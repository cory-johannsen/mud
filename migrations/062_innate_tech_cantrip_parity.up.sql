-- REQ-ITC-1: Remove innate tech rows for non-tech-capable characters.
-- Aggressor archetype jobs: beat_down_artist, boot_gun, boot_machete, gangster, goon, grunt,
--   mercenary, muscle, roid_rager, soldier, street_fighter, thug
-- Criminal archetype jobs: beggar, car_jacker, contract_killer, gambler, hanger_on, hooker,
--   smuggler, thief, tomb_raider
DELETE FROM character_innate_technologies
WHERE character_id IN (
    SELECT id FROM characters
    WHERE class IN (
        'beat_down_artist', 'boot_gun', 'boot_machete', 'gangster', 'goon', 'grunt',
        'mercenary', 'muscle', 'roid_rager', 'soldier', 'street_fighter', 'thug',
        'beggar', 'car_jacker', 'contract_killer', 'gambler', 'hanger_on', 'hooker',
        'smuggler', 'thief', 'tomb_raider'
    )
);

-- REQ-ITC-2: Set all remaining innate tech rows to unlimited.
-- MaxUses = 0 means unlimited per session.InnateSlot convention.
UPDATE character_innate_technologies
SET max_uses = 0, uses_remaining = 0;

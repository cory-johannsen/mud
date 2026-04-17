-- REQ-TIT-4: Insert missing tradition innate techs for all existing tech-capable characters.
-- ON CONFLICT DO NOTHING makes each INSERT idempotent — safe to run multiple times.
-- Job class names are the canonical IDs stored in characters.class.

-- technical tradition: nerd jobs
INSERT INTO character_innate_technologies (character_id, tech_id, max_uses, uses_remaining)
SELECT c.id, t.tech_id, 0, 0
FROM characters c
CROSS JOIN (VALUES
    ('atmospheric_surge'), ('blackout_pulse'), ('seismic_sense'),
    ('arc_lights'), ('pressure_burst')
) AS t(tech_id)
WHERE c.class IN (
    'natural_mystic', 'specialist', 'detective', 'journalist', 'hoarder',
    'grease_monkey', 'narc', 'engineer', 'cooker'
)
ON CONFLICT (character_id, tech_id) DO NOTHING;

-- bio_synthetic tradition: naturalist and drifter jobs
INSERT INTO character_innate_technologies (character_id, tech_id, max_uses, uses_remaining)
SELECT c.id, t.tech_id, 0, 0
FROM characters c
CROSS JOIN (VALUES
    ('moisture_reclaim'), ('viscous_spray'), ('nanite_infusion'), ('acid_spit')
) AS t(tech_id)
WHERE c.class IN (
    'rancher', 'hippie', 'laborer', 'hobo', 'tracker', 'freegan',
    'exterminator', 'fallen_trustafarian',
    'scout', 'cop', 'psychopath', 'driver', 'bagman', 'pilot',
    'warden', 'stalker', 'pirate', 'free_spirit'
)
ON CONFLICT (character_id, tech_id) DO NOTHING;

-- neural tradition: schemer and influencer jobs
INSERT INTO character_innate_technologies (character_id, tech_id, max_uses, uses_remaining)
SELECT c.id, t.tech_id, 0, 0
FROM characters c
CROSS JOIN (VALUES
    ('terror_broadcast'), ('chrome_reflex'), ('neural_flare'),
    ('static_veil'), ('synapse_tap')
) AS t(tech_id)
WHERE c.class IN (
    'narcomancer', 'maker', 'grifter', 'dealer', 'shit_stirrer',
    'salesman', 'mall_ninja', 'illusionist',
    'karen', 'politician', 'libertarian', 'entertainer', 'antifa',
    'bureaucrat', 'exotic_dancer', 'schmoozer', 'extortionist', 'anarchist'
)
ON CONFLICT (character_id, tech_id) DO NOTHING;

-- fanatic_doctrine tradition: zealot jobs
INSERT INTO character_innate_technologies (character_id, tech_id, max_uses, uses_remaining)
SELECT c.id, t.tech_id, 0, 0
FROM characters c
CROSS JOIN (VALUES
    ('doctrine_ward'), ('martyrs_resolve'), ('righteous_condemnation'),
    ('fervor_pulse'), ('litany_of_iron')
) AS t(tech_id)
WHERE c.class IN (
    'cult_leader', 'street_preacher', 'medic', 'guard', 'believer',
    'hired_help', 'vigilante', 'follower', 'trainee', 'pastor'
)
ON CONFLICT (character_id, tech_id) DO NOTHING;

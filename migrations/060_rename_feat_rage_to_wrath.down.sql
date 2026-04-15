-- Revert: rename 'wrath' back to 'rage' in character_feats.
UPDATE character_feats SET feat_id = 'rage' WHERE feat_id = 'wrath';

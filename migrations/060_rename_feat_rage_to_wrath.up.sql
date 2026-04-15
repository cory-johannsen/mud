-- REQ-80: rename Boot class feat 'rage' to 'wrath' in character_feats.
UPDATE character_feats SET feat_id = 'wrath' WHERE feat_id = 'rage';

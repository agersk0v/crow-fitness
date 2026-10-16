ALTER TABLE session_sets ADD COLUMN technique text NOT NULL DEFAULT 'decent'
    CHECK (technique IN ('good', 'decent', 'bad'));

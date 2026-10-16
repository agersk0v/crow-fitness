ALTER TABLE session_sets ADD COLUMN rir numeric
    CHECK (rir >= 0 AND rir <> 'NaN'::numeric AND rir <> 'Infinity'::numeric);

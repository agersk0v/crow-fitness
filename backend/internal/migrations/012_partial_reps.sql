ALTER TABLE session_sets ADD COLUMN partial_reps bigint
    CHECK (partial_reps > 0 AND partial_reps <= 9007199254740991);

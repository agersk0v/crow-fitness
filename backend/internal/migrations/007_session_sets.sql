CREATE TABLE session_sets (
    session_id bigint NOT NULL,
    owner_id bigint NOT NULL,
    id uuid NOT NULL,
    exercise_position bigint NOT NULL,
    weight numeric NOT NULL CHECK (weight >= 0 AND weight <> 'NaN'::numeric AND weight <> 'Infinity'::numeric AND scale(weight) <= 2),
    reps bigint NOT NULL CHECK (reps > 0),
    saved_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    deleted boolean NOT NULL DEFAULT false,
    PRIMARY KEY (session_id, id),
    FOREIGN KEY (owner_id, session_id) REFERENCES workout_sessions(owner_id, id) ON DELETE CASCADE,
    FOREIGN KEY (session_id, exercise_position) REFERENCES session_exercises(session_id, position)
);
-- Deleted rows prevent late save retries from recreating a removed set.
CREATE INDEX session_sets_owner_session ON session_sets(owner_id, session_id);

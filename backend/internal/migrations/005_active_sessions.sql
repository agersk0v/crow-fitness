-- Only active workouts are retained in this increment; ending deletes the snapshot.
CREATE TABLE active_sessions (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_id bigint NOT NULL UNIQUE REFERENCES owners(id),
    template_id bigint NOT NULL,
    name text NOT NULL CHECK (length(btrim(name)) > 0),
    started_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (owner_id, id),
    FOREIGN KEY (owner_id, template_id) REFERENCES templates(owner_id, id)
);
CREATE TABLE active_session_exercises (
    session_id bigint NOT NULL,
    owner_id bigint NOT NULL,
    position bigint NOT NULL CHECK (position > 0),
    -- Captured identity/name survive removal from a template or the exercise library.
    exercise_id bigint NOT NULL CHECK (exercise_id > 0),
    name text NOT NULL CHECK (length(btrim(name)) > 0),
    set_count bigint NOT NULL CHECK (set_count > 0),
    rep_min bigint,
    rep_max bigint,
    target_rir numeric CHECK (target_rir >= 0 AND target_rir <> 'NaN'::numeric AND target_rir <> 'Infinity'::numeric),
    PRIMARY KEY (session_id, position),
    FOREIGN KEY (owner_id, session_id) REFERENCES active_sessions(owner_id, id) ON DELETE CASCADE,
    CHECK ((rep_min IS NULL AND rep_max IS NULL) OR
           (rep_min IS NOT NULL AND rep_max IS NOT NULL AND rep_min > 0 AND rep_max >= rep_min))
);

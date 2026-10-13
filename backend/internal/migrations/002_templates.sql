-- Composite references keep a template and every selected exercise under one owner.
ALTER TABLE exercises ADD CONSTRAINT exercises_owner_id_id_unique UNIQUE (owner_id, id);
CREATE TABLE templates (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_id bigint NOT NULL REFERENCES owners(id),
    name text NOT NULL CHECK (length(btrim(name)) > 0),
    UNIQUE (owner_id, id)
);
CREATE TABLE template_exercises (
    template_id bigint NOT NULL,
    owner_id bigint NOT NULL,
    position bigint NOT NULL CHECK (position > 0),
    exercise_id bigint NOT NULL,
    set_count bigint NOT NULL CHECK (set_count > 0),
    rep_min bigint,
    rep_max bigint,
    target_rir numeric CHECK (target_rir >= 0 AND target_rir <> 'NaN'::numeric AND target_rir <> 'Infinity'::numeric),
    PRIMARY KEY (template_id, position),
    FOREIGN KEY (owner_id, template_id) REFERENCES templates(owner_id, id),
    FOREIGN KEY (owner_id, exercise_id) REFERENCES exercises(owner_id, id),
    CHECK ((rep_min IS NULL AND rep_max IS NULL) OR
           (rep_min IS NOT NULL AND rep_max IS NOT NULL AND rep_min > 0 AND rep_max >= rep_min))
);

CREATE TABLE template_exercise_alternatives (
 template_id bigint NOT NULL,
 owner_id bigint NOT NULL,
 position bigint NOT NULL,
 choice_position bigint NOT NULL CHECK(choice_position>0),
 exercise_id bigint NOT NULL,
 set_count bigint NOT NULL CHECK(set_count>0),
 rep_min bigint,
 rep_max bigint,
 target_rir numeric CHECK(target_rir>=0 AND target_rir<>'NaN'::numeric AND target_rir<>'Infinity'::numeric),
 PRIMARY KEY(template_id,position,choice_position),
 UNIQUE(template_id,position,exercise_id),
 FOREIGN KEY(template_id,position) REFERENCES template_exercises(template_id,position) ON DELETE CASCADE,
 FOREIGN KEY(owner_id,template_id) REFERENCES templates(owner_id,id),
 FOREIGN KEY(owner_id,exercise_id) REFERENCES exercises(owner_id,id),
 CHECK((rep_min IS NULL AND rep_max IS NULL) OR (rep_min IS NOT NULL AND rep_max IS NOT NULL AND rep_min>0 AND rep_max>=rep_min))
);
ALTER TABLE session_exercises ADD COLUMN selection_revision bigint NOT NULL DEFAULT 0 CHECK(selection_revision>=0);
CREATE TABLE session_exercise_choices (
 session_id bigint NOT NULL,
 owner_id bigint NOT NULL,
 position bigint NOT NULL,
 choice_position bigint NOT NULL CHECK(choice_position>=0),
 exercise_id bigint NOT NULL CHECK(exercise_id>0),
 name text NOT NULL CHECK(length(btrim(name))>0),
 set_count bigint NOT NULL CHECK(set_count>0),
 rep_min bigint,
 rep_max bigint,
 target_rir numeric CHECK(target_rir>=0 AND target_rir<>'NaN'::numeric AND target_rir<>'Infinity'::numeric),
 PRIMARY KEY(session_id,position,choice_position),
 UNIQUE(session_id,position,exercise_id),
 FOREIGN KEY(session_id,position) REFERENCES session_exercises(session_id,position) ON DELETE CASCADE,
 FOREIGN KEY(owner_id,session_id) REFERENCES workout_sessions(owner_id,id) ON DELETE CASCADE,
 CHECK((rep_min IS NULL AND rep_max IS NULL) OR (rep_min IS NOT NULL AND rep_max IS NOT NULL AND rep_min>0 AND rep_max>=rep_min))
);
-- Existing snapshots become single-choice slots, without touching recorded sets.
INSERT INTO session_exercise_choices(session_id,owner_id,position,choice_position,exercise_id,name,set_count,rep_min,rep_max,target_rir)
 SELECT session_id,owner_id,position,0,exercise_id,name,set_count,rep_min,rep_max,target_rir FROM session_exercises;

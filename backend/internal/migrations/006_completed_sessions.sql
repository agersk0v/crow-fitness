-- Retain session snapshots on completion while preserving existing active workouts.
ALTER TABLE active_sessions RENAME TO workout_sessions;
ALTER TABLE active_session_exercises RENAME TO session_exercises;
ALTER TABLE workout_sessions ADD COLUMN completed_at timestamptz;
ALTER TABLE workout_sessions ADD CONSTRAINT session_completion_time CHECK (completed_at IS NULL OR completed_at >= started_at);
ALTER TABLE workout_sessions DROP CONSTRAINT active_sessions_owner_id_key;
CREATE UNIQUE INDEX one_active_session_per_owner ON workout_sessions(owner_id) WHERE completed_at IS NULL;

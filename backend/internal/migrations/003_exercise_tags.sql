ALTER TABLE exercises ADD COLUMN muscle_groups text[] NOT NULL DEFAULT '{}';
ALTER TABLE exercises ADD CONSTRAINT exercises_muscle_groups_valid CHECK (
    muscle_groups <@ ARRAY['chest','back','shoulders','biceps','triceps','forearms','core','glutes','quadriceps','hamstrings','calves']::text[]
    AND array_position(muscle_groups, NULL) IS NULL
);

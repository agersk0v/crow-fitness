ALTER TABLE session_sets ADD COLUMN set_order bigint NOT NULL DEFAULT 0;
WITH numbered AS (
    SELECT session_id,id,row_number() OVER (
        PARTITION BY session_id,exercise_position ORDER BY saved_at,id
    ) AS position FROM session_sets
)
UPDATE session_sets s SET set_order=n.position
FROM numbered n WHERE n.session_id=s.session_id AND n.id=s.id;

package sessions

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Number occurrences before joining sets, so empty repeated occurrences do not
// shift the correspondence. Compare within the previous workout, not the
// currently filtered results; a previous workout with no sets has no baseline.
const completedSetComparisons = `WITH occurrences AS (
 SELECT e.*,s.completed_at,
 row_number() OVER (PARTITION BY e.session_id,e.exercise_id ORDER BY e.position) AS occurrence_number
 FROM session_exercises e JOIN workout_sessions s ON s.owner_id=e.owner_id AND s.id=e.session_id
 WHERE e.owner_id=$1 AND s.completed_at IS NOT NULL
), numbered_sets AS (
 SELECT ss.*,e.exercise_id,e.completed_at,e.occurrence_number,
 row_number() OVER (PARTITION BY ss.session_id,ss.exercise_position ORDER BY ss.set_order,ss.saved_at,ss.id) AS set_number
 FROM session_sets ss JOIN occurrences e ON e.owner_id=ss.owner_id AND e.session_id=ss.session_id AND e.position=ss.exercise_position
 WHERE NOT ss.deleted
), compared_sets AS (
 SELECT current.*,(current.weight-previous.weight)::text AS weight_change,
 (current.reps-previous.reps)::text AS reps_change
 FROM numbered_sets current
 LEFT JOIN LATERAL (
 SELECT s.id FROM workout_sessions s
 WHERE s.owner_id=$1 AND s.completed_at IS NOT NULL
 AND (s.completed_at,s.id)<(current.completed_at,current.session_id)
 AND EXISTS(SELECT 1 FROM session_exercises e WHERE e.owner_id=s.owner_id AND e.session_id=s.id AND e.exercise_id=current.exercise_id)
 ORDER BY s.completed_at DESC,s.id DESC LIMIT 1
 ) source ON true
 LEFT JOIN numbered_sets previous ON previous.session_id=source.id
 AND previous.exercise_id=current.exercise_id AND previous.occurrence_number=current.occurrence_number
 AND previous.set_number=current.set_number
) `

type SetChange struct {
	Weight string `json:"weight"`
	Reps   string `json:"reps"`
}

func setChange(weight, reps *string) *SetChange {
	if weight == nil || reps == nil {
		return nil
	}
	return &SetChange{Weight: *weight, Reps: *reps}
}

func readCompletedSets(ctx context.Context, tx pgx.Tx, owner int64, id string) ([]SavedSet, error) {
	rows, err := tx.Query(ctx, completedSetComparisons+`
 SELECT id::text,exercise_position,weight::text,reps,weight_change,reps_change,rir::text
 FROM compared_sets WHERE session_id=$2 ORDER BY exercise_position,set_order,saved_at,id`, owner, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SavedSet{}
	for rows.Next() {
		var item SavedSet
		var weight, reps *string
		if err = rows.Scan(&item.ID, &item.Position, &item.Weight, &item.Reps, &weight, &reps, &item.RIR); err != nil {
			return nil, err
		}
		item.Change = setChange(weight, reps)
		items = append(items, item)
	}
	return items, rows.Err()
}

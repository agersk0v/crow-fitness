package sessions

import (
	"context"
	"time"
)

// Reps and weight remain exact strings; chart coordinates are presentation only.
// Partials and load never influence the rep record or tie-breaking.
type ProgressPoint struct {
	SessionID    string    `json:"session_id"`
	SessionName  string    `json:"session_name"`
	CompletedAt  time.Time `json:"completed_at"`
	ExerciseName string    `json:"exercise_name"`
	Position     int64     `json:"exercise_position"`
	SetNumber    int64     `json:"set_number"`
	SetID        string    `json:"set_id"`
	Weight       string    `json:"weight"`
	Reps         string    `json:"reps"`
	RIR          *string   `json:"rir"`
	Technique    string    `json:"technique"`
	PartialReps  *int64    `json:"partial_reps"`
}

func (p Postgres) ExerciseProgress(ctx context.Context, owner int64, filter HistoryFilter) ([]ProgressPoint, error) {
	if !validID(filter.Exercise) {
		return nil, ValidationError{"exercise": "Choose one exercise for progress."}
	}
	rows, err := p.Pool.Query(ctx, `WITH matching AS (
 SELECT ss.*,s.name AS session_name,s.completed_at,se.name AS exercise_name,
 row_number() OVER(PARTITION BY ss.session_id,ss.exercise_position ORDER BY ss.set_order,ss.saved_at,ss.id) AS set_number
 FROM session_sets ss JOIN workout_sessions s ON s.owner_id=ss.owner_id AND s.id=ss.session_id
 JOIN session_exercises se ON se.owner_id=ss.owner_id AND se.session_id=ss.session_id AND se.position=ss.exercise_position
 WHERE ss.owner_id=$1 AND s.completed_at IS NOT NULL AND NOT ss.deleted
 `+completedSetFilter+`
 ), best AS (
 SELECT DISTINCT ON(session_id) * FROM matching
 ORDER BY session_id,reps DESC,array_position(ARRAY['bad','decent','good'],technique) DESC,exercise_position,set_order,saved_at,id
 )
 SELECT session_id::text,session_name,completed_at,exercise_name,exercise_position,set_number,id::text,weight::text,reps::text,rir::text,technique,partial_reps FROM best
 ORDER BY completed_at,best.session_id`, owner, filter.Query, filter.Exercise, filter.From, filter.Until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProgressPoint{}
	for rows.Next() {
		var item ProgressPoint
		if err = rows.Scan(&item.SessionID, &item.SessionName, &item.CompletedAt, &item.ExerciseName, &item.Position, &item.SetNumber, &item.SetID, &item.Weight, &item.Reps, &item.RIR, &item.Technique, &item.PartialReps); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

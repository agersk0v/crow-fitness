package sessions

import (
	"context"
	"errors"
	"math/big"
	"time"
)

// Exact source values stay separate from display-rounded estimates.
type ProgressSet struct {
	Position    int64   `json:"exercise_position"`
	SetNumber   int64   `json:"set_number"`
	SetID       string  `json:"set_id"`
	Weight      string  `json:"weight"`
	Reps        string  `json:"reps"`
	RIR         *string `json:"rir"`
	Technique   string  `json:"technique"`
	PartialReps *int64  `json:"partial_reps"`
	Estimate    string  `json:"estimate"`
}
type ProgressPoint struct {
	SessionID       string        `json:"session_id"`
	SessionName     string        `json:"session_name"`
	CompletedAt     time.Time     `json:"completed_at"`
	ExerciseName    string        `json:"exercise_name"`
	AverageEstimate string        `json:"average_estimate"`
	SetCount        int           `json:"set_count"`
	Sets            []ProgressSet `json:"sets"`
}

// All entered weights are treated as resistance, including zero. Technique and
// partial reps are contextual. Round only after averaging the exact estimates.
func averageEstimates(sets []ProgressSet) (string, error) {
	if len(sets) == 0 {
		return "", errors.New("progress requires saved sets")
	}
	sum := new(big.Rat)
	for i := range sets {
		weight, ok := new(big.Rat).SetString(sets[i].Weight)
		if !ok {
			return "", errors.New("invalid saved weight")
		}
		reps, ok := new(big.Int).SetString(sets[i].Reps, 10)
		if !ok {
			return "", errors.New("invalid saved reps")
		}
		factor := new(big.Rat).SetFrac(new(big.Int).Add(reps, big.NewInt(30)), big.NewInt(30))
		estimate := new(big.Rat).Mul(weight, factor)
		sum.Add(sum, estimate)
		sets[i].Estimate = estimate.FloatString(2)
	}
	return sum.Quo(sum, new(big.Rat).SetInt64(int64(len(sets)))).FloatString(2), nil
}

func (p Postgres) ExerciseProgress(ctx context.Context, owner int64, filter HistoryFilter) ([]ProgressPoint, error) {
	if !validID(filter.Exercise) {
		return nil, ValidationError{"exercise": "Choose one exercise for progress."}
	}
	rows, err := p.Pool.Query(ctx, `SELECT s.id::text,s.name,s.completed_at,se.name,
 ss.exercise_position,row_number() OVER(PARTITION BY ss.session_id,ss.exercise_position ORDER BY ss.set_order,ss.saved_at,ss.id),ss.id::text,ss.weight::text,ss.reps::text,ss.rir::text,ss.technique,ss.partial_reps
 FROM session_sets ss JOIN workout_sessions s ON s.owner_id=ss.owner_id AND s.id=ss.session_id
 JOIN session_exercises se ON se.owner_id=ss.owner_id AND se.session_id=ss.session_id AND se.position=ss.exercise_position
 WHERE ss.owner_id=$1 AND s.completed_at IS NOT NULL AND NOT ss.deleted
 `+completedSetFilter+`
 ORDER BY s.completed_at,s.id,ss.exercise_position,ss.set_order,ss.saved_at,ss.id`, owner, filter.Query, filter.Exercise, filter.From, filter.Until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProgressPoint{}
	for rows.Next() {
		var p ProgressPoint
		var set ProgressSet
		if err = rows.Scan(&p.SessionID, &p.SessionName, &p.CompletedAt, &p.ExerciseName, &set.Position, &set.SetNumber, &set.SetID, &set.Weight, &set.Reps, &set.RIR, &set.Technique, &set.PartialReps); err != nil {
			return nil, err
		}
		if len(items) == 0 || items[len(items)-1].SessionID != p.SessionID {
			items = append(items, p)
		}
		item := &items[len(items)-1]
		item.Sets = append(item.Sets, set)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].SetCount = len(items[i].Sets)
		items[i].AverageEstimate, err = averageEstimates(items[i].Sets)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

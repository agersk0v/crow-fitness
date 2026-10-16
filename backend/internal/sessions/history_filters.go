package sessions

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
	_ "time/tzdata"
)

type HistoryFilter struct {
	Query    string
	Exercise string
	Template string
	From     *time.Time
	Until    *time.Time
}

func ParseHistoryQuery(raw string) (HistoryFilter, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return HistoryFilter{}, errors.New("Use valid URL query parameters.")
	}
	return ParseHistoryFilter(values)
}

// Calendar boundaries use the caller's IANA timezone, including DST changes.
func ParseHistoryFilter(values url.Values) (HistoryFilter, error) {
	f := HistoryFilter{Query: strings.TrimSpace(values.Get("q")), Exercise: values.Get("exercise"), Template: values.Get("template")}
	for key, entries := range values {
		if key != "q" && key != "exercise" && key != "template" && key != "from" && key != "to" && key != "tz" {
			return f, errors.New("Unknown history filter.")
		}
		if len(entries) != 1 {
			return f, errors.New("Use each history filter once.")
		}
	}
	if f.Exercise != "" && !validID(f.Exercise) {
		return f, errors.New("Choose a valid exercise ID.")
	}
	if f.Template != "" && !validID(f.Template) {
		return f, errors.New("Choose a valid template ID.")
	}
	zone := values.Get("tz")
	if zone == "" {
		zone = "UTC"
	}
	location, err := time.LoadLocation(zone)
	if err != nil || zone == "Local" {
		return f, errors.New("Choose a valid timezone.")
	}
	var end *time.Time
	for _, key := range []string{"from", "to"} {
		if value := values.Get(key); value != "" {
			day, err := time.ParseInLocation("2006-01-02", value, location)
			if err != nil || day.Year() < 1 {
				return f, errors.New("Use valid dates in YYYY-MM-DD format.")
			}
			if key == "from" {
				f.From = &day
			} else {
				end = &day
				next := day.AddDate(0, 0, 1)
				f.Until = &next
			}
		}
	}
	if f.From != nil && end != nil && f.From.After(*end) {
		return f, errors.New("From date must be on or before To date.")
	}
	return f, nil
}

type HistoryExercise struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (p Postgres) HistoryExercises(ctx context.Context, owner int64) ([]HistoryExercise, error) {
	rows, err := p.Pool.Query(ctx, `SELECT exercise_id::text,name FROM (
 SELECT DISTINCT ON(e.exercise_id) e.exercise_id,e.name FROM session_exercises e
 JOIN workout_sessions s ON s.id=e.session_id AND s.owner_id=e.owner_id
 WHERE s.owner_id=$1 AND s.completed_at IS NOT NULL
 ORDER BY e.exercise_id,s.completed_at DESC,s.id DESC,e.position
 ) options ORDER BY lower(name),exercise_id`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []HistoryExercise{}
	for rows.Next() {
		var item HistoryExercise
		if err = rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// Use the latest captured name so renamed or archived templates retain their history.
func (p Postgres) HistoryTemplates(ctx context.Context, owner int64) ([]HistoryExercise, error) {
	rows, err := p.Pool.Query(ctx, `SELECT template_id::text,name FROM (
 SELECT DISTINCT ON(template_id) template_id,name FROM workout_sessions
 WHERE owner_id=$1 AND completed_at IS NOT NULL
 ORDER BY template_id,completed_at DESC,id DESC
 ) options ORDER BY lower(name),template_id`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []HistoryExercise{}
	for rows.Next() {
		var item HistoryExercise
		if err = rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

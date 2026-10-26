package exercises

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

const PageSize int64 = 4

type ListQuery struct {
	Page   int64
	Search string
	Group  string
}
type Page struct {
	Items        []Exercise `json:"items"`
	MuscleGroups []string   `json:"muscle_groups"`
	Number       int64      `json:"page"`
	Size         int64      `json:"page_size"`
	Total        int64      `json:"total_items"`
	Pages        int64      `json:"total_pages"`
}

func parseListQuery(raw string) (ListQuery, error) {
	query := ListQuery{Page: 1}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return query, ValidationError{"query": "Invalid query parameters."}
	}
	for key, entries := range values {
		if len(entries) != 1 {
			return query, ValidationError{key: "Use one value."}
		}
		switch key {
		case "paginated":
			if entries[0] != "true" {
				return query, ValidationError{key: "Use paginated=true."}
			}
		case "page":
			n, err := strconv.ParseInt(entries[0], 10, 64)
			if err != nil || n < 1 || strings.Trim(entries[0], "0123456789") != "" {
				return query, ValidationError{key: "Use a positive whole page number."}
			}
			query.Page = n
		case "q":
			query.Search = strings.TrimSpace(entries[0])
		case "muscle_group":
			query.Group = entries[0]
			found := query.Group == ""
			for _, group := range MuscleGroups {
				found = found || group == query.Group
			}
			if !found {
				return query, ValidationError{key: "Choose a predefined muscle group."}
			}
		default:
			return query, ValidationError{key: "Unknown query parameter."}
		}
	}
	return query, nil
}

func (p Postgres) ListPage(ctx context.Context, owner int64, query ListQuery) (Page, error) {
	result := Page{Items: []Exercise{}, MuscleGroups: MuscleGroups, Number: query.Page, Size: PageSize}
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	// strpos treats %, _ and backslashes literally, matching library substring search.
	const criteria = ` FROM exercises WHERE owner_id=$1 AND ($2='' OR strpos(lower(name),lower($2))>0) AND ($3='' OR $3=ANY(muscle_groups))`
	err = tx.QueryRow(ctx, `SELECT count(*)`+criteria, owner, query.Search, query.Group).Scan(&result.Total)
	if err != nil {
		return result, err
	}
	result.Pages = (result.Total + PageSize - 1) / PageSize
	if result.Number > result.Pages {
		result.Number = result.Pages
	}
	if result.Number < 1 {
		result.Number = 1
	}
	rows, err := tx.Query(ctx, `SELECT id::text,name,muscle_groups,description`+criteria+` ORDER BY exercises.id LIMIT $4 OFFSET $5`, owner, query.Search, query.Group, PageSize, (result.Number-1)*PageSize)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var item Exercise
		if err = rows.Scan(&item.ID, &item.Name, &item.MuscleGroups, &item.Description); err != nil {
			rows.Close()
			return result, err
		}
		result.Items = append(result.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

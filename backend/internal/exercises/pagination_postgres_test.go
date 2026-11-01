package exercises

import (
	"context"
	"github.com/agersk0v/crow-fitness/backend/internal/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"reflect"
	"testing"
)

func TestPostgresPages(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for disposable PostgreSQL tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO owners(id,label) VALUES(82,'Pagination test'),(83,'Pagination foreign')`); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM exercises WHERE owner_id IN(82,83); DELETE FROM owners WHERE id IN(82,83)`)
	store := Postgres{Pool: pool}
	ids := []string{}
	for i := 0; i < 45; i++ {
		item, err := store.Create(ctx, 82, Input{Name: "Same press %_", MuscleGroups: []string{"chest"}})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, item.ID)
	}
	foreign, err := store.Create(ctx, 83, Input{Name: "Same press %_", MuscleGroups: []string{"chest"}})
	if err != nil {
		t.Fatal(err)
	}
	collected := []string{}
	for page := int64(1); page <= 12; page++ {
		result, err := store.ListPage(ctx, 82, ListQuery{Page: page, Search: "PRESS %_", Group: "chest"})
		if err != nil {
			t.Fatal(err)
		}
		expected := 4
		if page == 12 {
			expected = 1
		}
		if len(result.Items) != expected || result.Total != 45 || result.Pages != 12 || result.Number != page || result.Size != 4 || !reflect.DeepEqual(result.MuscleGroups, MuscleGroups) {
			t.Fatalf("%+v", result)
		}
		for _, item := range result.Items {
			if item.ID == foreign.ID {
				t.Fatal("foreign owner leaked")
			}
			collected = append(collected, item.ID)
		}
	}
	if !reflect.DeepEqual(collected, ids) {
		t.Fatalf("page ordering: %v != %v", collected, ids)
	}
	legacy, err := store.List(ctx, 82)
	if err != nil || len(legacy) != 45 {
		t.Fatalf("legacy list truncated: %d %v", len(legacy), err)
	}
	clamped, err := store.ListPage(ctx, 82, ListQuery{Page: 9223372036854775807})
	if err != nil || clamped.Number != 12 {
		t.Fatalf("clamp: %+v %v", clamped, err)
	}
	for _, query := range []ListQuery{{Page: 2, Group: "back"}, {Page: 1, Search: "not found"}, {Page: 1, Search: "%_%"}} {
		result, err := store.ListPage(ctx, 82, query)
		if err != nil || result.Total != 0 || result.Pages != 0 || result.Number != 1 || len(result.Items) != 0 {
			t.Fatalf("empty/filter: %+v %v", result, err)
		}
	}
	for _, id := range ids[44:] {
		if err = store.Delete(ctx, 82, id); err != nil {
			t.Fatal(err)
		}
	}
	result, err := store.ListPage(ctx, 82, ListQuery{Page: 12})
	if err != nil || result.Number != 11 || result.Total != 44 {
		t.Fatalf("deleted last page: %+v %v", result, err)
	}
	if _, err = store.Update(ctx, 82, ids[0], Input{Name: "Renamed", MuscleGroups: []string{"back"}}); err != nil {
		t.Fatal(err)
	}
	result, err = store.ListPage(ctx, 82, ListQuery{Page: 1, Search: "renamed", Group: "back"})
	if err != nil || result.Total != 1 || result.Items[0].ID != ids[0] {
		t.Fatalf("edited filters: %+v %v", result, err)
	}
}

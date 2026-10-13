package exercises

import (
	"context"
	"github.com/agersk0v/crow-fitness/backend/internal/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
)

// TEST_DATABASE_URL must point to a separate disposable database.
func TestPostgresPersistence(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for disposable PostgreSQL test")
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
	store := Postgres{Pool: pool}
	item, err := store.Create(ctx, LocalOwnerID, "Persistence test")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DELETE FROM exercises WHERE id=$1", item.ID)
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	fresh, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	items, err := (Postgres{Pool: fresh}).List(ctx, LocalOwnerID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, saved := range items {
		if saved == item {
			found = true
		}
	}
	if !found {
		t.Fatal("record lost on migration/reconnect")
	}
	if _, err = pool.Exec(ctx, "INSERT INTO owners(id,label) VALUES (2,'Test owner')"); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DELETE FROM owners WHERE id=2")
	other, err := store.Create(ctx, 2, "Other owner exercise")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DELETE FROM exercises WHERE id=$1", other.ID)
	items, err = store.List(ctx, LocalOwnerID)
	if err != nil {
		t.Fatal(err)
	}
	for _, saved := range items {
		if saved.ID == other.ID {
			t.Fatal("another owner's exercise leaked")
		}
	}
	items, err = store.List(ctx, 999)
	if err != nil || len(items) != 0 {
		t.Fatalf("owner isolation: %v %v", items, err)
	}
	if _, err = store.Create(ctx, 999, "Unknown owner"); err == nil {
		t.Fatal("missing owner accepted")
	}
	if _, err = store.Create(ctx, LocalOwnerID, ""); err == nil {
		t.Fatal("blank name accepted")
	}
}

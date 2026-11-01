package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalid = errors.New("invalid or revoked session")

type Account struct {
	ID      int64  `json:"-"`
	OwnerID int64  `json:"-"`
	Subject string `json:"-"`
	Email   string `json:"email"`
	Name    string `json:"name"`
}
type Identity struct{ Subject, Email, Name string }

// SCS hashes browser tokens before calling this store. Tombstones prevent a
// concurrent pre-logout Commit from resurrecting a deleted session.
type sessionStore struct{ pool *pgxpool.Pool }

func (s sessionStore) Find(token string) ([]byte, bool, error) {
	return s.FindCtx(context.Background(), token)
}
func (s sessionStore) Commit(token string, b []byte, expiry time.Time) error {
	return s.CommitCtx(context.Background(), token, b, expiry)
}
func (s sessionStore) Delete(token string) error { return s.DeleteCtx(context.Background(), token) }
func (s sessionStore) FindCtx(ctx context.Context, token string) ([]byte, bool, error) {
	var b []byte
	err := s.pool.QueryRow(ctx, `SELECT data FROM auth_sessions WHERE token=$1 AND NOT revoked AND expiry>now()`, token).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	return b, err == nil, err
}
func (s sessionStore) CommitCtx(ctx context.Context, token string, b []byte, expiry time.Time) error {
	tag, err := s.pool.Exec(ctx, `INSERT INTO auth_sessions(token,data,expiry) VALUES($1,$2,$3) ON CONFLICT(token) DO UPDATE SET data=EXCLUDED.data,expiry=GREATEST(auth_sessions.expiry,EXCLUDED.expiry) WHERE NOT auth_sessions.revoked AND auth_sessions.expiry>now()`, token, b, expiry)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrInvalid
	}
	return err
}
func (s sessionStore) DeleteCtx(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	tag, err := s.pool.Exec(ctx, `INSERT INTO auth_sessions(token,data,expiry,revoked) VALUES($1,''::bytea,now(),true) ON CONFLICT(token) DO UPDATE SET revoked=true,data=''::bytea WHERE NOT auth_sessions.revoked AND auth_sessions.expiry>now()`, token)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrInvalid
	}
	return err
}
func (m *Manager) resolve(ctx context.Context, i Identity) (Account, error) {
	var a Account
	// Serialize only registrations for the same Google subject. No orphan owner
	// can be committed by a losing concurrent first login.
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return a, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,90))`, i.Subject); err != nil {
		return a, err
	}
	err = tx.QueryRow(ctx, `SELECT id,owner_id FROM accounts WHERE google_sub=$1`, i.Subject).Scan(&a.ID, &a.OwnerID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `INSERT INTO owners(label) VALUES('Account owner') RETURNING id`).Scan(&a.OwnerID)
		if err == nil {
			err = tx.QueryRow(ctx, `INSERT INTO accounts(owner_id,google_sub,email,name) VALUES($1,$2,$3,$4) RETURNING id`, a.OwnerID, i.Subject, i.Email, i.Name).Scan(&a.ID)
		}
	} else if err == nil {
		_, err = tx.Exec(ctx, `UPDATE accounts SET email=$2,name=$3 WHERE id=$1`, a.ID, i.Email, i.Name)
	}
	if err != nil {
		return a, err
	}
	a.Subject = i.Subject
	a.Email = i.Email
	a.Name = i.Name
	return a, tx.Commit(ctx)
}
func (m *Manager) account(ctx context.Context, id int64) (Account, error) {
	var a Account
	err := m.pool.QueryRow(ctx, `SELECT id,owner_id,google_sub,coalesce(email,''),coalesce(name,'') FROM accounts WHERE id=$1`, id).Scan(&a.ID, &a.OwnerID, &a.Subject, &a.Email, &a.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrInvalid
	}
	return a, err
}

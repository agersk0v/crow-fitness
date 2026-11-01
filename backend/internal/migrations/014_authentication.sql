-- Existing owner IDs and workout rows are retained and never claimed automatically.
CREATE SEQUENCE owner_ids;
SELECT setval('owner_ids', GREATEST(COALESCE((SELECT max(id) FROM owners), 0) + 1, 1), false);
ALTER TABLE owners ALTER COLUMN id SET DEFAULT nextval('owner_ids');
ALTER SEQUENCE owner_ids OWNED BY owners.id;
CREATE TABLE accounts (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_id bigint NOT NULL UNIQUE REFERENCES owners(id),
    google_sub text NOT NULL UNIQUE CHECK (length(google_sub) > 0),
    email text,
    name text,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE auth_sessions (
    token text PRIMARY KEY,
    data bytea NOT NULL,
    expiry timestamptz NOT NULL,
    revoked boolean NOT NULL DEFAULT false
);
CREATE INDEX auth_sessions_expiry ON auth_sessions(expiry);
CREATE TABLE auth_attempts (
    state text PRIMARY KEY,
    browser text NOT NULL,
    nonce text NOT NULL,
    verifier text NOT NULL,
    expiry timestamptz NOT NULL
);
CREATE INDEX auth_attempts_expiry ON auth_attempts(expiry);

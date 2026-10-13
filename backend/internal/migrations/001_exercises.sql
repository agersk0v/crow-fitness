CREATE TABLE owners (
    id bigint PRIMARY KEY,
    label text NOT NULL
);
INSERT INTO owners (id, label) VALUES (1, 'Local owner');
CREATE TABLE exercises (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_id bigint NOT NULL REFERENCES owners(id),
    name text NOT NULL CHECK (length(btrim(name)) > 0),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX exercises_owner_id_id ON exercises (owner_id, id);

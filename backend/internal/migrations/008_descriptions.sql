-- Library descriptions do not alter captured workout snapshots.
ALTER TABLE exercises ADD COLUMN description text NOT NULL DEFAULT '';
ALTER TABLE templates ADD COLUMN description text NOT NULL DEFAULT '';

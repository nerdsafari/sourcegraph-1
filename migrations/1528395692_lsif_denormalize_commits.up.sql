BEGIN;

CREATE TABLE lsif_nearest_uploads (
    repository_id integer NOT NULL,
    "commit" text NOT NULL,
    upload_id integer NOT NULL,
    distance integer NOT NULL
);

CREATE INDEX lsif_nearest_uploads_repository_id_commit ON lsif_nearest_uploads(repository_id, "commit");

CREATE TABLE lsif_dirty_repositories (
    repository_id integer PRIMARY KEY,
    dirty boolean NOT NULL,
    last_updated_at timestamp with time zone
);

COMMIT;

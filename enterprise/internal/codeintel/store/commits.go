package store

import (
	"context"
	"database/sql"

	"github.com/keegancsmith/sqlf"
)

// scanUploadMeta scans upload metadata grouped by commit from the return value of `*store.query`.
func scanUploadMeta(rows *sql.Rows, queryErr error) (_ map[string][]UploadMeta, err error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer func() { err = closeRows(rows, err) }()

	uploadMeta := map[string][]UploadMeta{}
	for rows.Next() {
		var uploadID int
		var commit string
		var root string
		var indexer string
		if err := rows.Scan(&uploadID, &commit, &root, &indexer); err != nil {
			return nil, err
		}

		uploadMeta[commit] = append(uploadMeta[commit], UploadMeta{
			UploadID: uploadID,
			Root:     root,
			Indexer:  indexer,
		})
	}

	return uploadMeta, nil
}

// HasCommit determines if the given commit is known for the given repository.
func (s *store) HasCommit(ctx context.Context, repositoryID int, commit string) (bool, error) {
	count, _, err := scanFirstInt(s.query(ctx, sqlf.Sprintf(`
		SELECT COUNT(*)
		FROM lsif_nearest_uploads
		WHERE repository_id = %s and commit = %s
		LIMIT 1
	`, repositoryID, commit)))

	return count > 0, err
}

// MarkRepositoryAsDirty marks the given repository's commit graph as out of date.
func (s *store) MarkRepositoryAsDirty(ctx context.Context, repositoryID int) error {
	return s.queryForEffect(
		ctx,
		sqlf.Sprintf(`
			INSERT INTO lsif_dirty_repositories (repository_id, dirty)
			VALUES (%s, true)
			ON CONFLICT (repository_id) DO UPDATE SET dirty = true
		`, repositoryID),
	)
}

// DirtyRepositories returns the set of identifiers for repositories whose commit graphs are out of date.
func (s *store) DirtyRepositories(ctx context.Context) ([]int, error) {
	return scanInts(s.query(ctx, sqlf.Sprintf(`SELECT repository_id FROM lsif_dirty_repositories WHERE dirty = true`)))
}

// CalculateVisibleUploads uses the given commit graph and the tip commit of the default branch to determine
// the set of LSIF uploads that are visible for each commit, and the set of uploads which are visible at the
// tip. The decorated commit graph is serialized to Postgres for use by find closest dumps queries.
func (s *store) CalculateVisibleUploads(ctx context.Context, repositoryID int, graph map[string][]string, tipCommit string) error {
	tx, err := s.transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	uploadMeta, err := scanUploadMeta(tx.query(ctx, sqlf.Sprintf(`
		SELECT id, commit, root, indexer
		FROM lsif_uploads
		WHERE repository_id = %s
	`, repositoryID)))
	if err != nil {
		return err
	}

	visibleUploads, err := calculateVisibleUploads(graph, uploadMeta)
	if err != nil {
		return err
	}

	n := 0
	for _, uploads := range visibleUploads {
		n += len(uploads)
	}

	var ids []*sqlf.Query
	for _, uploadMeta := range visibleUploads[tipCommit] {
		ids = append(ids, sqlf.Sprintf("%s", uploadMeta.UploadID))
	}

	if err := tx.queryForEffect(ctx, sqlf.Sprintf(`DELETE FROM lsif_nearest_uploads WHERE repository_id = %s`, repositoryID)); err != nil {
		return err
	}

	rows := make([]*sqlf.Query, 0, n)
	for commit, uploads := range visibleUploads {
		for _, uploadMeta := range uploads {
			rows = append(rows, sqlf.Sprintf(
				"(%s, %s, %s, %s)",
				repositoryID,
				commit,
				uploadMeta.UploadID,
				uploadMeta.Distance,
			))
		}
	}

	// TODO - extract
	batch := func(queries []*sqlf.Query, n int) [][]*sqlf.Query {
		batchSize := 65535 / n // TODO - define a constant

		var batches [][]*sqlf.Query
		for len(queries) > 0 {
			if len(queries) > batchSize {
				batches = append(batches, queries[:batchSize])
				queries = queries[batchSize:]
			} else {
				batches = append(batches, queries)
			}
		}

		return batches
	}

	for _, batch := range batch(rows, 4) {
		if err := tx.queryForEffect(ctx, sqlf.Sprintf(
			`INSERT INTO lsif_nearest_uploads (repository_id, "commit", upload_id, distance) VALUES %s`,
			sqlf.Join(batch, ","),
		)); err != nil {
			return err
		}
	}

	//
	// TODO - maybe don't store this directly in the table?
	//

	if err := tx.queryForEffect(ctx, sqlf.Sprintf(
		`UPDATE lsif_uploads SET visible_at_tip = (id IN (%s)) WHERE repository_id = %s`,
		sqlf.Join(ids, ","), // TODO - syntax error if empty, I think
		repositoryID,
	)); err != nil {
		return err
	}

	// TODO - only do this if some token matches
	if err := tx.queryForEffect(
		ctx,
		sqlf.Sprintf(`
			INSERT INTO lsif_dirty_repositories (repository_id, dirty, last_updated_at)
			VALUES (%s, false, clock_timestamp())
			ON CONFLICT (repository_id) DO UPDATE SET dirty = false, last_updated_at = clock_timestamp()
		`, repositoryID),
	); err != nil {
		return err
	}

	return nil
}

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

// TODO - document, test
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

// TODO - document, test
func (s *store) DirtyRepositories(ctx context.Context) ([]int, error) {
	return scanInts(s.query(ctx, sqlf.Sprintf(`SELECT repository_id FROM lsif_dirty_repositories WHERE dirty = true`)))
}

// TODO - rename, document, test
func (s *store) FixCommits(ctx context.Context, repositoryID int, graph map[string][]string, tipCommit string) error {
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

	if err := tx.queryForEffect(ctx, sqlf.Sprintf(`DELETE FROM lsif_nearest_uploads WHERE repository_id = %s`, repositoryID)); err != nil {
		return err
	}

	commitMeta := make(map[string]CommitMeta, len(graph))
	for commit, parents := range graph {
		commitMeta[commit] = CommitMeta{
			Parents: parents,
			Uploads: uploadMeta[commit],
		}
	}

	// TODO - rename
	allDistances := calculateReachability(commitMeta)

	n := 0
	for _, uploads := range allDistances {
		n += len(uploads)
	}

	var ids []*sqlf.Query
	for _, uploadMeta := range allDistances[tipCommit] {
		ids = append(ids, sqlf.Sprintf("%s", uploadMeta.UploadID))
	}

	if err := tx.queryForEffect(ctx, sqlf.Sprintf(
		`UPDATE lsif_uploads SET visible_at_tip = (id IN (%s)) WHERE repository_id = %s`,
		sqlf.Join(ids, ","), // TODO - syntax error if empty, I think
		repositoryID,
	)); err != nil {
		return err
	}

	rows := make([]*sqlf.Query, 0, n)
	for commit, uploads := range allDistances {
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

	// TODO - write a small helper for this
	for len(rows) > 0 {
		var batch []*sqlf.Query
		if len(rows) > 65535/4 {
			batch = rows[:65535/4]
			rows = rows[65535/4:]
		} else {
			batch = rows
			rows = nil
		}

		if err := tx.queryForEffect(ctx, sqlf.Sprintf(
			`INSERT INTO lsif_nearest_uploads (repository_id, "commit", upload_id, distance) VALUES %s`,
			sqlf.Join(batch, ","),
		)); err != nil {
			return err
		}
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

// HasCommit determines if the given commit is known for the given repository.
func (s *store) HasCommit(ctx context.Context, repositoryID int, commit string) (bool, error) {
	count, _, err := scanFirstInt(s.query(ctx, sqlf.Sprintf(`
		SELECT COUNT(*)
		FROM lsif_commits
		WHERE repository_id = %s and commit = %s
		LIMIT 1
	`, repositoryID, commit)))

	return count > 0, err
}

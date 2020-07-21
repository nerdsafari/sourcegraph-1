package commits

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/gitserver"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/store"
)

// Updater calculates, denormalizes, and stores the set of uploads visible from every commit
// for a given repository. A repository's commit graph is updated when we receive code intel
// queries for a commit we are unaware of (a commit newer than our latest LSIF upload), and
// after processing an upload for a repository.
type Updater interface {
	Update(ctx context.Context, repositoryID int, blocking bool) error
}

type updater struct {
	store           store.Store
	gitserverClient gitserver.Client
}

func NewUpdater(store store.Store, gitserverClient gitserver.Client) Updater {
	return &updater{
		store:           store,
		gitserverClient: gitserverClient,
	}
}

// Update pulls the commit graph for the given repository from gitserver, pulls the set of
// LSIF upload objects for the given repository from Postgres, and correlates them into a
// visibility graph. This graph is then upserted back into Postgres for use by find closest
// dumps queries.
//
// If blocking is true, then this method will wait for an advisory lock to be granted. If
// blocking is false, this procedure will only be performed if the commit graph for this
// repository is not being calculated by another service.
func (u *updater) Update(ctx context.Context, repositoryID int, blocking bool) (err error) {
	ok, unlock, err := u.store.Lock(ctx, repositoryID, blocking)
	if err != nil || !ok {
		return errors.Wrap(err, "store.Lock")
	}
	defer func() {
		err = unlock(err)
	}()

	graph, err := u.gitserverClient.AllCommits(context.Background(), u.store, repositoryID)
	if err != nil {
		return errors.Wrap(err, "gitserver.AllCommits")
	}

	tipCommit, err := u.gitserverClient.Head(context.Background(), u.store, repositoryID)
	if err != nil {
		return errors.Wrap(err, "gitserver.Head")
	}

	if err := u.store.CalculateVisibleUploads(context.Background(), repositoryID, graph, tipCommit); err != nil {
		return errors.Wrap(err, "store.CalculateVisibleUploads")
	}

	return nil
}

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
	Update(ctx context.Context, repositoryID int, blocking bool, check CheckFunc) error
}

// CheckFunc is the shape of the function invoked to determine if an update is necessary
// after successfully acquiring a lock.
type CheckFunc func(ctx context.Context) (bool, error)

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
//
// If a check function is supplied, it is called after acquiring the lock but before updating
// the commit graph. This can be used to check that an update is still necessary depending on
// the triggering conditions. Returning false from this function will cause the function to
// return without updating. A null function can be passed to skip this check.
func (u *updater) Update(ctx context.Context, repositoryID int, blocking bool, check CheckFunc) (err error) {
	ok, unlock, err := u.store.Lock(ctx, repositoryID, blocking)
	if err != nil || !ok {
		return errors.Wrap(err, "store.Lock")
	}
	defer func() {
		err = unlock(err)
	}()

	if check != nil {
		if ok, err := check(ctx); err != nil || !ok {
			return err
		}
	}

	graph, err := u.gitserverClient.CommitGraph(context.Background(), u.store, repositoryID)
	if err != nil {
		return errors.Wrap(err, "gitserver.CommitGraph")
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

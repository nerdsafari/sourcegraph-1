package commits

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/gitserver"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/store"
)

// TODO - document
type Updater interface {
	Update(ctx context.Context, repositoryID int, blocking bool) error
}

type updater struct {
	store           store.Store
	gitserverClient gitserver.Client
}

// TODO - document
func NewUpdater(store store.Store, gitserverClient gitserver.Client) Updater {
	return &updater{
		store:           store,
		gitserverClient: gitserverClient,
	}
}

// TODO - document, test
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

	if err := u.store.FixCommits(context.Background(), repositoryID, graph, tipCommit); err != nil {
		return errors.Wrap(err, "store.FixCommits")
	}

	return nil
}

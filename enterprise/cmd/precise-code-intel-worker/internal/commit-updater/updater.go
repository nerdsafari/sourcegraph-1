package commitupdater

import (
	"context"
	"errors"
	"time"

	"github.com/efritz/glock"
	"github.com/inconshreveable/log15"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/commits"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/store"
)

// TODO - document
type Updater struct {
	store    store.Store
	updater  commits.Updater
	options  UpdaterOptions
	clock    glock.Clock
	ctx      context.Context // root context passed to the database
	cancel   func()          // cancels the root context
	finished chan struct{}   // signals that Start has finished
}

type UpdaterOptions struct {
	Interval time.Duration
}

func NewUpdater(store store.Store, updater commits.Updater, options UpdaterOptions) *Updater {
	return newUpdater(store, updater, options, glock.NewRealClock())
}

func newUpdater(store store.Store, updater commits.Updater, options UpdaterOptions, clock glock.Clock) *Updater {
	ctx, cancel := context.WithCancel(context.Background())

	return &Updater{
		store:    store,
		updater:  updater,
		options:  options,
		clock:    clock,
		ctx:      ctx,
		cancel:   cancel,
		finished: make(chan struct{}),
	}
}

// TODO - document
func (u *Updater) Start() {
	defer close(u.finished)

loop:
	for {
		repositoryIDs, err := u.store.DirtyRepositories(u.ctx)
		if err != nil {
			log15.Error("Failed to TODO", "err", err)
		}

		for _, repositoryID := range repositoryIDs {
			if err := u.updater.Update(context.Background(), repositoryID, false); err != nil {
				for ex := err; ex != nil; ex = errors.Unwrap(ex) {
					if err == u.ctx.Err() {
						break loop
					}
				}

				log15.Error("Failed to TODO", "err", err)
			}
		}

		select {
		case <-u.clock.After(u.options.Interval):
		case <-u.ctx.Done():
			return
		}
	}
}

// TODO - document
func (u *Updater) Stop() {
	u.cancel()
	<-u.finished
}

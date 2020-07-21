package commitupdater

import (
	"testing"
	"time"

	"github.com/efritz/glock"
	commitsmocks "github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/commits/mocks"
	storemocks "github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/store/mocks"
)

func TestUpdater(t *testing.T) {
	store := storemocks.NewMockStore()
	updater := commitsmocks.NewMockUpdater()
	clock := glock.NewMockClock()
	options := UpdaterOptions{
		Interval: time.Second,
	}

	store.DirtyRepositoriesFunc.SetDefaultReturn([]int{50, 51, 52}, nil)

	periodicUpdater := newUpdater(store, updater, options, clock)
	go func() { periodicUpdater.Start() }()
	clock.BlockingAdvance(time.Second)
	periodicUpdater.Stop()

	if callCount := len(updater.UpdateFunc.History()); callCount < 3 {
		t.Errorf("unexpected update call count. want>=%d have=%d", 3, callCount)
	} else {
		for i, repositoryID := range []int{50, 51, 52} {
			if updater.UpdateFunc.History()[i].Arg1 != repositoryID {
				t.Errorf("unexpected repository id arg. want=%d have=%d", repositoryID, updater.UpdateFunc.History()[0].Arg1)
			}
			if updater.UpdateFunc.History()[i].Arg2 {
				t.Errorf("unexpected blocking arg. want=%v have=%v", false, updater.UpdateFunc.History()[0].Arg2)
			}
		}
	}
}

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

	store.DirtyRepositoriesFunc.SetDefaultReturn(map[int]int{50: 3, 51: 2, 52: 6}, nil)

	periodicUpdater := newUpdater(store, updater, options, clock)
	go func() { periodicUpdater.Start() }()
	clock.BlockingAdvance(time.Second)
	periodicUpdater.Stop()

	if callCount := len(updater.TryUpdateFunc.History()); callCount < 3 {
		t.Errorf("unexpected update call count. want>=%d have=%d", 3, callCount)
	} else {
		testCases := []struct {
			repositoryID int
			dirtyFlag    int
		}{
			{50, 3},
			{51, 2},
			{52, 6},
		}
		for i, testCase := range testCases {
			if updater.TryUpdateFunc.History()[i].Arg1 != testCase.repositoryID {
				t.Errorf("unexpected repository id arg. want=%d have=%d", testCase.repositoryID, updater.TryUpdateFunc.History()[0].Arg1)
			}
			if updater.TryUpdateFunc.History()[i].Arg2 != testCase.dirtyFlag {
				t.Errorf("unexpected dirty flag arg. want=%d have=%d", testCase.dirtyFlag, updater.TryUpdateFunc.History()[0].Arg2)
			}
		}
	}
}

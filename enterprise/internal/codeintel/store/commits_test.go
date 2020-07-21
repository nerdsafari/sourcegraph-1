package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/db/dbtesting"
)

func TestHasCommit(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dbtesting.SetupGlobalTestDB(t)
	store := rawTestStore()

	testCases := []struct {
		repositoryID int
		commit       string
		exists       bool
	}{
		{50, makeCommit(1), true},
		{50, makeCommit(2), false},
		{51, makeCommit(1), false},
	}

	// if err := store.UpdateCommits(context.Background(), 50, map[string][]string{
	// 	makeCommit(1): {},
	// }); err != nil {
	// 	t.Fatalf("unexpected error updating commits: %s", err)
	// }

	for _, testCase := range testCases {
		name := fmt.Sprintf("repositoryID=%d commit=%s", testCase.repositoryID, testCase.commit)

		t.Run(name, func(t *testing.T) {
			exists, err := store.HasCommit(context.Background(), testCase.repositoryID, testCase.commit)
			if err != nil {
				t.Fatalf("unexpected error checking if commit exists: %s", err)
			}
			if exists != testCase.exists {
				t.Errorf("unexpected exists. want=%v have=%v", testCase.exists, exists)
			}
		})
	}
}

func TestMarkRepositoryAsDirty(t *testing.T) {
	// TODO
}

func TestDirtyRepositories(t *testing.T) {
	// TODO
}

func TestCalculateVisibleUploads(t *testing.T) {
	// TODO
}

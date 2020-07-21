package store

import (
	"context"

	"github.com/hashicorp/go-multierror"
	"github.com/keegancsmith/sqlf"
)

// TODO - document, choose better value
const appLockKey = 1330

// TODO - document
type UnlockFunc func(err error) error

// TODO - document, test
func (s *store) Lock(ctx context.Context, key int, blocking bool) (locked bool, _ UnlockFunc, err error) {
	if blocking {
		locked, err = s.lock(ctx, key)
	} else {
		locked, err = s.tryLock(ctx, key)
	}

	if err != nil || !locked {
		return false, nil, err
	}

	unlock := func(err error) error {
		if unlockErr := s.unlock(key); unlockErr != nil {
			err = multierror.Append(err, unlockErr)
		}

		return err
	}

	return true, unlock, nil
}

// TODO - document
func (s *store) lock(ctx context.Context, key int) (bool, error) {
	err := s.queryForEffect(ctx, sqlf.Sprintf(`SELECT pg_advisory_lock(%s, %s)`, appLockKey, key))
	if err != nil {
		return false, err
	}
	return true, nil
}

// TODO - document
func (s *store) tryLock(ctx context.Context, key int) (bool, error) {
	ok, _, err := scanFirstBool(s.query(ctx, sqlf.Sprintf(`SELECT pg_try_advisory_lock(%s, %s)`, appLockKey, key)))
	if err != nil || !ok {
		return false, err
	}
	return true, nil
}

// TODO - document
func (s *store) unlock(key int) error {
	err := s.queryForEffect(context.Background(), sqlf.Sprintf(`SELECT pg_advisory_unlock(%s, %s)`, appLockKey, key))
	return err
}

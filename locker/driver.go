package locker

import (
	"context"

	"github.com/sosnovski/synch/locker/lock"
)

// Driver represents an interface for acquiring and managing distributed locks.
type Driver interface {
	// TryLock attempts to acquire a distributed lock with the specified parameters.
	//
	// It takes a context.Context and a lock.Params object as input and returns a pointer to a lock.Lock object and an error.
	//
	// If the lock acquisition is successful, the returned lock.Lock object contains information such as the lock ID, instance ID,
	// timeout duration, and heartbeat interval. If the lock acquisition fails, an error is returned.
	//
	TryLock(ctx context.Context, params lock.Params) (*lock.Lock, error)
}

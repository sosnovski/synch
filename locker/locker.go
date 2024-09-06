// Package locker provides functionalities to locks in your application.
// Specifically, it manages allocation and release of resources in multithreaded and distributed environments,
// providing synchronization services.
// You can create a lock object and use this to protect critical sections of the code.
package locker

import (
	"context"
	stdErrors "errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/sosnovski/synch/locker/errors"
	"github.com/sosnovski/synch/locker/lock"
	"github.com/sosnovski/synch/log"
)

const (
	// defaultLockTimeout is the default timeout for acquiring a lock.
	defaultLockTimeout = 5 * time.Second

	// defaultHeartbeatInterval is the default interval at which heartbeats are sent to maintain a lock's validity.
	defaultHeartbeatInterval = 1 * time.Second

	// defaultPrefix is the default prefix used for generating the instance ID.
	defaultPrefix = "synch"
)

var (
	ErrApplyOptions     = stdErrors.New("apply locker options error")
	ErrApplyLockOptions = stdErrors.New("apply lock options error")
)

// Locker represents a distributed lock manager that uses a given Driver
// for acquiring and managing distributed locks. It provides various methods for
// acquiring and using locks.
type Locker struct {
	log                log.Logger
	driver             Driver
	onWaitIterateError func(ctx context.Context, err error)
	instanceID         string
}

// New creates a new instance of the Locker struct.
//
// The Locker object represents a distributed lock manager that uses the given Driver
// for acquiring and managing distributed locks. The Driver interface is responsible
// for acquiring and releasing locks.
//
// If the driver is nil, it returns an error errors.ErrDriverIsNil.
//
// If applying an option fails, it returns an error with the message ErrApplyOptions: <error>.
//
// Example:
//
//	driver := ...
//	locker, err := New(driver)
//	if err != nil {
//	  // handle error
//	}
//	// use locker to acquire and release locks
func New(driver Driver, options ...Option) (*Locker, error) {
	locker := &Locker{
		log:                log.Noop{},
		driver:             driver,
		instanceID:         fmt.Sprintf("%s-%d-%d", defaultPrefix, time.Now().UnixNano(), os.Getpid()),
		onWaitIterateError: nil,
	}

	if driver == nil {
		return nil, errors.ErrDriverIsNil
	}

	for _, option := range options {
		if err := option(locker); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrApplyOptions, err)
		}
	}

	return locker, nil
}

// TryLock attempts to acquire a lock with the given lockID and options.
//
// ctx: context for acquire lock.
// lockID: the ID of the lock to acquire.
// options: options to customize lock acquisition.
//
// Returns the acquired lock object if successful, or an error if unsuccessful.
// If successful, the lock object should be closed using the Close() method when no longer needed.
//
// Example:
//
//	driver := ...
//	locker, err := New(driver)
//	if err != nil {
//	  // handle error
//	}
//	lock, err := locker.TryLock(ctx, "my_lock_id")
//	if err != nil {
//	  // handle error
//	}
//
//	defer lock.Close(ctx)
//	// do something
func (l *Locker) TryLock(ctx context.Context, lockID string, options ...lock.Option) (*lock.Lock, error) {
	params := lock.Params{
		ID:                lockID,
		InstanceID:        l.instanceID,
		Timeout:           defaultLockTimeout,
		HeartbeatInterval: defaultHeartbeatInterval,
		Data:              nil,
		GroupID:           "",
	}

	for _, option := range options {
		if err := option(&params); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrApplyLockOptions, err)
		}
	}

	if params.HeartbeatInterval*2 > params.Timeout {
		return nil, fmt.Errorf(
			"%w: heartbeet interval %d should be at least twice as small as %d timeout",
			errors.ErrLockHeartbeatIntervalToHigh,
			params.HeartbeatInterval,
			params.Timeout,
		)
	}

	lockObj, err := l.driver.TryLock(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("driver: %w", err)
	}

	return lockObj, nil
}

// TryLockDo attempts to acquire a lock with the given lockID and options, and then executes the provided function.
//
// ctx: context for acquire lock.
// lockID: the ID of the lock to acquire.
// fun: the function to be executed.
// options: options to customize lock acquisition.
//
// If successful, after the function is executed,
// the lock will be released automatically by calling the lock.Close method.
//
// Example:
//
//	driver := ...
//	locker, err := New(driver)
//	if err != nil {
//	  // handle error
//	}
//	err := locker.TryLockDo(ctx, "my_lock_id", func(_ context.Context) error {
//		// do something
//		return nil
//	})
//	if err != nil {
//	  // handle error
//	}
func (l *Locker) TryLockDo(
	ctx context.Context,
	lockID string,
	fun func(context.Context) error,
	options ...lock.Option,
) error {
	closer, err := l.TryLock(ctx, lockID, options...)
	if err != nil {
		return fmt.Errorf("try lock: %w", err)
	}

	return do(ctx, closer, fun)
}

// WaitLock acquires a lock with the given lockID and options. If acquiring the lock fails,
// it waits for a specified interval and tries again until the lock is acquired or the context is canceled.
// If an error occurs during waiting or acquiring the lock, an error is returned.
//
// lockID: the ID of the lock to acquire.
// retryInterval: the duration to wait between retry attempts.
// options: options to customize lock acquisition.
//
// Returns the acquired lock object if successful, or an error if unsuccessful.
// If successful, the lock object should be closed using the Close() method when no longer needed.
//
// Example:
//
//	locker := ...
//	lock, err := locker.WaitLock(ctx, "my_lock_id", time.Second)
//	if err != nil {
//	  // handle error
//	}
//
//	defer lock.Close(ctx)
//	// do something with the lock
func (l *Locker) WaitLock(
	ctx context.Context,
	lockID string,
	retryInterval time.Duration,
	options ...lock.Option,
) (*lock.Lock, error) {
	for {
		lockObg, err := l.TryLock(ctx, lockID, options...)
		if err != nil {
			if l.onWaitIterateError != nil {
				l.onWaitIterateError(ctx, err)
			}

			if stdErrors.Is(err, errors.ErrLockAlreadyHeld) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(retryInterval):
					continue
				}
			}

			return nil, err
		}

		return lockObg, nil
	}
}

// WaitLockDo acquires a lock with the given lockID and options, and then executes the provided function.
// If acquiring the lock fails, it waits for a specified interval
// and tries again until the lock is acquired or the context is canceled.
// If an error occurs during waiting or acquiring the lock, an error is returned.
//
// ctx: context for acquire lock.
// lockID: the ID of the lock to acquire.
// retryInterval: the duration to wait between retry attempts.
// fun: the function to be executed.
// options: options to customize lock acquisition.
//
// If successful, after the function is executed,
// the lock will be released automatically by calling the lock.Close method.
//
// Example:
//
//	locker, err := New(driver)
//	if err != nil {
//	  // handle error
//	}
//	err := locker.WaitLockDo(ctx, "my_lock_id", time.Second, func(_ context.Context) error {
//		// do something
//		return nil
//	})
//	if err != nil {
//	  // handle error
//	}
func (l *Locker) WaitLockDo(
	ctx context.Context,
	lockID string,
	retryInterval time.Duration,
	fun func(context.Context) error,
	options ...lock.Option,
) error {
	lockObj, err := l.WaitLock(ctx, lockID, retryInterval, options...)
	if err != nil {
		return fmt.Errorf("wait lock: %w", err)
	}

	return do(ctx, lockObj, fun)
}

// do execute function end return errors if they exist.
func do(ctx context.Context, lock *lock.Lock, fun func(context.Context) error) error {
	var (
		resultErr error
		done      = make(chan struct{})
		mu        = sync.Mutex{}
		appendErr = func(err error) {
			mu.Lock()
			defer mu.Unlock()

			resultErr = multierror.Append(resultErr, err)
		}
	)
	go func() {
		if err := fun(lock.ShutdownCtx()); err != nil {
			appendErr(fmt.Errorf("fun: %w", err))
		}
		close(done)
	}()

	select {
	case <-ctx.Done():
		appendErr(ctx.Err())
	case <-done:
	}

	if err := lock.Close(context.Background()); err != nil {
		appendErr(fmt.Errorf("close lock: %w", err))
	}

	<-done

	if resultErr != nil {
		return resultErr
	}

	return nil
}

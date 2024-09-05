// Package lock represents a Lock struct for control the lock.
// Also represents a silentCancelContext for the correct operation with shutdown errors.
package lock

import (
	"context"
	"sync/atomic"
	"time"
)

// closer represents a function that takes a context.Context argument and returns an error.
// It is typically used to perform cleanup or closing operations.
type closer func(ctx context.Context) error

// Lock represents a lock object used to manage distributed locks.
// It contains information such as the shutdown context, a closer function for cleanup or closing operations,
// lock ID and instance ID, timeout, and heartbeat interval.
type Lock struct {
	shutdownCtx       context.Context
	closer            closer
	id                string
	instanceID        string
	groupID           string
	data              []byte
	timeout           time.Duration
	heartbeatInterval time.Duration
	closed            atomic.Bool
}

// NewLock returns a new instance of the Lock struct with the given parameters.
// The Lock struct represents a lock object used to manage distributed locks.
// It contains information such as the shutdown context, a closer function for cleanup and closing operations,
// lock ID and instance ID, timeout, and heartbeat interval.
func NewLock(
	shutdownCtx context.Context,
	closer closer,
	id string,
	instanceID string,
	timeout time.Duration,
	heartbeatInterval time.Duration,
	data []byte,
	groupID string,
) *Lock {
	return &Lock{
		shutdownCtx:       shutdownCtx,
		closed:            atomic.Bool{},
		closer:            closer,
		id:                id,
		instanceID:        instanceID,
		timeout:           timeout,
		heartbeatInterval: heartbeatInterval,
		data:              data,
		groupID:           groupID,
	}
}

// ID returns the ID of the lock.
func (l *Lock) ID() string {
	return l.id
}

// InstanceID returns the instance ID of the lock.
func (l *Lock) InstanceID() string {
	return l.instanceID
}

// Timeout returns the timeout duration of the lock.
func (l *Lock) Timeout() time.Duration {
	return l.timeout
}

// HeartbeatInterval returns the heartbeat interval duration of the lock.
func (l *Lock) HeartbeatInterval() time.Duration {
	return l.heartbeatInterval
}

// ShutdownCtx returns the shutdown context associated with the lock.
// It can be used to monitor when the lock has been shut down.
func (l *Lock) ShutdownCtx() context.Context {
	return l.shutdownCtx
}

// Data returns the data associated with the lock as a byte slice.
func (l *Lock) Data() []byte {
	return l.data
}

// GroupID returns the group id associated with the lock.
func (l *Lock) GroupID() string {
	return l.groupID
}

// Close closes the lock.
// This method cancels the shutdown context and waits for all Goroutines to finish.
// It is safe to use concurrently.
// It is safe for repeat calls.
func (l *Lock) Close(ctx context.Context) error {
	if l.closed.Swap(true) {
		return nil
	}

	return l.closer(ctx)
}

// Params represents a set of parameters used for acquiring and managing a lock.
type Params struct {
	ID                string
	InstanceID        string
	GroupID           string
	Data              []byte
	Timeout           time.Duration
	HeartbeatInterval time.Duration
}

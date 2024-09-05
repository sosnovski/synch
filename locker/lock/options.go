package lock

import (
	"time"

	"github.com/sosnovski/synch/locker/errors"
)

// Option represents a function type that modifies Params.
type Option func(params *Params) error

// WithInstanceID is a LockOption that sets the InstanceID field of the lock.Params struct.
// It returns an error of type ErrInstanceIDIsEmpty if the provided instanceID is empty.
func WithInstanceID(instanceID string) Option {
	return func(l *Params) error {
		if instanceID == "" {
			return errors.ErrInstanceIDIsEmpty
		}

		l.InstanceID = instanceID

		return nil
	}
}

// WithTimeout modifies the lock.Params.Timeout field with the provided value.
// It returns an errors.ErrLockTimeoutIsEmpty if the timeout is 0.
func WithTimeout(timeout time.Duration) Option {
	return func(l *Params) error {
		if timeout == 0 {
			return errors.ErrLockTimeoutIsEmpty
		}

		l.Timeout = timeout

		return nil
	}
}

// WithHeartbeatInterval is a LockOption function that modifies the HeartbeatInterval
// field of lock.Params. This field represents the optional duration between sending
// heartbeats to keep the lock alive. It should be set to a value that is half as much
// as the Timeout field. If the interval parameter is zero, it returns an error of type
// errors.ErrLockHeartbeatIntervalIsEmpty.
func WithHeartbeatInterval(interval time.Duration) Option {
	return func(l *Params) error {
		if interval == 0 {
			return errors.ErrLockHeartbeatIntervalIsEmpty
		}

		l.HeartbeatInterval = interval

		return nil
	}
}

// WithData is a LockOption that sets the Data field of the Params struct.
// The provided Data is assigned to the data field of the lock table.
// It always returns nil error.
func WithData(data []byte) Option {
	return func(l *Params) error {
		l.Data = data

		return nil
	}
}

// WithGroupID is a LockOption that sets the GroupID field of the Params struct.
// The provided GroupID is assigned to the group_id field of the lock table.
// It always returns nil error.
func WithGroupID(groupID string) Option {
	return func(l *Params) error {
		l.GroupID = groupID

		return nil
	}
}

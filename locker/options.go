package locker

import (
	"context"

	"github.com/sosnovski/synch/locker/errors"
	"github.com/sosnovski/synch/log"
)

// Option represents a functional option for configuring a Locker object.
//
// An Option is a function that takes a pointer to a Locker object as a parameter
// and returns an error. It allows users to customize the behavior of a Locker object
// by applying various configuration options.
//
// The Option function should validate the input parameters and modify the Locker object accordingly.
// If any validation fails, the Option function should return a non-nil error.
//
// Example:
//
//	func WithDefaultInstanceID(instanceID string) Option {
//		return func(l *Locker) error {
//			if instanceID == "" {
//				return errors.ErrInstanceIDIsEmpty
//			}
//
//			l.instanceID = instanceID
//
//			return nil
//		}
//	}
//
//	func WithLogger(log log.Logger) Option {
//		return func(l *Locker) error {
//			if log == nil {
//				return errors.ErrLoggerIsNil
//			}
//
//			l.log = log
//
//			return nil
//		}
//	}
//
//	func WithOnWaitIterateError(f func(ctx context.Context, err error)) Option {
//		return func(l *Locker) error {
//			if f == nil {
//				return errors.ErrOnWaitIterateErrorFuncIsNil
//			}
//
//			l.onWaitIterateError = f
//
//			return nil
//		}
//	}
//
//	func NewLocker(driver Driver, options ...Option) (*Locker, error) {
//		locker := &Locker{
//			log:        log.Noop{},
//			driver:     driver,
//			instanceID: fmt.Sprintf("%s-%d-%d", defaultPrefix, time.Now().UnixNano(), os.Getpid()),
//		}
//
//		if driver == nil {
//			return nil, errors.ErrDriverIsNil
//		}
//
//		for _, option := range options {
//			if err := option(locker); err != nil {
//				return nil, fmt.Errorf("%w: %w", ErrApplyOptions, err)
//			}
//		}
//
//		return locker, nil
//	}
type Option func(*Locker) error

func WithDefaultInstanceID(instanceID string) Option {
	return func(l *Locker) error {
		if instanceID == "" {
			return errors.ErrInstanceIDIsEmpty
		}

		l.instanceID = instanceID

		return nil
	}
}

func WithLogger(log log.Logger) Option {
	return func(l *Locker) error {
		if log == nil {
			return errors.ErrLoggerIsNil
		}

		l.log = log

		return nil
	}
}

func WithOnWaitIterateError(f func(ctx context.Context, err error)) Option {
	return func(l *Locker) error {
		if f == nil {
			return errors.ErrOnWaitIterateErrorFuncIsNil
		}

		l.onWaitIterateError = f

		return nil
	}
}

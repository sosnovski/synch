package errors

import (
	"errors"
)

var (
	ErrLockAlreadyHeld              = errors.New("lock already held")
	ErrLockHasBeenLost              = errors.New("lock has bee lost")
	ErrHeartbeatIntervalToHigh      = errors.New("heartbeat interval too high")
	ErrDriverIsNil                  = errors.New("driver is nil")
	ErrInstanceIDIsEmpty            = errors.New("instance id is empty")
	ErrLoggerIsNil                  = errors.New("logger is nil")
	ErrLockTimeoutIsEmpty           = errors.New("lock timeout is empty")
	ErrLockHeartbeatIntervalIsEmpty = errors.New("lock heartbeat interval is empty")
	ErrOnWaitIterateErrorFuncIsNil  = errors.New("on wait iterate error func is nil")
)

package redis

import (
	"errors"
)

var (
	ErrLoggerIsNil          = errors.New("logger is nil")
	ErrPrefixLockKeyIsEmpty = errors.New("lock prefix key is empty")
	ErrClientIsNil          = errors.New("client is nil")
	ErrApplyOptions         = errors.New("apply driver options error")
	ErrCommandFailed        = errors.New("command failed")
)

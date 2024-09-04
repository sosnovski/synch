package sql

import (
	"errors"
)

var (
	ErrDialectNotImplementMigrate = errors.New("dialect not implement migrate interface")
	ErrMigrationContextMustBeSet  = errors.New("migration context must be set")
	ErrLoggerIsNil                = errors.New("logger is nil")
	ErrTableNameIsEmpty           = errors.New("table name is empty")
	ErrDialectIsNil               = errors.New("dialect is nil")
	ErrConnIsNil                  = errors.New("connection is nil")
	ErrApplyOptions               = errors.New("apply driver options error")
	ErrMigrateFailed              = errors.New("migrate failed")
)

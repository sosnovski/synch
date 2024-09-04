package sql

import (
	"context"
)

// Option is a type representing an option function that can be used to modify a Driver instance.
type Option func(*Driver) error

// WithTableName is an option function that sets the name of the database table used for managing locks.
// It returns a function that modifies the Driver instance by setting the provided table name.
// If the provided name is empty, it returns an ErrTableNameIsEmpty error.
func WithTableName(name string) Option {
	return func(d *Driver) error {
		if name == "" {
			return ErrTableNameIsEmpty
		}

		d.tableName = name

		return nil
	}
}

// WithAutoMigration is an option function that enables or disables the auto migration feature in the Driver instance.
// If enable is true, it checks if the dialect implements the MigrateDialect interface.
// If the dialect does not implement the MigrateDialect interface, it returns an ErrDialectNotImplementMigrate error.
// If enable is false, it simply sets the enableAutoMigration field to the provided value.
func WithAutoMigration(enable bool) Option {
	return func(d *Driver) error {
		if enable {
			if _, ok := d.dialect.(MigrateDialect); !ok {
				return ErrDialectNotImplementMigrate
			}
		}

		d.enableAutoMigration = enable

		return nil
	}
}

// WithMigrationContext is an option function that sets the migration context for the Driver instance.
// If the provided context is nil, it returns an ErrMigrationContextMustBeSet error.
// If the dialect of the Driver instance does not implement the MigrateDialect interface,
// it returns an ErrDialectNotImplementMigrate error.
func WithMigrationContext(ctx context.Context) Option {
	return func(d *Driver) error {
		if ctx == nil {
			return ErrMigrationContextMustBeSet
		}

		if _, ok := d.dialect.(MigrateDialect); !ok {
			return ErrDialectNotImplementMigrate
		}

		d.migrationCtx = ctx

		return nil
	}
}

// WithLogger is an option function that sets the logger used for logging warnings.
// If the provided logger is nil, it returns an ErrLoggerIsNil error.
func WithLogger(log Logger) Option {
	return func(d *Driver) error {
		if log == nil {
			return ErrLoggerIsNil
		}

		d.log = log

		return nil
	}
}

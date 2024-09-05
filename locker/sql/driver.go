// Package sql represent lock.Driver implementation for work with SQL databases.
// It includes implementation Dialect interface for postgres and mysql by PostgresDialect,
// MysqlDialect and MariadbDialect structs.
package sql

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/sosnovski/synch/locker/errors"
	"github.com/sosnovski/synch/locker/lock"
	"github.com/sosnovski/synch/log"
)

const (
	defaultTableName = "synch_locks" // defaultTableName is the default name of the database table used for managing locks.
)

// Driver represents a database driver used for managing locks in a database table.
type Driver struct {
	// log represents a Logger used for logging warnings.
	// Use WithLogger option for set the logger for the Driver instance.
	// Default: log.Noop
	log Logger

	// dialect represents a Dialect interface used for managing locks in a database table.
	dialect Dialect

	// migrationCtx represents an optional context used for migrations in the Driver type.
	// Use WithMigrationContext for set migration app-specific context for the Driver instance.
	// Default: nil
	migrationCtx context.Context

	// conn is a pointer to sql.DB used for managing database connections and transactions.
	conn *sql.DB

	// tableName contains the name of the database table used for managing locks.
	// Use WithTableName for setting app-specific table name for the Driver instance.
	// Default: see defaultTableName
	tableName string

	// enableAutoMigration represents a boolean flag indicating whether the migration feature is enabled.
	// Use WithAutoMigration for manage enableAutoMigration option for the Driver instance.
	// Default: false
	enableAutoMigration bool
}

// NewDriver creates a new instance of the Driver struct with the specified connection, dialect, and options.
// It returns the created Driver instance and an error if any.
//
// The `conn` parameter is a pointer to sql.DB used for managing database connections and transactions.
//
// The `dialect` parameter is an interface used for managing locks in a database table.
// The Dialect interface defines methods for upserting and deleting a lock, as well as sending a heartbeat to keep the lock alive.
//
// The `options` parameter is a variadic list of Option functions that can be used to modify the created Driver instance.
// Using options may cause errors. See specific options for details.
//
// If the `dialect` parameter is nil, it returns an error of type ErrDialectIsNil.
// If the `conn` parameter is nil, it returns an error of type ErrConnIsNil.
//
// If the enableAutoMigration field of the created Driver instance is true, it attempts to migrate the database using the Migrate method.
// If the migration fails, it returns an error of type ErrMigrateFailed.
//
// Example:
//
//	conn, err := sql.Open("postgres", "user=postgres password=secret dbname=mydb")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	dialect := MyDialect{}
//
//	driver, err := NewDriver(conn, dialect, WithAutoMigration(true), WithTableName("locks"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Use the driver instance...
func NewDriver(conn *sql.DB, dialect Dialect, options ...Option) (*Driver, error) {
	driver := &Driver{
		log:                 log.Noop{},
		conn:                conn,
		dialect:             dialect,
		tableName:           defaultTableName,
		enableAutoMigration: false,
		migrationCtx:        context.Background(),
	}

	if dialect == nil {
		return nil, ErrDialectIsNil
	}

	if conn == nil {
		return nil, ErrConnIsNil
	}

	for _, option := range options {
		if err := option(driver); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrApplyOptions, err)
		}
	}

	if driver.enableAutoMigration {
		if err := driver.Migrate(driver.migrationCtx); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrMigrateFailed, err)
		}
	}

	return driver, nil
}

// TryLock tries to acquire a lock with the specified parameters.
// If no rows are affected by the UpsertLock method, it returns an errors.ErrLockAlreadyHeld.
// If manage to take the lock, the method also starts the heartbeat automatically.
// If you use PostgresDialect, MysqlDialect or MariadbDialect their implementations are safe for use concurrently,
// in other implementations, safety is not guaranteed.
func (d *Driver) TryLock(
	ctx context.Context,
	params lock.Params,
) (*lock.Lock, error) {
	res, err := d.dialect.UpsertLock(ctx, d.conn, d.tableName, params)
	if err != nil {
		return nil, fmt.Errorf("upsert lock: %w", err)
	}

	rowsCount, _ := res.RowsAffected()
	if rowsCount == 0 {
		return nil, errors.ErrLockAlreadyHeld
	}

	wg := &sync.WaitGroup{}
	shutdownCtx, shutdown := d.startHeartbeat(wg, params)

	shutdownFun := func(ctx context.Context) error {
		shutdown(nil)
		wg.Wait()

		if err := d.dialect.DeleteLock(ctx, d.conn, d.tableName, params); err != nil {
			return fmt.Errorf("delete lock: %w", err)
		}

		return nil
	}

	return lock.New( //nolint:contextcheck //because lock.Lock struct should give a shutdown context
		lock.SilentCancelContext(shutdownCtx),
		shutdownFun,
		params.ID,
		params.InstanceID,
		params.Timeout,
		params.HeartbeatInterval,
		params.Data,
		params.GroupID,
	), nil
}

// Migrate migrates the database table using the Migrate method of the MigrateDialect interface.
// If dialect was not implemented MigrateDialect, this method will be panic.
// It is unsafe to use concurrently if your implementation of Dialect interface has no check if table exists.
// Method return ErrDialectNotImplementMigrate if dialect not implementing MigrateDialect.
func (d *Driver) Migrate(ctx context.Context) error {
	migrate, ok := d.dialect.(MigrateDialect)
	if !ok {
		return ErrDialectNotImplementMigrate
	}

	if err := migrate.Migrate(ctx, d.conn, d.tableName); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return nil
}

// startHeartbeat starts the heartbeat goroutine to send heartbeats at a specified interval.
func (d *Driver) startHeartbeat(wg *sync.WaitGroup, params lock.Params) (context.Context, context.CancelCauseFunc) {
	shutdownCtx, shutdown := context.WithCancelCause(context.Background())

	go func() {
		ticker := time.NewTicker(params.HeartbeatInterval)
		defer ticker.Stop()

		for {
			wg.Add(1)

			if err := d.sendHeartbeat(shutdownCtx, params, wg); err != nil {
				shutdown(fmt.Errorf("failed to send heartbeat: %w", err))
			}

			select {
			case <-ticker.C:
				continue
			case <-shutdownCtx.Done():
				return
			}
		}
	}()

	return shutdownCtx, shutdown
}

// sendHeartbeat sends a heartbeat to keep the lock alive.
// If no rows are affected by the Heartbeat method, it returns an errors.ErrLockHasBeenLost.
func (d *Driver) sendHeartbeat(ctx context.Context, params lock.Params, wg *sync.WaitGroup) error {
	defer wg.Done()

	res, err := d.dialect.Heartbeat(ctx, d.conn, d.tableName, params)
	if err != nil {
		return fmt.Errorf("heartbeat: %w", err)
	}

	rowsCount, _ := res.RowsAffected()
	if rowsCount == 0 {
		return errors.ErrLockHasBeenLost
	}

	return nil
}

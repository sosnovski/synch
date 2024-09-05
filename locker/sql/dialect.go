package sql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sosnovski/synch/locker/lock"
)

// MigrateDialect is an interface that defines the Migrate method. This method is used to perform migrations on a
// database table. It returns an error if there was a problem performing the migration.
type MigrateDialect interface {
	Migrate(ctx context.Context, conn *sql.DB, tableName string) error
}

// Dialect is an interface used for managing locks in a database table. It defines methods
// for upserting a lock, deleting a lock, and sending a heartbeat to keep the lock alive.
//
// The UpsertLock method upserts a lock in the database table. It takes in a context, connection
// to the database, table name, and lock parameters. It returns an `sql.Result` and an error. The
// lock parameters should be of type lock.Params, which has fields such as ID, InstanceID,
// Timeout, and HeartbeatInterval.
//
// The DeleteLock method deletes a lock from the database table. It takes in a context,
// connection to the database, table name, and lock parameters. It returns an error.
//
// The Heartbeat method sends a heartbeat to keep the lock alive. It takes in a context,
// connection to the database, table name, and lock parameters. It returns an `sql.Result`
// and an error.
type Dialect interface {
	// UpsertLock is a method that upsets a lock in a database table. It returns an `sql.Result` and an error.
	//
	// The lock parameters should be of type lock.Params, which has the following fields:
	//   - ID: The unique identifier of the lock
	//   - InstanceID: The identifier of the instance acquiring the lock
	//   - Timeout: The duration after which the lock will expire
	//   - HeartbeatInterval: The duration between sending heartbeats to keep the lock alive
	//
	// This method is part of the Dialect interface, which is used for managing locks in a
	// database table. It is typically called from the TryLock method in the Driver struct.
	//
	// Example usage:
	//   res, err := d.dialect.UpsertLock(ctx, d.conn, d.tableName, params)
	//   if err != nil {
	//     return nil, fmt.Errorf("upsert lock: %w", err)
	//   }
	//
	//   rowsCount, _ := res.RowsAffected()
	//   if rowsCount == 0 {
	//     return nil, errors.ErrLockAlreadyHeld
	//   }
	UpsertLock(ctx context.Context, conn *sql.DB, tableName string, params lock.Params) (sql.Result, error)

	// DeleteLock deletes a lock from the database table.
	// It returns an error if there was a problem performing the deletion.
	DeleteLock(ctx context.Context, conn *sql.DB, tableName string, params lock.Params) error

	// Heartbeat sends a heartbeat to keep the lock alive. It returns an `sql.Result`
	// and an error.
	//
	// The lock parameters should be of type lock.Params, which has fields such as ID,
	// InstanceID, Timeout, and HeartbeatInterval.
	//
	// Example usage:
	//   res, err := dialect.Heartbeat(ctx, conn, tableName, params)
	//   if err != nil {
	//     return nil, fmt.Errorf("heartbeat: %w", err)
	//   }
	//
	//   rowsCount, _ := res.RowsAffected()
	//   if rowsCount == 0 {
	//     return nil, errors.ErrLockHasBeenLost
	//   }
	Heartbeat(ctx context.Context, conn *sql.DB, tableName string, params lock.Params) (sql.Result, error)
}

// PostgresDialect represents a dialect for working with Postgres databases.
// Implements the Dialect and MigrateDialect interface.
type PostgresDialect struct{}

// Migrate creates a new table if it doesn't exist already with the specified name.
// It is safe to use concurrently.
//
// SQL query is as follows:
//
//	CREATE TABLE IF NOT EXISTS {tableName} (
//		id          	VARCHAR(255) 	PRIMARY KEY,
//	  	locked_by		VARCHAR(255) 	NOT NULL,
//	  	locked_at		TIMESTAMPTZ  	NOT NULL,
//	  	last_heartbeat 	TIMESTAMPTZ		NOT NULL,
//	  	data 		    BYTEA
//	);
//
// Returns an error if there is an issue creating the table.
func (PostgresDialect) Migrate(ctx context.Context, conn *sql.DB, tableName string) error {
	_, err := conn.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
     	id             VARCHAR(255) PRIMARY KEY,
     	locked_by      VARCHAR(255) NOT NULL,
     	locked_at      TIMESTAMPTZ 	NOT NULL,
     	last_heartbeat TIMESTAMPTZ 	NOT NULL,
		data 		   BYTEA
	);`, tableName))
	if err != nil {
		return fmt.Errorf("create table %s: %w", tableName, err)
	}

	return nil
}

// UpsertLock inserts a new record into the specified table with the given parameters.
// If a record with the same primary key already exists, it updates the "locked_by"
// and "last_heartbeat" columns based on certain conditions.
// It is safe to use concurrently.
//
// Parameters:
// - ctx: The context for the database operation.
// - conn: The database connection.
// - tableName: The name of the table.
// - params: The lock parameters including ID, InstanceID, Timeout, and HeartbeatInterval.
//
// Returns:
// - sql.Result: The result of the database operation.
// - error: An error if there is an issue upserting the record.
//
// The SQL statement used for upserting the record is as follows:
//
//	INSERT INTO {tableName} (id, locked_by, locked_at, last_heartbeat, data)
//	VALUES ($1, $2, NOW(), NOW(), $3)
//	ON CONFLICT (id) DO UPDATE
//	SET locked_by = $2, data = $3, locked_at = NOW(), last_heartbeat = NOW()
//	WHERE locks.last_heartbeat < NOW() - ($4 * interval '1 ms');
//
// The function returns an error if there is an issue executing the database query.
func (PostgresDialect) UpsertLock(
	ctx context.Context,
	conn *sql.DB,
	tableName string,
	params lock.Params,
) (sql.Result, error) {
	res, err := conn.ExecContext(ctx, fmt.Sprintf(`
            INSERT INTO %s as locks (id, locked_by, locked_at, last_heartbeat, data)
            VALUES ($1, $2, NOW(), NOW(), $3)
            ON CONFLICT (id) DO UPDATE
            SET locked_by = $2, data = $3, locked_at = NOW(), last_heartbeat = NOW()
            WHERE locks.last_heartbeat < NOW() - ($4 * interval '1 ms');
        `, tableName),
		params.ID, params.InstanceID, params.Data, params.Timeout.Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("upsert to %s: %w", tableName, err)
	}

	return res, nil
}

// DeleteLock deletes records from the specified table where the "locked_by" column matches the given instance ID.
// It is safe to use concurrently.
//
// Parameters:
// - ctx: The context for the database operation.
// - conn: The database connection.
// - tableName: The name of the table.
// - params: The lock parameters including InstanceID.
//
// Returns:
// - error: An error if there is an issue deleting the records.
//
// The SQL statement used for deleting the records is as follows:
//
//	DELETE FROM {tableName} WHERE locked_by = $1
//
// The value for the placeholder in the SQL statement is provided by the InstanceID parameter.
// The function returns an error if there is an issue executing the database query.
func (PostgresDialect) DeleteLock(ctx context.Context, conn *sql.DB, tableName string, params lock.Params) error {
	_, err := conn.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE locked_by = $1;`, tableName),
		params.InstanceID,
	)
	if err != nil {
		return fmt.Errorf("delete from %s: %w", tableName, err)
	}

	return nil
}

// Heartbeat updates the last_heartbeat column of the specified table for the given ID and locked_by value.
// It is safe to use concurrently.
//
// Parameters:
// - ctx: The context for the database operation.
// - conn: The database connection.
// - tableName: The name of the table.
// - params: The lock parameters including ID and InstanceID.
//
// Returns:
// - sql.Result: The result of the database operation.
// - error: An error if there is an issue updating the last_heartbeat column.
//
// The SQL statement used for updating the last_heartbeat column is as follows:
//
//	UPDATE {tableName} SET last_heartbeat = NOW() WHERE id = $1 AND locked_by = $2
//
// The values for the placeholders in the SQL statement are provided by the params argument.
// The function returns an error if there is an issue executing the database query.
func (PostgresDialect) Heartbeat(
	ctx context.Context,
	conn *sql.DB,
	tableName string,
	params lock.Params,
) (sql.Result, error) {
	res, err := conn.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET last_heartbeat = NOW() WHERE id = $1 AND locked_by = $2;`, tableName),
		params.ID,
		params.InstanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("update last_heartbeat in %s: %w", tableName, err)
	}

	return res, nil
}

// MysqlDialect represents a dialect for working with MySQL databases.
// Implements the Dialect and MigrateDialect interface.
type MysqlDialect struct{}

// Migrate creates a table in the MySQL database if it does not already exist.
// It is safe to use concurrently.
//
// SQL query is as follows:
//
//	CREATE TABLE IF NOT EXISTS {tableName} (
//		id				VARCHAR(255) PRIMARY KEY,
//		locked_by		VARCHAR(255) NOT NULL,
//		locked_at		DATETIME(3)  NOT NULL,
//		last_heartbeat 	DATETIME(3)  NOT NULL,
//		data			BLOB
//	);
//
// Returns an error if there is an issue creating the table.
func (MysqlDialect) Migrate(ctx context.Context, conn *sql.DB, tableName string) error {
	_, err := conn.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
     	id             	VARCHAR(255) PRIMARY KEY,
     	locked_by      	VARCHAR(255) NOT NULL,
     	locked_at      	DATETIME(3)  NOT NULL,
     	last_heartbeat 	DATETIME(3)  NOT NULL,
		data			BLOB
	);`, tableName))
	if err != nil {
		return fmt.Errorf("create table %s: %w", tableName, err)
	}

	return nil
}

// UpsertLock inserts a new record into the specified table with the given parameters.
// If a record with the same primary key already exists, it updates the "locked_by"
// and "last_heartbeat" columns based on certain conditions.
// It is safe to use concurrently.
//
// Parameters:
// - ctx: The context for the database operation.
// - conn: The database connection.
// - tableName: The name of the table.
// - params: The lock parameters including ID, InstanceID, Timeout, and HeartbeatInterval.
//
// Returns:
// - sql.Result: The result of the database operation.
// - error: An error if there is an issue upserting the record.
//
// The SQL statement used for upserting the record is as follows:
//
//	INSERT INTO {tableName} (id, locked_by, locked_at, last_heartbeat)
//	VALUES (?, ?, NOW(3),NOW(3), ?)
//	ON DUPLICATE KEY UPDATE
//	locked_by = IF(last_heartbeat < (NOW(3) - INTERVAL ?*1000 MICROSECOND), VALUES(locked_by), locked_by),
//	last_heartbeat = IF(last_heartbeat < (NOW(3) - INTERVAL ?*1000 MICROSECOND), VALUES(last_heartbeat), last_heartbeat),
//	data = IF(last_heartbeat < (NOW(3) - INTERVAL ?*1000 MICROSECOND), VALUES(data), data);
//
// The function returns an error if there is an issue executing the database query.
func (MysqlDialect) UpsertLock(
	ctx context.Context,
	conn *sql.DB,
	tableName string,
	params lock.Params,
) (sql.Result, error) {
	ms := params.Timeout.Milliseconds()

	res, err := conn.ExecContext(ctx, fmt.Sprintf(`
            INSERT INTO %s (id, locked_by, locked_at, last_heartbeat, data)
            VALUES (?, ?, NOW(3),NOW(3), ?)
            ON DUPLICATE KEY UPDATE
			locked_by = IF(last_heartbeat < (NOW(3) - INTERVAL ?*1000 MICROSECOND), VALUES(locked_by), locked_by),
			last_heartbeat = IF(last_heartbeat < (NOW(3) - INTERVAL ?*1000 MICROSECOND), VALUES(last_heartbeat), last_heartbeat),
			data = IF(last_heartbeat < (NOW(3) - INTERVAL ?*1000 MICROSECOND), VALUES(data), data);
        `, tableName),
		params.ID, params.InstanceID, params.Data, ms, ms, ms)
	if err != nil {
		return nil, fmt.Errorf("upsert to %s: %w", tableName, err)
	}

	return res, nil
}

// DeleteLock deletes records from the specified table where the "locked_by" column matches the given instance ID.
// It is safe to use concurrently.
//
// Parameters:
// - ctx: The context for the database operation.
// - conn: The database connection.
// - tableName: The name of the table.
// - params: The lock parameters including InstanceID.
//
// Returns:
// - error: An error if there is an issue deleting the records.
//
// The SQL statement used for deleting the records is as follows:
//
//	DELETE FROM {tableName} WHERE locked_by = ?
//
// The function returns an error if there is an issue executing the database query.
func (MysqlDialect) DeleteLock(ctx context.Context, conn *sql.DB, tableName string, params lock.Params) error {
	_, err := conn.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE locked_by = ?;`, tableName),
		params.InstanceID,
	)
	if err != nil {
		return fmt.Errorf("delete from %s: %w", tableName, err)
	}

	return nil
}

// Heartbeat updates the last_heartbeat column of the specified table for the given ID and locked_by value.
// It is safe to use concurrently.
//
// Parameters:
// - ctx: The context for the database operation.
// - conn: The database connection.
// - tableName: The name of the table.
// - params: The lock parameters including ID and InstanceID.
//
// Returns:
// - sql.Result: The result of the database operation.
// - error: An error if there is an issue updating the last_heartbeat column.
//
// The SQL statement used for updating the last_heartbeat column is as follows:
//
//	UPDATE {tableName} SET last_heartbeat = NOW(3) WHERE id = ? AND locked_by = ?
//
// The function returns an error if there is an issue executing the database query.
func (MysqlDialect) Heartbeat(
	ctx context.Context,
	conn *sql.DB,
	tableName string,
	params lock.Params,
) (sql.Result, error) {
	res, err := conn.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET last_heartbeat = NOW(3) WHERE id = ? AND locked_by = ?`, tableName),
		params.ID,
		params.InstanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("update last_heartbeat in %s: %w", tableName, err)
	}

	return res, nil
}

// MariadbDialect represents a dialect for working with MariaDB databases.
// Same as MysqlDialect
// Implements the Dialect and MigrateDialect interface.
type MariadbDialect struct {
	MysqlDialect
}

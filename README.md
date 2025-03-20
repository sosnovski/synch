# Synch [![Go Reference](https://pkg.go.dev/badge/github.com/sosnovski/synch.svg)](https://pkg.go.dev/github.com/sosnovski/synch)

The synch library represents a compact toolset of synchronization primitives, devised to streamline the process of addressing complex challenges, such as distributed locking or signal handling in concurrent systems.

[![CI](https://github.com/sosnovski/synch/actions/workflows/ci.yml/badge.svg?&event=push)](https://github.com/sosnovski/synch/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sosnovski/synch)](https://goreportcard.com/report/github.com/sosnovski/synch)
[![codecov](https://codecov.io/gh/sosnovski/synch/graph/badge.svg?token=ECXT65ROPR)](https://codecov.io/gh/sosnovski/synch)

## Getting Started
To use the Synch, you must make sure it is present in $GOPATH or in your vendor directory.
```bash
$ go get -u github.com/sosnovski/synch
```

# Locker
The **Synch** library includes the locker package, which brings the capabilities of managing distributed locks, constructed atop various databases such as Postgres, MySQL, Redis, and others.

## Use cases
In distributed systems where several processing units need to carry out operations on specific resources like campaigns or customers, this client creates exclusive locks to prevent concurrent modifications. Thus, it provides a straightforward solution to achieve distributed locking.

Additionally, the lock client can be instrumental in leader election. It facilitates the selection of a single host as the leader for smooth operations. In case the chosen leader fails, another host assumes the leader's role within a user-defined lease period.

To sum up, this lock client is an essential tool for managing complex situations in distributed systems, such as concurrency issues and leader election.

### Supported Drivers
1. **SQL**  ✅
2. **Redis** ✅

### Supported dialects for SQL driver
1. ✅ **Postgres**
2. ✅ **Mysql**    
3. ✅ **MariaDB**    

## Examples

### Create a new lock

```go
import (
	"database/sql"

	_ "github.com/lib/pq"
	sqllocker "github.com/sosnovski/synch/locker/sql"
	"github.com/sosnovski/synch/locker"
)

conn, err := sql.Open("postgres", "user=postgres password=secret dbname=mydb")
if err != nil {
	// handle error
}

driver, err := sqllocker.NewDriver(
	conn,
	sqldriver.PostgresDialect{},
	WithAutoMigration(true),
	WithTableName("locks_v1"),
)
if err != nil {
	// handle error
}

locker, err := locker.New(driver)
if err != nil {
	// handle error
}

// use locker object
```

### Try to acquire a lock
This method tries to take the lock, and if it is already taken by someone, it immediately returns an error.

```go
lock, err := locker.TryLock(ctx, "my_lock_id")
if err != nil {
	// handle error
}
defer lock.Close(ctx) // Remember to always close the lock when done
```

### Try to acquire a lock and execute anonymous function
This method does the same thing as TryLock, but it is convenient to use it together with an anonymous function.
The lock is released automatically after exiting the anonymous function.

```go
// Wrap some application logic with a lock
err := locker.TryLockDo(ctx, "my_lock_id", func(_ context.Context) error {  
	// do something  
	return nil // or error
})  
if err != nil {  
	// handle error
}
```

### Wait to acquire a lock 
This is a blocking code that will wait until the lock can be taken. 
You can set a timeout using context.WithTimeout or context.WithDealine.

```go
lock, err := locker.WaitLock(ctx, "my_lock_id", time.Second)
if err != nil {
	// handle error
}
defer lock.Close(ctx) // Remember to always close the lock when done
```

### Wait to acquire a lock and execute anonymous function 
This method does the same thing as WaitLock, but it is convenient to use it together with an anonymous function.
The lock is released automatically after exiting the anonymous function.

```go
// Wrap some application logic with a lock and wait when lock is not available
err := locker.WaitLockDo(ctx, "my_lock_id", time.Second, func(_ context.Context) error { 
	// do something
	return nil  
})
if err != nil {
	// handle error
}
```

### Wait to acquire a lock and execute anonymous function with custom wait context
This method does the same thing as WaitLock, but it is convenient to use it together with an anonymous function.
The key difference from WaitLockDo, that this method supports a custom context for waiting for the lock to be released.
This can be useful when you need to wait for a lock to be released for a limited time, 
but perform an anonymous function longer or for an unlimited time.
The lock is released automatically after exiting the anonymous function.

```go
// Wait acquire lock with timeout for 1 minute
waitLockCtx, cancel := context.WithDeadline(ctx, time.Minute)
defer cancel()

// Wrap some application logic with a lock and wait when lock is not available
err := locker.WaitLockDoWithCustomCtx(ctx, waitLockCtx, "my_lock_id", time.Second, func(_ context.Context) error { 
	// do something
	return nil  
})
if err != nil {
	// handle error
}
```

To learn more about the Synch project, reviewing the documentation comments in each file may provide more context. Always remember to close the Lock when it's not going to be used anymore.

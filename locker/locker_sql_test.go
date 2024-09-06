package locker

import (
	"context"
	stdSql "database/sql"
	stdErrors "errors"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mariadb"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/sosnovski/synch/locker/errors"
	"github.com/sosnovski/synch/locker/lock"
	"github.com/sosnovski/synch/locker/sql"
)

var errTest = stdErrors.New("test error")

const (
	containerStartupTimeout  = 5 * time.Second
	containerOccurrenceCount = 2
)

type dialectWithoutMigration struct {
	dialect sql.Dialect
}

//nolint:wrapcheck //its only for test
func (d dialectWithoutMigration) UpsertLock(
	ctx context.Context,
	conn *stdSql.DB,
	tableName string,
	params lock.Params,
) (stdSql.Result, error) {
	return d.dialect.UpsertLock(ctx, conn, tableName, params)
}

//nolint:wrapcheck //its only for test
func (d dialectWithoutMigration) DeleteLock(
	ctx context.Context,
	conn *stdSql.DB,
	tableName string,
	params lock.Params,
) error {
	return d.dialect.DeleteLock(ctx, conn, tableName, params)
}

//nolint:wrapcheck //its only for test
func (d dialectWithoutMigration) Heartbeat(
	ctx context.Context,
	conn *stdSql.DB,
	tableName string,
	params lock.Params,
) (stdSql.Result, error) {
	return d.dialect.Heartbeat(ctx, conn, tableName, params)
}

type terminator interface {
	Terminate(ctx context.Context) error
}

type sqlContainer struct {
	terminator       terminator
	connectionString string
}

func createPostgresContainer(ctx context.Context) (*sqlContainer, error) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:16",
		postgres.WithDatabase("test-db"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(containerOccurrenceCount).WithStartupTimeout(containerStartupTimeout)),
	)
	if err != nil {
		return nil, fmt.Errorf("run postgres container: %w", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("could not create postgres connection string: %w", err)
	}

	return &sqlContainer{
		terminator:       pgContainer,
		connectionString: connStr,
	}, nil
}

func createMysqlContainer(ctx context.Context) (*sqlContainer, error) {
	mysqlContainer, err := mysql.Run(ctx,
		"mysql:9",
		mysql.WithDatabase("test-db"),
		mysql.WithUsername("mysql"),
		mysql.WithPassword("mysql"),
	)
	if err != nil {
		return nil, fmt.Errorf("run mysql container: %w", err)
	}

	connStr, err := mysqlContainer.ConnectionString(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not create mysql connection string: %w", err)
	}

	return &sqlContainer{
		terminator:       mysqlContainer,
		connectionString: connStr,
	}, nil
}

func createMariaDBContainer(ctx context.Context) (*sqlContainer, error) {
	mysqlContainer, err := mariadb.Run(ctx,
		"mariadb:11",
		mariadb.WithDatabase("test-db"),
		mariadb.WithUsername("mysql"),
		mariadb.WithPassword("mysql"),
	)
	if err != nil {
		return nil, fmt.Errorf("run mysql container: %w", err)
	}

	connStr, err := mysqlContainer.ConnectionString(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not create mariadb connection string: %w", err)
	}

	return &sqlContainer{
		terminator:       mysqlContainer,
		connectionString: connStr,
	}, nil
}

type LockerSQLTestSuite struct {
	suite.Suite
	sqlContainer    *sqlContainer
	ctx             context.Context
	conn            *stdSql.DB
	dialect         sql.Dialect
	containerCreate func(context.Context) (*sqlContainer, error)
	connectionFunc  func(connectionString string) (*stdSql.DB, error)
	stealQuery      string
}

func (s *LockerSQLTestSuite) SetupSuite() {
	s.ctx = context.Background()

	container, err := s.containerCreate(s.ctx)
	if err != nil {
		log.Fatal(err)
	}

	s.sqlContainer = container

	conn, err := s.connectionFunc(s.sqlContainer.connectionString)
	s.Require().NoError(err)

	s.conn = conn
}

func (s *LockerSQLTestSuite) TearDownSuite() {
	if err := s.conn.Close(); err != nil {
		log.Fatalf("could not close connection: %v", err)
	}

	if err := s.sqlContainer.terminator.Terminate(s.ctx); err != nil {
		log.Fatalf("error terminating container: %s", err)
	}
}

func (s *LockerSQLTestSuite) TestTryLockWithoutMigrate() {
	t := s.T()

	s.TearDownSuite()
	s.SetupSuite()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(false))
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	_, err = locker.TryLock(s.ctx, lockID)
	require.ErrorContains(t, err, "exist")
}

func (s *LockerSQLTestSuite) TestTryLockWithMigrate() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NoError(t, l.Close(s.ctx))
}

func (s *LockerSQLTestSuite) TestWithMigrateContextCanceled() {
	t := s.T()

	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	d, err := sql.NewDriver(s.conn,
		s.dialect,
		sql.WithAutoMigration(true),
		sql.WithMigrationContext(ctx),
	)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, d)
}

func (s *LockerSQLTestSuite) TestWithNilMigrateContextCanceled() {
	t := s.T()

	d, err := sql.NewDriver(s.conn,
		s.dialect,
		sql.WithAutoMigration(true),
		sql.WithMigrationContext(nil), //nolint: staticcheck //case it is a unit-test
	)
	require.ErrorIs(t, err, sql.ErrMigrationContextMustBeSet)
	require.Nil(t, d)
}

func (s *LockerSQLTestSuite) TestMigrateWithNonImplementMigrateDialect() {
	t := s.T()

	d, err := sql.NewDriver(s.conn, dialectWithoutMigration{dialect: s.dialect})
	require.NoError(t, err)
	require.NotNil(t, d)

	err = d.Migrate(s.ctx)
	require.ErrorIs(t, err, sql.ErrDialectNotImplementMigrate)
}

func (s *LockerSQLTestSuite) TestTryLockAndWaitClose() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NotNil(t, l)
	time.AfterFunc(defaultHeartbeatInterval, func() {
		require.NoError(t, l.Close(s.ctx))
	})

	now := time.Now()

	<-l.ShutdownCtx().Done()
	require.NoError(t, l.ShutdownCtx().Err())
	require.Less(t, time.Since(now), defaultHeartbeatInterval+150*time.Millisecond)
}

func (s *LockerSQLTestSuite) TestTryLocAndCheckLockFields() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NoError(t, l.Close(s.ctx))

	require.Equal(t, lockID, l.ID())
	require.True(t, strings.HasPrefix(l.InstanceID(), defaultPrefix))
	require.Equal(t, defaultLockTimeout, l.Timeout())
	require.Equal(t, defaultHeartbeatInterval, l.HeartbeatInterval())
}

func (s *LockerSQLTestSuite) TestTryLocWithData() {
	t := s.T()

	var (
		lockID = fmt.Sprintf("test-lock-%d", time.Now().UnixNano())
		data   = []byte("some data")
	)

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID, lock.WithData(data))
	require.NoError(t, err)
	require.NoError(t, l.Close(s.ctx))

	require.Equal(t, lockID, l.ID())
	require.True(t, strings.HasPrefix(l.InstanceID(), defaultPrefix))
	require.Equal(t, data, l.Data())
}

func (s *LockerSQLTestSuite) TestTryLocWithGroupID() {
	t := s.T()

	var (
		lockID  = fmt.Sprintf("test-lock-%d", time.Now().UnixNano())
		groupID = "test-group-id"
	)

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID, lock.WithGroupID(groupID))
	require.NoError(t, err)
	require.NoError(t, l.Close(s.ctx))

	require.Equal(t, lockID, l.ID())
	require.True(t, strings.HasPrefix(l.InstanceID(), defaultPrefix))
	require.Equal(t, groupID, l.GroupID())
}

func (s *LockerSQLTestSuite) TestCreateLockerWithNilDriver() {
	t := s.T()

	locker, err := New(nil)
	require.ErrorIs(t, err, errors.ErrDriverIsNil)
	require.Nil(t, locker)
}

func (s *LockerSQLTestSuite) TestCreateDriverWithNilConnection() {
	t := s.T()

	d, err := sql.NewDriver(nil, s.dialect)
	require.ErrorIs(t, err, sql.ErrConnIsNil)
	require.Nil(t, d)
}

func (s *LockerSQLTestSuite) TestTryLockWithLockParamsInstanceID() {
	t := s.T()

	var (
		lockID     = fmt.Sprintf("test-lock-%d", time.Now().UnixNano())
		instanceID = fmt.Sprintf("test-instance-%d", time.Now().UnixNano())
	)

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID, lock.WithInstanceID(instanceID))
	require.NoError(t, err)
	require.NotNil(t, locker)
	require.NoError(t, l.Close(s.ctx))

	require.Equal(t, l.ID(), lockID)
	require.Equal(t, l.InstanceID(), instanceID)
}

func (s *LockerSQLTestSuite) TestTryLockWithInvalidHeartbeatInterval() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID,
		lock.WithTimeout(4*time.Second),
		lock.WithHeartbeatInterval(3*time.Second),
	)
	require.ErrorIs(t, err, errors.ErrLockHeartbeatIntervalToHigh)
	require.Nil(t, l)
}

func (s *LockerSQLTestSuite) TestTryLockWithEmptyLockParamsInstanceID() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID, lock.WithInstanceID(""))
	require.ErrorIs(t, err, errors.ErrInstanceIDIsEmpty)
	require.Nil(t, l)
}

func (s *LockerSQLTestSuite) TestCreateLockerWithInstanceID() {
	t := s.T()

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)
	require.NotNil(t, d)

	var (
		lockID     = fmt.Sprintf("test-lock-%d", time.Now().UnixNano())
		instanceID = fmt.Sprintf("test-instance-%d", time.Now().UnixNano())
	)

	locker, err := New(d, WithDefaultInstanceID(instanceID))
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NotNil(t, locker)
	require.NoError(t, l.Close(s.ctx))

	require.Equal(t, l.ID(), lockID)
	require.Equal(t, l.InstanceID(), instanceID)
}

func (s *LockerSQLTestSuite) TestCreateLockerWithEmptyInstanceID() {
	t := s.T()

	d, err := sql.NewDriver(s.conn, s.dialect)
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d, WithDefaultInstanceID(""))
	require.ErrorIs(t, err, errors.ErrInstanceIDIsEmpty)
	require.Nil(t, locker)
}

func (s *LockerSQLTestSuite) TestCreateDriverWithNilDialect() {
	t := s.T()

	d, err := sql.NewDriver(s.conn, nil)
	require.ErrorIs(t, err, sql.ErrDialectIsNil)
	require.Nil(t, d)
}

func (s *LockerSQLTestSuite) TestHeartbeatFailedByShutdownDB() {
	t := s.T()

	defer s.SetupSuite()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)

	s.TearDownSuite()

	now := time.Now()

	<-l.ShutdownCtx().Done()
	err = l.ShutdownCtx().Err()
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, "failed to send heartbeat")
	require.Less(t, time.Since(now), defaultHeartbeatInterval+150*time.Millisecond)
}

func (s *LockerSQLTestSuite) TestHeartbeatFailedByNoAffectedRows() {
	t := s.T()

	var (
		lockID    = fmt.Sprintf("test-lock-%d", time.Now().UnixNano())
		tableName = "test_locks"
	)

	d, err := sql.NewDriver(s.conn,
		s.dialect,
		sql.WithAutoMigration(true),
		sql.WithTableName(tableName),
	)
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)

	_, err = s.conn.Exec(fmt.Sprintf(s.stealQuery, tableName), lockID)
	require.NoError(t, err)

	now := time.Now()

	<-l.ShutdownCtx().Done()
	err = l.ShutdownCtx().Err()
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, err, errors.ErrLockHasBeenLost)
	require.Less(t, time.Since(now), defaultHeartbeatInterval+150*time.Millisecond)
}

func (s *LockerSQLTestSuite) TestTryLockDoWaitDefaultInterval() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	err = locker.TryLockDo(s.ctx, lockID, func(_ context.Context) error {
		<-time.After(defaultHeartbeatInterval)

		return nil
	})

	now := time.Now()

	require.NoError(t, err)
	require.Less(t, time.Since(now), defaultHeartbeatInterval+150*time.Millisecond)
}

func (s *LockerSQLTestSuite) TestTryLockDoWithErrorOnClose() {
	t := s.T()

	defer s.SetupSuite()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	err = locker.TryLockDo(s.ctx, lockID, func(_ context.Context) error {
		s.TearDownSuite()

		return nil
	})

	require.ErrorContains(t, err, "close lock")
}

func (s *LockerSQLTestSuite) TestTryLockDoWithReturnNilError() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	err = locker.TryLockDo(s.ctx, lockID, func(_ context.Context) error {
		return nil
	})

	require.NoError(t, err)
}

func (s *LockerSQLTestSuite) TestTryLockDoReturnError() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	err = locker.TryLockDo(s.ctx, lockID, func(_ context.Context) error {
		return fmt.Errorf("do err: %w", errTest)
	})

	require.ErrorIs(t, err, errTest)
}

func (s *LockerSQLTestSuite) TestTryLockDoWithLoopClosedByContext() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	ctx, cancel := context.WithTimeout(s.ctx, time.Second)
	defer cancel()

	err = locker.TryLockDo(ctx, lockID, func(_ context.Context) error {
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	})

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

// nolint: err113 //its need for test
func (s *LockerSQLTestSuite) TestTryLockDoWithLoopClosedByContextReturnError() {
	t := s.T()

	var (
		lockID  = fmt.Sprintf("test-lock-%d", time.Now().UnixNano())
		testErr = stdErrors.New("test error")
	)

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	ctx, cancel := context.WithTimeout(s.ctx, time.Second)
	defer cancel()

	err = locker.TryLockDo(ctx, lockID, func(_ context.Context) error {
		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("%w: %w", ctx.Err(), testErr)
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	})

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.ErrorIs(t, err, testErr)
}

func (s *LockerSQLTestSuite) TestTryLockDoWithLockAlreadyHeld() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NotNil(t, l)

	defer func() {
		require.NoError(t, l.Close(s.ctx))
	}()

	err = locker.TryLockDo(s.ctx, lockID, func(_ context.Context) error {
		return nil
	})

	require.ErrorIs(t, err, errors.ErrLockAlreadyHeld)
}

func (s *LockerSQLTestSuite) TestWaitLock() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	lock2, err := locker.WaitLock(s.ctx, lockID, time.Second)
	require.NoError(t, err)
	require.NotNil(t, lock2)
	require.NoError(t, lock2.Close(s.ctx))
}

func (s *LockerSQLTestSuite) TestWaitLockWithLockAlreadyHeld() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NotNil(t, l)

	time.AfterFunc(defaultHeartbeatInterval, func() {
		require.NoError(t, l.Close(s.ctx))
	})

	now := time.Now()
	lock2, err := locker.WaitLock(s.ctx, lockID, time.Second)
	require.NoError(t, err)
	require.NotNil(t, lock2)
	require.NoError(t, lock2.Close(s.ctx))

	require.Less(t, time.Since(now), defaultHeartbeatInterval*2+150*time.Millisecond)
}

func (s *LockerSQLTestSuite) TestWaitLockWithContextCanceled() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NotNil(t, l)

	time.AfterFunc(defaultHeartbeatInterval, func() {
		require.NoError(t, l.Close(s.ctx))
	})

	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	lock2, err := locker.WaitLock(ctx, lockID, time.Second)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, lock2)
}

func (s *LockerSQLTestSuite) TestTryLockWithDoubleClose() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NotNil(t, l)
	require.NoError(t, l.Close(s.ctx))
	require.NoError(t, l.Close(s.ctx))
}

func (s *LockerSQLTestSuite) TestWaitLockWithContextCanceledAfterIterate() {
	t := s.T()

	var (
		lockID  = fmt.Sprintf("test-lock-%d", time.Now().UnixNano())
		iterate = 0
	)

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(s.ctx)
	locker, err := New(d, WithOnWaitIterateError(func(_ context.Context, _ error) {
		if iterate == 0 {
			cancel()
		}

		iterate++
	}))
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NotNil(t, l)

	time.AfterFunc(defaultHeartbeatInterval, func() {
		require.NoError(t, l.Close(s.ctx))
	})

	lock2, err := locker.WaitLock(ctx, lockID, time.Second)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, lock2)
}

func (s *LockerSQLTestSuite) TestWaitLockDo() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NotNil(t, l)

	time.AfterFunc(defaultHeartbeatInterval, func() {
		require.NoError(t, l.Close(s.ctx))
	})

	now := time.Now()

	err = locker.WaitLockDo(s.ctx, lockID, time.Second, func(_ context.Context) error {
		return nil
	})
	require.NoError(t, err)
	require.Less(t, time.Since(now), defaultHeartbeatInterval*2+150*time.Millisecond)
}

func (s *LockerSQLTestSuite) TestWaitLockDoWithContextCanceled() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := sql.NewDriver(s.conn, s.dialect, sql.WithAutoMigration(true))
	require.NoError(t, err)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NotNil(t, l)

	time.AfterFunc(defaultHeartbeatInterval, func() {
		require.NoError(t, l.Close(s.ctx))
	})

	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	err = locker.WaitLockDo(ctx, lockID, time.Second, func(_ context.Context) error {
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
}

func TestSQLLockerPostgresDialectSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &LockerSQLTestSuite{
		dialect:         sql.PostgresDialect{},
		containerCreate: createPostgresContainer,
		connectionFunc: func(connectionString string) (*stdSql.DB, error) {
			return stdSql.Open("postgres", connectionString)
		},
		stealQuery: `UPDATE %s SET locked_by = 'some_id' WHERE id = $1`,
	})
}

func TestSQLLockerMysqlDialectSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &LockerSQLTestSuite{
		dialect:         sql.MysqlDialect{},
		containerCreate: createMysqlContainer,
		connectionFunc: func(connectionString string) (*stdSql.DB, error) {
			return stdSql.Open("mysql", connectionString)
		},
		stealQuery: `UPDATE %s SET locked_by = 'some_id' WHERE id = ?`,
	})
}

func TestSQLLockerMariadbDialectSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &LockerSQLTestSuite{
		dialect:         sql.MariadbDialect{},
		containerCreate: createMariaDBContainer,
		connectionFunc: func(connectionString string) (*stdSql.DB, error) {
			return stdSql.Open("mysql", connectionString)
		},
		stealQuery: `UPDATE %s SET locked_by = 'some_id' WHERE id = ?`,
	})
}

package locker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sosnovski/synch/locker/errors"
	"github.com/sosnovski/synch/locker/lock"
	redisDriver "github.com/sosnovski/synch/locker/redis"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	redisCont "github.com/testcontainers/testcontainers-go/modules/redis"
)

type redisContainer struct {
	terminator       terminator
	connectionString string
}

func createRedisContainer(ctx context.Context) (*redisContainer, error) {
	container, err := redisCont.Run(ctx,
		"redis:7",
		redisCont.WithLogLevel(redisCont.LogLevelVerbose),
	)
	if err != nil {
		return nil, fmt.Errorf("run postgres container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not create redis connection string: %w", err)
	}

	return &redisContainer{
		terminator:       container,
		connectionString: connStr,
	}, nil
}

type LockerRedisTestSuite struct {
	suite.Suite
	container        *redisContainer
	client           *redis.Client
	ctx              context.Context
	containerCreate  func(context.Context) (*redisContainer, error)
	createClientFunc func(connectionString string) *redis.Client
}

func (s *LockerRedisTestSuite) SetupSuite() {
	s.ctx = context.Background()

	container, err := s.containerCreate(s.ctx)
	if err != nil {
		log.Fatal(err)
	}

	s.container = container
	s.client = s.createClientFunc(s.container.connectionString)
}

func (s *LockerRedisTestSuite) TearDownSuite() {
	if err := s.container.terminator.Terminate(s.ctx); err != nil {
		log.Fatalf("error terminating container: %s", err)
	}
}

func (s *LockerRedisTestSuite) TestClientIsNil() {
	t := s.T()

	s.TearDownSuite()
	s.SetupSuite()

	d, err := redisDriver.NewDriver(nil)
	require.Nil(t, d)
	require.ErrorIs(t, err, redisDriver.ErrClientIsNil)
}

func (s *LockerRedisTestSuite) TestNewDriverWithErrorOptions() {
	t := s.T()

	s.TearDownSuite()
	s.SetupSuite()

	d, err := redisDriver.NewDriver(s.client, redisDriver.WithPrefixLockKey(""))
	require.ErrorIs(t, err, redisDriver.ErrApplyOptions)
	require.ErrorIs(t, err, redisDriver.ErrPrefixLockKeyIsEmpty)
	require.Nil(t, d)
}

func (s *LockerRedisTestSuite) TestTryLock() {
	t := s.T()

	s.TearDownSuite()
	s.SetupSuite()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NoError(t, l.Close(s.ctx))
}

func (s *LockerRedisTestSuite) TestTryLockAndWaitClose() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
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

func (s *LockerRedisTestSuite) TestTryLocAndCheckLockFields() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
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

func (s *LockerRedisTestSuite) TestTryLocWithData() {
	t := s.T()

	var (
		lockID = fmt.Sprintf("test-lock-%d", time.Now().UnixNano())
		data   = []byte("some data")
	)

	d, err := redisDriver.NewDriver(s.client)
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

func (s *LockerRedisTestSuite) TestTryLocWithGroupID() {
	t := s.T()

	var (
		lockID  = fmt.Sprintf("test-lock-%d", time.Now().UnixNano())
		groupID = "test-group-id"
	)

	d, err := redisDriver.NewDriver(s.client)
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

func (s *LockerRedisTestSuite) TestTryLockWithLockParamsInstanceID() {
	t := s.T()

	var (
		lockID     = fmt.Sprintf("test-lock-%d", time.Now().UnixNano())
		instanceID = fmt.Sprintf("test-instance-%d", time.Now().UnixNano())
	)

	d, err := redisDriver.NewDriver(s.client)
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

func (s *LockerRedisTestSuite) TestTryLockWithInvalidHeartbeatInterval() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
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

func (s *LockerRedisTestSuite) TestHeartbeatFailedByShutdownDB() {
	t := s.T()

	defer s.SetupSuite()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

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

func (s *LockerRedisTestSuite) TestHeartbeatFailedByNoKey() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	const key = "test_key"

	d, err := redisDriver.NewDriver(s.client, redisDriver.WithPrefixLockKey(key))
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)

	cmd := s.client.HSet(s.ctx, fmt.Sprintf("%s:%s", key, lockID), "locked_by", "new_instance_id")
	require.NoError(t, cmd.Err())
	require.Equal(t, int64(0), cmd.Val())

	now := time.Now()

	<-l.ShutdownCtx().Done()
	err = l.ShutdownCtx().Err()
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, err, errors.ErrLockHasBeenLost)
	require.Less(t, time.Since(now), defaultHeartbeatInterval+150*time.Millisecond)
}

func (s *LockerRedisTestSuite) TestTryLockDoWaitDefaultInterval() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

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

func (s *LockerRedisTestSuite) TestTryLockDoWithErrorOnClose() {
	t := s.T()

	defer s.SetupSuite()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	err = locker.TryLockDo(s.ctx, lockID, func(_ context.Context) error {
		s.TearDownSuite()

		return nil
	})

	require.ErrorContains(t, err, "close lock")
}

func (s *LockerRedisTestSuite) TestTryLockDoWithReturnNilError() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	err = locker.TryLockDo(s.ctx, lockID, func(_ context.Context) error {
		return nil
	})

	require.NoError(t, err)
}

func (s *LockerRedisTestSuite) TestTryLockDoReturnError() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	err = locker.TryLockDo(s.ctx, lockID, func(_ context.Context) error {
		return fmt.Errorf("do err: %w", errTest)
	})

	require.ErrorIs(t, err, errTest)
}

func (s *LockerRedisTestSuite) TestTryLockDoWithLoopClosedByContext() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

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

// nolint: wrapcheck //its need for test
func (s *LockerRedisTestSuite) TestTryLockDoWithLoopClosedByContextReturnError() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	ctx, cancel := context.WithTimeout(s.ctx, time.Second)
	defer cancel()

	err = locker.TryLockDo(ctx, lockID, func(_ context.Context) error {
		for {
			select {
			case <-ctx.Done():
				return errTest
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	})

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.ErrorIs(t, err, errTest)
}

func (s *LockerRedisTestSuite) TestTryLockDoWithLockAlreadyHeld() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

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

func (s *LockerRedisTestSuite) TestWaitLock() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	lock2, err := locker.WaitLock(s.ctx, lockID, time.Second)
	require.NoError(t, err)
	require.NotNil(t, lock2)
	require.NoError(t, lock2.Close(s.ctx))
}

func (s *LockerRedisTestSuite) TestWaitLockWithLockAlreadyHeld() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
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
	lock2, err := locker.WaitLock(s.ctx, lockID, time.Second)
	require.NoError(t, err)
	require.NotNil(t, lock2)
	require.NoError(t, lock2.Close(s.ctx))

	require.Less(t, time.Since(now), defaultHeartbeatInterval*2+150*time.Millisecond)
}

func (s *LockerRedisTestSuite) TestWaitLockWithContextCanceled() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
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

	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	lock2, err := locker.WaitLock(ctx, lockID, time.Second)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, lock2)
}

func (s *LockerRedisTestSuite) TestTryLockWithDoubleClose() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NotNil(t, l)
	require.NoError(t, l.Close(s.ctx))
	require.NoError(t, l.Close(s.ctx))
}

func (s *LockerRedisTestSuite) TestWaitLockWithContextCanceledAfterIterate() {
	t := s.T()

	var (
		lockID  = fmt.Sprintf("test-lock-%d", time.Now().UnixNano())
		iterate = 0
	)

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

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

func (s *LockerRedisTestSuite) TestWaitLockDo() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
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

	err = locker.WaitLockDo(s.ctx, lockID, time.Second, func(_ context.Context) error {
		return nil
	})
	require.NoError(t, err)
	require.Less(t, time.Since(now), defaultHeartbeatInterval*2+150*time.Millisecond)
}

func (s *LockerRedisTestSuite) TestWaitLockDoWithWaitCtx() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
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

	err = locker.WaitLockDoWithWaitCtx(s.ctx, s.ctx, lockID, time.Second, func(_ context.Context) error {
		return nil
	})
	require.NoError(t, err)
	require.Less(t, time.Since(now), defaultHeartbeatInterval*2+150*time.Millisecond)
}

func (s *LockerRedisTestSuite) TestWaitLockDoWithWaitCtxCloseWaitCtx() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	lck, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NotNil(t, lck)

	waitCtx, cancel := context.WithCancel(s.ctx)

	time.AfterFunc(defaultHeartbeatInterval, func() {
		cancel()
		require.NoError(t, lck.Close(s.ctx))
	})

	now := time.Now()

	err = locker.WaitLockDoWithWaitCtx(s.ctx, waitCtx, lockID, time.Second, func(_ context.Context) error {
		return nil
	})

	<-lck.ShutdownCtx().Done()

	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, "wait lock")
	require.Less(t, time.Since(now), defaultHeartbeatInterval*2+150*time.Millisecond)
}

func (s *LockerRedisTestSuite) TestWaitLockDoWithWaitCtxCloseMainCtx() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NotNil(t, l)

	mainCtx, cancel := context.WithCancel(s.ctx)

	time.AfterFunc(defaultHeartbeatInterval, func() {
		require.NoError(t, l.Close(s.ctx))
		cancel()
	})

	now := time.Now()

	err = locker.WaitLockDoWithWaitCtx(mainCtx, s.ctx, lockID, time.Second, func(_ context.Context) error {
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(now), defaultHeartbeatInterval*2+150*time.Millisecond)
}

// nolint: wrapcheck //its need for test
func (s *LockerRedisTestSuite) TestWaitLockDoWithWaitCtxCloseMainCtxWithReturnError() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
	require.NoError(t, err)
	require.NotNil(t, d)

	locker, err := New(d)
	require.NoError(t, err)
	require.NotNil(t, locker)

	l, err := locker.TryLock(s.ctx, lockID)
	require.NoError(t, err)
	require.NotNil(t, l)

	mainCtx, cancel := context.WithCancel(s.ctx)

	time.AfterFunc(defaultHeartbeatInterval, func() {
		require.NoError(t, l.Close(s.ctx))
		cancel()
	})

	now := time.Now()

	err = locker.WaitLockDoWithWaitCtx(mainCtx, s.ctx, lockID, time.Second, func(_ context.Context) error {
		return errTest
	})
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, err, errTest)
	require.Less(t, time.Since(now), defaultHeartbeatInterval*2+150*time.Millisecond)
}

// nolint: wrapcheck //its need for test
func (s *LockerRedisTestSuite) TestWaitLockDoWithWaitCtxWithReturnError() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
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

	err = locker.WaitLockDoWithWaitCtx(s.ctx, s.ctx, lockID, time.Second, func(_ context.Context) error {
		return errTest
	})
	require.NotErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, err, errTest)
	require.Less(t, time.Since(now), defaultHeartbeatInterval*2+150*time.Millisecond)
}

func (s *LockerRedisTestSuite) TestWaitLockDoWithContextCanceled() {
	t := s.T()

	lockID := fmt.Sprintf("test-lock-%d", time.Now().UnixNano())

	d, err := redisDriver.NewDriver(s.client)
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

	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	err = locker.WaitLockDo(ctx, lockID, time.Second, func(_ context.Context) error {
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
}

func TestRedisLockerSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &LockerRedisTestSuite{
		containerCreate: createRedisContainer,
		createClientFunc: func(connectionString string) *redis.Client {
			return redis.NewClient(&redis.Options{
				Addr: strings.ReplaceAll(connectionString, "redis://", ""),
			})
		},
	})
}

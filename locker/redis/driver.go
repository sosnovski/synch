package redis

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sosnovski/synch/locker/errors"
	"github.com/sosnovski/synch/locker/lock"
	"github.com/sosnovski/synch/log"
)

const (
	defaultPrefixLockKey = "synch_locks" // defaultPrefixLockKey is the default name of the key used for managing locks.
	lockedByField        = "locked_by"
	groupIDField         = "group_id"
	dataField            = "data"
)

type Driver struct {
	log           Logger
	cli           *redis.Client
	prefixLockKey string
}

func NewDriver(cli *redis.Client, options ...Option) (*Driver, error) {
	driver := &Driver{
		log:           log.Noop{},
		cli:           cli,
		prefixLockKey: defaultPrefixLockKey,
	}

	if cli == nil {
		return nil, ErrClientIsNil
	}

	for _, option := range options {
		if err := option(driver); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrApplyOptions, err)
		}
	}

	return driver, nil
}

func (d *Driver) TryLock(ctx context.Context, params lock.Params) (*lock.Lock, error) {
	key := d.key(params)

	var cmds []*redis.BoolCmd

	_, err := d.cli.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		cmds = append(cmds,
			pipe.HSetNX(ctx, key, lockedByField, params.InstanceID),
			pipe.HSetNX(ctx, key, groupIDField, params.GroupID),
			pipe.HSetNX(ctx, key, dataField, params.Data),
			pipe.Expire(ctx, key, params.Timeout),
		)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("tx pipelined: %w", err)
	}

	for _, cmd := range cmds {
		if !cmd.Val() {
			return nil, errors.ErrLockAlreadyHeld
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	shutdownCtx, shutdown := d.startHeartbeat(wg, key, params)

	shutdownFun := func(ctx context.Context) error {
		shutdown(nil)
		wg.Wait()

		if err := d.deleteLock(ctx, key, params); err != nil {
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

// startHeartbeat starts the heartbeat goroutine to send heartbeats at a specified interval.
func (d *Driver) startHeartbeat(wg *sync.WaitGroup, key string, params lock.Params) (context.Context, context.CancelCauseFunc) {
	shutdownCtx, shutdown := context.WithCancelCause(context.Background())

	go func() {
		defer wg.Done()

		ticker := time.NewTicker(params.HeartbeatInterval)
		defer ticker.Stop()

		for {
			if err := d.sendHeartbeat(shutdownCtx, key, params); err != nil {
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

func (d *Driver) sendHeartbeat(ctx context.Context, key string, params lock.Params) error {
	script := `
		local locked_by = redis.call("HGET", KEYS[1], ARGV[1])
		if locked_by == ARGV[2] then
			return redis.call("EXPIRE", KEYS[1], ARGV[3])
		else
			return 0
		end
	`

	res, err := d.cli.Eval(ctx,
		script,
		[]string{key},
		lockedByField,
		params.InstanceID,
		int(params.Timeout.Seconds()),
	).Result()
	if err != nil {
		return fmt.Errorf("eval: %w", err)
	}

	count, _ := res.(int64)
	if count != 1 {
		return errors.ErrLockHasBeenLost
	}

	return nil
}

func (d *Driver) deleteLock(ctx context.Context, key string, params lock.Params) error {
	script := `
		local locked_by = redis.call("HGET", KEYS[1], ARGV[1])
		if locked_by == ARGV[2] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`

	err := d.cli.Eval(ctx, script, []string{key}, lockedByField, params.InstanceID).Err()
	if err != nil {
		return fmt.Errorf("eval: %w", err)
	}

	return nil
}

func (d *Driver) key(params lock.Params) string {
	return fmt.Sprintf("%s:%s", d.prefixLockKey, params.ID)
}

package redis

import (
	"errors"
	"testing"

	"github.com/sosnovski/synch/log"
)

func TestApplyWithLogger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		log  log.Logger
		want error
		name string
	}{
		{
			name: "empty logger",
			log:  nil,
			want: ErrLoggerIsNil,
		},
		{
			name: "not empty logger",
			log:  log.Noop{},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := &Driver{}
			if got := WithLogger(tt.log)(d); !errors.Is(tt.want, got) {
				t.Errorf("apply WithLogger() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyWithPrefixLockKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want    error
		name    string
		lockKey string
	}{
		{
			name:    "empty prefix lock key",
			lockKey: "",
			want:    ErrPrefixLockKeyIsEmpty,
		},
		{
			name:    "not empty prefix lock key",
			lockKey: "test-lock-key",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := &Driver{}
			if got := WithPrefixLockKey(tt.lockKey)(d); !errors.Is(tt.want, got) {
				t.Errorf("apply WithPrefixLockKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

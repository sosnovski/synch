package locker

import (
	"context"
	stdErrors "errors"
	"testing"

	"github.com/sosnovski/synch/locker/errors"
	"github.com/sosnovski/synch/log"
)

func TestApplyWithDefaultInstanceID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want       error
		name       string
		instanceID string
	}{
		{
			name:       "empty instanceID",
			instanceID: "",
			want:       errors.ErrInstanceIDIsEmpty,
		},
		{
			name:       "not empty instanceID",
			instanceID: "test-instance-id",
			want:       nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := WithDefaultInstanceID(tt.instanceID)(&Locker{}); !stdErrors.Is(tt.want, got) {
				t.Errorf("apply WithDefaultInstanceID() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
			want: errors.ErrLoggerIsNil,
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

			if got := WithLogger(tt.log)(&Locker{}); !stdErrors.Is(tt.want, got) {
				t.Errorf("apply WithLogger() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyWithOnWaitIterateError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want error
		f    func(ctx context.Context, err error)
		name string
	}{
		{
			name: "empty after iterate func",
			f:    nil,
			want: errors.ErrOnWaitIterateErrorFuncIsNil,
		},
		{
			name: "not empty after iterate func",
			f:    func(_ context.Context, _ error) {},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := WithOnWaitIterateError(tt.f)(&Locker{}); !stdErrors.Is(tt.want, got) {
				t.Errorf("apply WithOnWaitIterateError() = %v, want %v", got, tt.want)
			}
		})
	}
}

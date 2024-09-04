package lock

import (
	stdErrors "errors"
	"testing"
	"time"

	"github.com/sosnovski/synch/locker/errors"
)

func TestApplyWithInstanceID(t *testing.T) {
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

			if got := WithInstanceID(tt.instanceID)(&Params{}); !stdErrors.Is(tt.want, got) {
				t.Errorf("apply WithInstanceID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyWithHeartbeatInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want     error
		name     string
		interval time.Duration
	}{
		{
			name:     "empty interval",
			interval: 0,
			want:     errors.ErrLockHeartbeatIntervalIsEmpty,
		},
		{
			name:     "not empty interval",
			interval: 1,
			want:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := WithHeartbeatInterval(tt.interval)(&Params{}); !stdErrors.Is(tt.want, got) {
				t.Errorf("apply WithHeartbeatInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyWithTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want    error
		name    string
		timeout time.Duration
	}{
		{
			name:    "empty timeout",
			timeout: 0,
			want:    errors.ErrLockTimeoutIsEmpty,
		},
		{
			name:    "not empty timeout",
			timeout: 1,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := WithTimeout(tt.timeout)(&Params{}); !stdErrors.Is(tt.want, got) {
				t.Errorf("apply WithTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyWithData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want error
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
			want: nil,
		},
		{
			name: "nil data",
			data: nil,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := WithData(tt.data)(&Params{}); !stdErrors.Is(tt.want, got) {
				t.Errorf("apply TestWithData() = %v, want %v", got, tt.want)
			}
		})
	}
}

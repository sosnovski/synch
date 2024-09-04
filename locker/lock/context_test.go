package lock

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errTest = errors.New("test error")

func TestSilentCancelContext_Err(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		context func() context.Context
		name    string
	}{
		{
			name: "context.Background",
			context: func() context.Context {
				return context.Background()
			},
			wantErr: nil,
		},
		{
			name: "context.TODO",
			context: func() context.Context {
				return context.TODO()
			},
			wantErr: nil,
		},
		{
			name: "context.WithCancel",
			context: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				return ctx
			},
			wantErr: nil,
		},
		{
			name: "context.WithTimeout",
			context: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 0)
				defer cancel()

				return ctx
			},
			wantErr: context.DeadlineExceeded,
		},
		{
			name: "context.WithDeadline",
			context: func() context.Context {
				ctx, cancel := context.WithDeadline(context.Background(), time.Time{})
				defer cancel()

				return ctx
			},
			wantErr: context.DeadlineExceeded,
		},
		{
			name: "context.WithCancelCause with testErr assert testErr",
			context: func() context.Context {
				ctx, cancel := context.WithCancelCause(context.Background())
				defer cancel(errTest)

				return ctx
			},
			wantErr: errTest,
		},
		{
			name: "context.WithCancelCause with nil error",
			context: func() context.Context {
				ctx, cancel := context.WithCancelCause(context.Background())
				defer cancel(nil)

				return ctx
			},
			wantErr: nil,
		},
		{
			name: "context.WithCancelCause with testErr assert context.Canceled",
			context: func() context.Context {
				ctx, cancel := context.WithCancelCause(context.Background())
				defer cancel(errTest)

				return ctx
			},
			wantErr: context.Canceled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := SilentCancelContext(tt.context()).Err(); !errors.Is(err, tt.wantErr) {
				t.Errorf("Err() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

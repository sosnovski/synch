package log

import (
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"go.uber.org/zap"
)

var errTest = errors.New("some error")

func TestNoopLogger_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		name string
	}{
		{
			name: "with error",
			err:  errTest,
		},
		{
			name: "with nil error",
			err:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			Noop{}.Error(tt.err)
		})
	}
}

func TestNoopLogger_Warn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		name string
	}{
		{
			name: "with error",
			err:  errTest,
		},
		{
			name: "with nil error",
			err:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			Noop{}.Warn(tt.err)
		})
	}
}

func Test_zapWrapper_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		name string
	}{
		{
			name: "with error",
			err:  errTest,
		},
		{
			name: "with nil error",
			err:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			NewFromZap(zap.NewNop()).Error(tt.err)
		})
	}
}

func Test_zapWrapper_Warn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		name string
	}{
		{
			name: "with error",
			err:  errTest,
		},
		{
			name: "with nil error",
			err:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			NewFromZap(zap.NewNop()).Warn(tt.err)
		})
	}
}

func Test_zeroLogWrapper_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		name string
	}{
		{
			name: "with error",
			err:  errTest,
		},
		{
			name: "with nil error",
			err:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			NewFromZerolog(zerolog.Nop()).Error(tt.err)
		})
	}
}

func Test_zeroLogWrapper_Warn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		name string
	}{
		{
			name: "with error",
			err:  errTest,
		},
		{
			name: "with nil error",
			err:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			NewFromZerolog(zerolog.Nop()).Warn(tt.err)
		})
	}
}

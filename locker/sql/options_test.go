package sql

import (
	"context"
	"errors"
	"testing"

	"github.com/sosnovski/synch/log"
)

func TestApplyWithAutoMigration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want   error
		driver func() *Driver
		name   string
		enable bool
	}{
		{
			name:   "enable with implement MigrateDialect",
			enable: true,
			driver: func() *Driver {
				return &Driver{
					dialect: &PostgresDialect{},
				}
			},
			want: nil,
		},
		{
			name:   "enable with not implement MigrateDialect",
			enable: true,
			driver: func() *Driver {
				return &Driver{
					dialect: nil,
				}
			},
			want: ErrDialectNotImplementMigrate,
		},
		{
			name:   "disable with implement MigrateDialect",
			enable: false,
			driver: func() *Driver {
				return &Driver{
					dialect: &PostgresDialect{},
				}
			},
			want: nil,
		},
		{
			name:   "disable with not implement MigrateDialect",
			enable: false,
			driver: func() *Driver {
				return &Driver{
					dialect: nil,
				}
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := WithAutoMigration(tt.enable)(tt.driver()); !errors.Is(tt.want, got) {
				t.Errorf("apply WithAutoMigration() = %v, want %v", got, tt.want)
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

func TestApplyWithMigrationContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ctx    context.Context
		want   error
		driver func() *Driver
		name   string
	}{
		{
			name: "nil context with implement MigrateDialect",
			ctx:  nil,
			driver: func() *Driver {
				return &Driver{
					dialect: &PostgresDialect{},
				}
			},
			want: ErrMigrationContextMustBeSet,
		},
		{
			name: "nil context with not implement MigrateDialect",
			ctx:  nil,
			driver: func() *Driver {
				return &Driver{
					dialect: nil,
				}
			},
			want: ErrMigrationContextMustBeSet,
		},
		{
			name: "not nil context with implement MigrateDialect",
			ctx:  t.Context(),
			driver: func() *Driver {
				return &Driver{
					dialect: &PostgresDialect{},
				}
			},
			want: nil,
		},
		{
			name: "not nil context with not implement MigrateDialect",
			ctx:  t.Context(),
			driver: func() *Driver {
				return &Driver{
					dialect: nil,
				}
			},
			want: ErrDialectNotImplementMigrate,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := WithMigrationContext(tt.ctx)(tt.driver()); !errors.Is(tt.want, got) {
				t.Errorf("apply WithMigrationContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyWithTableName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want      error
		name      string
		tableName string
	}{
		{
			name:      "empty table name",
			tableName: "",
			want:      ErrTableNameIsEmpty,
		},
		{
			name:      "not empty table name",
			tableName: "test-table-name",
			want:      nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := &Driver{}
			if got := WithTableName(tt.tableName)(d); !errors.Is(tt.want, got) {
				t.Errorf("apply WithTableName() = %v, want %v", got, tt.want)
			}
		})
	}
}

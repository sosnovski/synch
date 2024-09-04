package lock

import (
	"context"
	"errors"
	"fmt"
)

// silentCancelContext is a type that embeds the `context.Context` interface.
// It is used to create a context that mute `context.Canceled` error if Cause error is nil.
// Instead of returning `context.Canceled`, it returns `nil`
// when the context is canceled without Cause error.
type silentCancelContext struct {
	context.Context
}

// SilentCancelContext creates a new context that mute `context.Canceled` error
// if Cause error is nil.
func SilentCancelContext(context context.Context) context.Context {
	return &silentCancelContext{Context: context}
}

// Err returns the error associated with the silentCancelContext.
// If the underlying context error is not context.Canceled, it returns the original error from the context.
// If the Cause error is context.Canceled, it returns nil.
// Otherwise, it returns the compound error consisting of the original context error and the Cause error.
func (c *silentCancelContext) Err() error {
	if !errors.Is(c.Context.Err(), context.Canceled) {
		return c.Context.Err() //nolint: wrapcheck // returned original error from context
	}

	err := context.Cause(c.Context)
	if errors.Is(err, context.Canceled) {
		return nil
	}

	return fmt.Errorf("%w: %w", c.Context.Err(), err)
}

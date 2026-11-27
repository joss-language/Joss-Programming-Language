package core

import (
	"context"
	"io"
	"time"
)

// WithResource opens a resource implementing io.Closer, executes use, and
// guarantees Close() is invoked on every exit path (including errors/panics).
func WithResource[T io.Closer](open func() (T, error), use func(T) error) error {
	res, err := open()
	if err != nil {
		return err
	}
	defer func() { _ = res.Close() }()
	return use(res)
}

// WithTimeout creates a bounded context, executes fn, and guarantees cancel()
// is always invoked so timers and goroutines never leak.
func WithTimeout(d time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	return fn(ctx)
}

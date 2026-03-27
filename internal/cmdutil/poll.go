package cmdutil

import (
	"context"
	"time"
)

func PollLoop(ctx context.Context, interval time.Duration, fn func() error) error {
	for {
		if err := fn(); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

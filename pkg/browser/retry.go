package browser

import (
	"context"
	"errors"
	"time"
)

var defaultBackoffs = []time.Duration{
	0,
	20 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	100 * time.Millisecond,
	500 * time.Millisecond,
}

type nonRetriableError struct {
	err error
}

func (e *nonRetriableError) Error() string { return e.err.Error() }
func (e *nonRetriableError) Unwrap() error { return e.err }

func retryWithBackoff(ctx context.Context, action func() error) error {
	var lastErr error
	idx := 0
	for {
		if err := action(); err != nil {
			var nre *nonRetriableError
			if errors.As(err, &nre) {
				return nre.err
			}
			lastErr = err
		} else {
			return nil
		}

		delay := defaultBackoffs[idx]
		if idx < len(defaultBackoffs)-1 {
			idx++
		}

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return lastErr
			}
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

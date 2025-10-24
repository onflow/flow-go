package subscription2

import (
	"context"
	"time"
)

type Subscription interface {
	ID() string
	Channel() <-chan any
	Err() error

	Send(ctx context.Context, value any, timeout time.Duration) error
	CloseWithError(error)
	Close()
}

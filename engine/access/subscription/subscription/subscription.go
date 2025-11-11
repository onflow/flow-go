package subscription

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/subscription"
)

// TODO: add a comment that Subscription is not thread safe

type Subscription[T any] struct {
	id     string
	ch     chan T
	err    error
	once   sync.Once
	closed bool
}

var _ subscription.Subscription[any] = (*Subscription[any])(nil)

func NewSubscription[T any](bufferSize int) *Subscription[T] {
	if bufferSize <= 0 {
		bufferSize = 1
	}

	return &Subscription[T]{
		id: uuid.New().String(),
		ch: make(chan T, bufferSize),
	}
}

func NewFailedSubscription[T any](err error, msg string) *Subscription[T] {
	sub := NewSubscription[T](0)

	// if the error is a grpc error, wrap it to preserve the error code
	if st, ok := status.FromError(err); ok {
		sub.CloseWithError(status.Errorf(st.Code(), "%s: %s", msg, st.Message()))
		return sub
	}

	sub.CloseWithError(fmt.Errorf("%s: %w", msg, err))
	return sub
}

func (s *Subscription[T]) ID() string {
	return s.id
}

func (s *Subscription[T]) Channel() <-chan T {
	return s.ch
}

func (s *Subscription[T]) Err() error {
	return s.err
}

func (s *Subscription[T]) Close() {
	// TODO: do we need to use sync.Once ? the abstraction is not thread safe anyway
	s.once.Do(func() {
		close(s.ch)
		s.closed = true
	})
}
func (s *Subscription[T]) CloseWithError(err error) {
	s.err = err
	s.Close()
}

func (s *Subscription[T]) Send(ctx context.Context, value T, timeout time.Duration) error {
	if s.closed {
		return fmt.Errorf("subscription closed")
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-waitCtx.Done():
		return waitCtx.Err()
	case s.ch <- value:
		return nil
	}
}

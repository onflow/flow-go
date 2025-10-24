package subscription2

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/subscription2"
)

type SubscriptionImpl struct {
	id     string
	ch     chan any
	err    error
	once   sync.Once
	closed bool
}

var _ subscription2.Subscription = (*SubscriptionImpl)(nil)

func NewSubscription(bufferSize int) *SubscriptionImpl {
	if bufferSize <= 0 {
		bufferSize = 1
	}

	return &SubscriptionImpl{
		id: uuid.New().String(),
		ch: make(chan any, bufferSize),
	}
}

func NewFailedSubscription(err error, msg string) *SubscriptionImpl {
	sub := NewSubscription(0)

	// if the error is a grpc error, wrap it to preserve the error code
	if st, ok := status.FromError(err); ok {
		sub.CloseWithError(status.Errorf(st.Code(), "%s: %s", msg, st.Message()))
		return sub
	}

	sub.CloseWithError(fmt.Errorf("%s: %w", msg, err))
	return sub
}

func (s *SubscriptionImpl) ID() string {
	return s.id
}

func (s *SubscriptionImpl) Channel() <-chan any {
	return s.ch
}

func (s *SubscriptionImpl) Err() error {
	return s.err
}

func (s *SubscriptionImpl) Close() {
	s.once.Do(func() {
		close(s.ch)
		s.closed = true
	})
}
func (s *SubscriptionImpl) CloseWithError(err error) {
	s.err = err
	s.Close()
}

func (s *SubscriptionImpl) Send(ctx context.Context, value any, timeout time.Duration) error {
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

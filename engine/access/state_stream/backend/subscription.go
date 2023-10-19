package backend

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/state_stream"
)

// GetDataByHeightFunc is a callback used by subscriptions to retrieve data for a given height.
// Expected errors:
// - storage.ErrNotFound
// - execution_data.BlobNotFoundError
// All other errors are considered exceptions
type GetDataByHeightFunc func(ctx context.Context, height uint64) (interface{}, error)

var _ state_stream.Subscription = (*SubscriptionImpl)(nil)

type SubscriptionImpl struct {
	id string

	// ch is the channel used to pass data to the receiver
	ch chan interface{}

	// err is the error that caused the subscription to fail
	err error

	// once is used to ensure that the channel is only closed once
	once sync.Once

	// closed tracks whether or not the subscription has been closed
	closed bool
}

func NewSubscription(bufferSize int) *SubscriptionImpl {
	return &SubscriptionImpl{
		id: uuid.New().String(),
		ch: make(chan interface{}, bufferSize),
	}
}

// ID returns the subscription ID
// Note: this is not a cryptographic hash
func (sub *SubscriptionImpl) ID() string {
	return sub.id
}

// Channel returns the channel from which subscription data can be read
func (sub *SubscriptionImpl) Channel() <-chan interface{} {
	return sub.ch
}

// Err returns the error that caused the subscription to fail
func (sub *SubscriptionImpl) Err() error {
	return sub.err
}

// Fail registers an error and closes the subscription channel
func (sub *SubscriptionImpl) Fail(err error) {
	sub.err = err
	sub.Close()
}

// Close is called when a subscription ends gracefully, and closes the subscription channel
func (sub *SubscriptionImpl) Close() {
	sub.once.Do(func() {
		close(sub.ch)
		sub.closed = true
	})
}

// Send sends a value to the subscription channel or returns an error
// Expected errors:
// - context.DeadlineExceeded if send timed out
// - context.Canceled if the client disconnected
func (sub *SubscriptionImpl) Send(ctx context.Context, v interface{}, timeout time.Duration) error {
	if sub.closed {
		return fmt.Errorf("subscription closed")
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-waitCtx.Done():
		return waitCtx.Err()
	case sub.ch <- v:
		return nil
	}
}

// NewFailedSubscription returns a new subscription that has already failed with the given error and
// message. This is useful to return an error that occurred during subscription setup.
func NewFailedSubscription(err error, msg string) *SubscriptionImpl {
	sub := NewSubscription(0)

	// if error is a grpc error, wrap it to preserve the error code
	if st, ok := status.FromError(err); ok {
		sub.Fail(status.Errorf(st.Code(), "%s: %s", msg, st.Message()))
		return sub
	}

	// otherwise, return wrap the message normally
	sub.Fail(fmt.Errorf("%s: %w", msg, err))
	return sub
}

var _ state_stream.Subscription = (*HeightBasedSubscription)(nil)
var _ state_stream.Streamable = (*HeightBasedSubscription)(nil)

// HeightBasedSubscription is a subscription that retrieves data sequentially by block height
type HeightBasedSubscription struct {
	*SubscriptionImpl
	nextHeight uint64
	getData    GetDataByHeightFunc
}

func NewHeightBasedSubscription(bufferSize int, firstHeight uint64, getData GetDataByHeightFunc) *HeightBasedSubscription {
	return &HeightBasedSubscription{
		SubscriptionImpl: NewSubscription(bufferSize),
		nextHeight:       firstHeight,
		getData:          getData,
	}
}

// Next returns the value for the next height from the subscription
func (s *HeightBasedSubscription) Next(ctx context.Context) (interface{}, error) {
	v, err := s.getData(ctx, s.nextHeight)
	if err != nil {
		return nil, fmt.Errorf("could not get data for height %d: %w", s.nextHeight, err)
	}
	s.nextHeight++
	return v, nil
}

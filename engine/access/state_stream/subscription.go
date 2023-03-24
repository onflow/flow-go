package state_stream

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// GetDataByHeightFunc is a callback used by subscriptions to retrieve data for a given height.
// Expected errors:
// - storage.ErrNotFound
// - execution_data.BlobNotFoundError
// All other errors are considered exceptions
type GetDataByHeightFunc func(ctx context.Context, height uint64) (interface{}, error)

// Subscription represents a streaming request, and handles the communication between the grpc handler
// and the backend implementation.
type Subscription interface {
	// ID returns the unique identifier for this subscription used for logging
	ID() string

	// Channel returns the channel from which subscriptino data can be read
	Channel() <-chan interface{}

	// Err returns the error that caused the subscription to fail
	Err() error
}

type SubscriptionImpl struct {
	id string

	// ch is the channel used to pass data to the receiver
	ch chan interface{}

	// err is the error that caused the subscription to fail
	err error
}

func NewSubscription() *SubscriptionImpl {
	return &SubscriptionImpl{
		id: uuid.New().String(),
		ch: make(chan interface{}),
	}
}

// ID returns the subscription ID
// Note: this is not a cryptographic hash
func (sub *SubscriptionImpl) ID() string {
	return sub.id
}

// Channel returns the channel from which subscriptino data can be read
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
	close(sub.ch)
}

// Close is called when a subscription ends gracefully, and closes the subscription channel
func (sub *SubscriptionImpl) Close() {
	close(sub.ch)
}

// Send sends a value to the subscription channel or returns an error
// Expected errors:
// - context.DeadlineExceeded if send timed out
// - context.Canceled if the client disconnected
func (sub *SubscriptionImpl) Send(ctx context.Context, v interface{}, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-waitCtx.Done():
		return waitCtx.Err()
	case sub.ch <- v:
		return nil
	}
}

var _ Subscription = (*HeightBasedSubscription)(nil)
var _ Streamable = (*HeightBasedSubscription)(nil)

// HeightBasedSubscription is a subscription that retrieves data sequentially by block height
type HeightBasedSubscription struct {
	*SubscriptionImpl
	nextHeight uint64
	getData    GetDataByHeightFunc
}

func NewHeightBasedSubscription(getData GetDataByHeightFunc) *HeightBasedSubscription {
	return &HeightBasedSubscription{
		SubscriptionImpl: NewSubscription(),
		getData:          getData,
	}
}

// Next returns the value for the next height from the subscription
func (s *HeightBasedSubscription) Next(ctx context.Context) (interface{}, error) {
	v, err := s.getData(ctx, s.nextHeight)
	if err != nil {
		return nil, err
	}
	s.nextHeight++
	return v, nil
}

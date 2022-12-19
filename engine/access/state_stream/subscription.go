package state_stream

import (
	"context"
	"fmt"
	"time"

	"github.com/onflow/flow-go/utils/unittest"
)

type GetDataByHeightFunc func(ctx context.Context, height uint64) (interface{}, error)

type Subscription interface {
	ID() string
	Done()
	Send(context.Context, interface{}, time.Duration) error
	Fail(error)
	Channel() <-chan interface{}
	Err() error
}

type SubscriptionImpl struct {
	id  string
	ch  chan interface{}
	err error
}

func NewSubscription() *SubscriptionImpl {
	return &SubscriptionImpl{
		// TODO: don't use unittest package
		id: unittest.GenerateRandomStringWithLen(16),
		ch: make(chan interface{}),
	}
}

func (sub *SubscriptionImpl) ID() string {
	return sub.id
}

func (sub *SubscriptionImpl) Channel() <-chan interface{} {
	return sub.ch
}

func (sub *SubscriptionImpl) Err() error {
	return sub.err
}

func (sub *SubscriptionImpl) Fail(err error) {
	sub.err = err
	close(sub.ch)
}

func (sub *SubscriptionImpl) Done() {
	close(sub.ch)
}

func (sub *SubscriptionImpl) Send(ctx context.Context, v interface{}, timeout time.Duration) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("client disconnected")
	case <-time.After(timeout):
		return fmt.Errorf("timeout sending response")
	case sub.ch <- v:
		return nil
	}
}

var _ Subscription = (*HeightBasedSubscription)(nil)
var _ Streamable = (*HeightBasedSubscription)(nil)

type HeightBasedSubscription struct {
	*SubscriptionImpl
	nextHeight uint64
	getData    GetDataByHeightFunc
}

func (s *HeightBasedSubscription) Next(ctx context.Context) (interface{}, error) {
	v, err := s.getData(ctx, s.nextHeight)
	if err != nil {
		return nil, err
	}
	s.nextHeight++
	return v, nil
}

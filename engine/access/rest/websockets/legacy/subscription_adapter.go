package legacy

import (
	"sync"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription_old"
)

// AdaptSubscription wraps a typed generic subscription into a legacy untyped subscription
// used by the legacy websocket controller.
func AdaptSubscription[T any](sub subscription.Subscription[T]) subscription_old.Subscription {
	adapter := &legacySubscriptionAdapter[T]{
		sub: sub,
		ch:  make(chan interface{}),
	}
	adapter.start()
	return adapter
}

type legacySubscriptionAdapter[T any] struct {
	sub  subscription.Subscription[T]
	ch   chan any
	once sync.Once
	err  error
}

func (a *legacySubscriptionAdapter[T]) ID() string { return a.sub.ID() }

func (a *legacySubscriptionAdapter[T]) Channel() <-chan any { return a.ch }

func (a *legacySubscriptionAdapter[T]) Err() error { return a.err }

func (a *legacySubscriptionAdapter[T]) start() {
	go func() {
		defer close(a.ch)
		for v := range a.sub.Channel() {
			// forward as interface{}
			a.ch <- any(v)
		}
		// capture terminal error if any
		a.err = a.sub.Err()
	}()
}

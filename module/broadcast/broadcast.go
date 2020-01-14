package broadcast

import (
	"fmt"
	"sync"

	"github.com/dapperlabs/flow-go/module"
)

type Subscription struct {
	id          int
	ch          <-chan struct{}
	unsubscribe func() error
}

func (s *Subscription) Ch() <-chan struct{} {
	return s.ch
}

func (s *Subscription) Unsubscribe() error {
	return s.unsubscribe()
}

type Broadcaster struct {
	subscriptions map[int]chan struct{}
	mu            sync.Mutex
	nextID        int
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscriptions: make(map[int]chan struct{}),
	}
}

func (b *Broadcaster) Subscribe() module.Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextID++
	id := b.nextID

	ch := make(chan struct{})
	b.subscriptions[id] = ch

	unsubscribe := func() error {
		return b.unsubscribe(id)
	}

	sub := Subscription{
		id:          id,
		ch:          ch,
		unsubscribe: unsubscribe,
	}

	return &sub
}

func (b *Broadcaster) Broadcast() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, ch := range b.subscriptions {
		ch <- struct{}{}
	}
}

func (b *Broadcaster) unsubscribe(id int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch, ok := b.subscriptions[id]
	if !ok {
		return fmt.Errorf("cannot unsubscribe non-existent subscription")
	}

	close(ch)
	delete(b.subscriptions, id)
	return nil
}

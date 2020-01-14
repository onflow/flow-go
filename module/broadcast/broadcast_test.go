package broadcast_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/broadcast"
)

// Counter implements a thread-safe counter for tracking received broadcasts.
type Counter struct {
	mu    sync.Mutex
	count int
}

func (c *Counter) Add() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count++
}

func (c *Counter) N() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.count
}

func TestBroadcast(t *testing.T) {
	caster := broadcast.NewBroadcaster()

	var (
		wg          sync.WaitGroup
		counter     Counter
		N           = 100
		subscribers = make([]module.Subscription, N)
	)

	for i := range subscribers {
		sub := caster.Subscribe()
		subscribers[i] = sub

		go func() {
			wg.Add(1)
			defer wg.Done()

			select {
			case <-time.After(time.Second):
				t.Fail()
			case <-sub.Ch():
				counter.Add()
			}
		}()
	}

	caster.Broadcast()
	wg.Wait()

	assert.Equal(t, len(subscribers), counter.N())
}

func TestUnsubscribe(t *testing.T) {
	caster := broadcast.NewBroadcaster()

	var (
		wg          sync.WaitGroup
		counter     Counter // keep track of received broadcasts
		N           = 100
		subscribers = make([]module.Subscription, N)
	)

	for i := range subscribers {
		sub := caster.Subscribe()
		subscribers[i] = sub

		go func() {
			wg.Add(1)
			defer wg.Done()

			select {
			case <-time.After(time.Second):
				t.Fail()
			case _, ok := <-sub.Ch():
				if !ok {
					// channel closed
					return
				}
				counter.Add()
			}
		}()
	}

	// unsubscribe half of the subscribers
	// the unsubscribed subscribers should have exited their goroutine
	for i := 0; i < N/2; i++ {
		err := subscribers[i].Unsubscribe()
		assert.Nil(t, err)
	}

	caster.Broadcast()
	wg.Wait()

	assert.Equal(t, N/2, counter.N())
}

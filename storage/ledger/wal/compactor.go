package wal

import (
	"fmt"
	"io"
	"sync"
	"time"
)

type Compactor struct {
	checkpointer *Checkpointer
	done         chan struct{}
	stopc        chan struct{}
	wg           sync.WaitGroup
}

func NewCompactor(checkpointer *Checkpointer) *Compactor {
	return &Compactor{
		checkpointer: checkpointer,
		done:         make(chan struct{}),
		stopc:        make(chan struct{}),
	}
}

func (c *Compactor) Ready() <-chan struct{} {
	ch := make(chan struct{})
	defer close(ch)
	return ch
}

func (c *Compactor) Done() <-chan struct{} {
	c.stopc <- struct{}{}

	ch := make(chan struct{})

	go func() {
		c.wg.Wait()
		close(ch)
	}()

	return ch
}

// Start periodically fires Run function, every `interval`
// This function blocks until the `Done()` is called, so it's advised to
// run in it different thread.
// If called more then once, behaviour is undefined.
func (c *Compactor) Start(interval time.Duration) {
	c.wg.Add(1)

	for {
		if err := c.Run(); err != nil {
			//c.log.Error(err).Log("error running compactor", )
		}

		select {
		case <-c.stopc:
			c.wg.Done()
			return
		case <-time.After(interval):
		}
	}
}

func (c *Compactor) Run() error {
	from, to, err := c.checkpointer.NotCheckpointedSegments()
	if err != nil {
		return fmt.Errorf("cannot get latest checkpoint: %w", err)
	}

	// more then one segment means we can checkpoint safely up to `to`-1
	// presumably last segment is being written to
	if to-from > 1 {
		err = c.checkpointer.Checkpoint(to-1, func() (io.WriteCloser, error) {
			return c.checkpointer.CheckpointWriter(to - 1)
		})
		if err != nil {
			return fmt.Errorf("error creating checkpoint: %w", err)
		}
	}
	return nil
}

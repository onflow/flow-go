package wal

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/onflow/flow-go/module/lifecycle"
)

type Compactor struct {
	checkpointer *Checkpointer
	stopc        chan struct{}
	lm           *lifecycle.LifecycleManager
	sync.Mutex
	interval           time.Duration
	checkpointDistance uint
	checkpointsToKeep  uint
}

func NewCompactor(checkpointer *Checkpointer, interval time.Duration, checkpointDistance uint, checkpointsToKeep uint) *Compactor {
	if checkpointDistance < 1 {
		checkpointDistance = 1
	}
	return &Compactor{
		checkpointer:       checkpointer,
		stopc:              make(chan struct{}),
		lm:                 lifecycle.NewLifecycleManager(),
		interval:           interval,
		checkpointDistance: checkpointDistance,
		checkpointsToKeep:  checkpointsToKeep,
	}
}

// Ready periodically fires Run function, every `interval`
func (c *Compactor) Ready() <-chan struct{} {
	c.lm.OnStart(func() {
		go c.start()
	})
	return c.lm.Started()
}

func (c *Compactor) Done() <-chan struct{} {
	c.lm.OnStop(func() {
		c.stopc <- struct{}{}
	})
	return c.lm.Stopped()
}

func (c *Compactor) start() {
	for {
		//TODO Log error
		_ = c.Run()

		select {
		case <-c.stopc:
			return
		case <-time.After(c.interval):
		}
	}
}

func (c *Compactor) Run() error {
	c.Lock()
	defer c.Unlock()

	err := c.createCheckpoints()
	if err != nil {
		return fmt.Errorf("cannot create checkpoints: %w", err)
	}

	err = c.cleanupCheckpoints()
	if err != nil {
		return fmt.Errorf("cannot cleanup checkpoints: %w", err)
	}

	return nil
}

func (c *Compactor) createCheckpoints() error {
	from, to, err := c.checkpointer.NotCheckpointedSegments()
	if err != nil {
		return fmt.Errorf("cannot get latest checkpoint: %w", err)
	}

	fmt.Printf("creating a checkpoint from segment %d to segment %d\n", from, to)

	// more then one segment means we can checkpoint safely up to `to`-1
	// presumably last segment is being written to
	if to-from > int(c.checkpointDistance) {
		checkpointNumber := to - 1
		fmt.Printf("checkpointing to %d\n", checkpointNumber)

		err = c.checkpointer.Checkpoint(checkpointNumber, func() (io.WriteCloser, error) {
			return c.checkpointer.CheckpointWriter(checkpointNumber)
		})
		if err != nil {
			return fmt.Errorf("error creating checkpoint (%d): %w", checkpointNumber, err)
		}
	}
	return nil
}

func (c *Compactor) cleanupCheckpoints() error {
	// don't bother listing checkpoints if we keep them all
	if c.checkpointsToKeep == 0 {
		return nil
	}
	checkpoints, err := c.checkpointer.Checkpoints()
	if err != nil {
		return fmt.Errorf("cannot list checkpoints: %w", err)
	}
	if len(checkpoints) > int(c.checkpointsToKeep) {
		checkpointsToRemove := checkpoints[:len(checkpoints)-int(c.checkpointsToKeep)] // if condition guarantees this never fails

		for _, checkpoint := range checkpointsToRemove {
			err := c.checkpointer.RemoveCheckpoint(checkpoint)
			if err != nil {
				return fmt.Errorf("cannot remove checkpoint %d: %w", checkpoint, err)
			}
		}
	}
	return nil
}

package integration_test

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
)

type CounterConsumer struct {
	notifications.NoopConsumer
	total     uint
	finalized func(uint)
}

func (c *CounterConsumer) OnFinalizedBlock(block *model.Block) {
	c.total++

	// notify stopper of total finalized
	c.finalized(c.total)
}

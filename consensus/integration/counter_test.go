package integration_test

import (
	"time"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/rs/zerolog"
)

type CounterConsumer struct {
	notifications.NoopConsumer
	log       zerolog.Logger
	interval  time.Duration
	next      time.Time
	count     uint
	total     uint
	finalized func(uint)
}

func (c *CounterConsumer) OnFinalizedBlock(block *model.Block) {

	// count the finalized block
	c.count++
	c.total++

	// notify stopper of total finalized
	c.finalized(c.total)

	// if we are still before the next printing, skip rest
	diff := time.Now().Sub(c.next)
	if diff < c.interval {
		return
	}

	// otherwise, print number of finalized count and reset
	c.log.Info().Uint("count", c.count).Dur("ms", diff).Uint("total", c.total).Msg("finalized blocks")
	c.next = c.next.Add(c.interval)
	c.count = 0
}

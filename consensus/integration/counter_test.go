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
	ma        []uint
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

	// append the values to the moving average and shorten as makes sense
	c.ma = append(c.ma, c.count)
	if len(c.ma) > 300 {
		c.ma = c.ma[1:]
	}

	// calculate 60 second moving average
	var ma60 float32
	end60 := len(c.ma)
	start60 := len(c.ma) - 60
	if start60 < 0 {
		start60 = 0
	}
	for _, count := range c.ma[start60:end60] {
		ma60 += float32(count)
	}
	ma60 = ma60 / float32(end60-start60)

	// calculate 300 second moving average
	var ma300 float32
	end300 := len(c.ma)
	start300 := len(c.ma) - 300
	if start300 < 0 {
		start300 = 0
	}
	for _, count := range c.ma[start300:end300] {
		ma300 += float32(count)
	}
	ma300 = ma300 / float32(end300-start300)

	// otherwise, print number of finalized count and reset
	c.log.Info().
		Uint("rate", c.count).
		Float32("ma_60", ma60).
		Float32("ma_300", ma300).
		Uint("total", c.total).Msg("finalized blocks")

	// reset count and next log time
	c.next = c.next.Add(c.interval)
	c.count = 0
}

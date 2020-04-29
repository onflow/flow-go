package integration_test

import (
	"time"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/rs/zerolog"
)

type CounterConsumer struct {
	notifications.NoopConsumer
	log      zerolog.Logger
	interval time.Duration
	next     time.Time
	counter  uint
	total    uint
}

func NewCounterConsumer(log zerolog.Logger) *CounterConsumer {
	cc := CounterConsumer{
		log:      log,
		interval: time.Second,
		next:     time.Now().UTC().Add(time.Second),
		counter:  0,
		total:    0,
	}
	return &cc
}

func (c *CounterConsumer) OnFinalizedBlock(block *model.Block) {

	// count the finalized block
	c.counter++
	c.total++

	// if we are still before the next printing, skip rest
	now := time.Now().UTC()
	if now.Before(c.next) {
		return
	}

	// otherwise, print number of finalized blocks and reset
	c.log.Info().Dur("interval", c.interval).Uint("counter", c.counter).Uint("total", c.total).Msg("finalized blocks counter")
	c.next = now.Add(c.interval)
	c.counter = 0
}

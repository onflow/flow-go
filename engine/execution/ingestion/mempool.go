package ingestion

//revive:disable:unexported-return

import (
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
)

type Mempool struct {
	ExecutionQueue    *stdmap.Queues
	BlockByCollection *stdmap.BlockByCollections
}

func (m *Mempool) Run(f func(blockByCollection *stdmap.BlockByCollectionBackdata, executionQueue *stdmap.QueuesBackdata) error) error {
	return m.ExecutionQueue.Run(func(queueBackdata *stdmap.QueuesBackdata) error {
		return m.BlockByCollection.Run(func(blockByCollectionBackdata *stdmap.BlockByCollectionBackdata) error {
			return f(blockByCollectionBackdata, queueBackdata)
		})
	})
}

func newMempool() *Mempool {
	m := &Mempool{
		BlockByCollection: stdmap.NewBlockByCollections(),
		ExecutionQueue:    stdmap.NewQueues(),
	}

	return m
}

package ingestion

//revive:disable:unexported-return

import (
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
)

type Mempool struct {
	BlockByCollection *stdmap.BlockByCollections
	ExecutionQueue    *stdmap.Queues
	OrphanQueue       *stdmap.Queues
	SyncQueues        *stdmap.Queues
}

func (m *Mempool) Run(f func(blockByCollection *stdmap.BlockByCollectionBackdata, executionQueue *stdmap.QueuesBackdata, orphanQueue *stdmap.QueuesBackdata) error) error {
	return m.ExecutionQueue.Run(func(queueBackdata *stdmap.QueuesBackdata) error {
		return m.BlockByCollection.Run(func(blockByCollectionBackdata *stdmap.BlockByCollectionBackdata) error {
			return m.OrphanQueue.Run(func(orphanBackdata *stdmap.QueuesBackdata) error {
				return f(blockByCollectionBackdata, queueBackdata, orphanBackdata)
			})
		})
	})
}

func newMempool() *Mempool {
	m := &Mempool{
		BlockByCollection: stdmap.NewBlockByCollections(),
		ExecutionQueue:    stdmap.NewQueues(),
		OrphanQueue:       stdmap.NewQueues(),
		SyncQueues:        stdmap.NewQueues(),
	}

	return m
}

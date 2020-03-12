package ingestion

//revive:disable:unexported-return

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
)

type Mempool struct {
	BlockByCollection BlockByCollectionMempool
	ExecutionQueue    QueueMempool
	OrphanQueue       QueueMempool
}

func (m *Mempool) Run(f func(blockByCollection *BlockByCollectionBackdata, executionQueue *QueuesBackdata, orphanQueue *QueuesBackdata) error) error {
	return m.ExecutionQueue.Run(func(queueBackdata *QueuesBackdata) error {
		return m.BlockByCollection.Run(func(blockByCollectionBackdata *BlockByCollectionBackdata) error {
			return m.OrphanQueue.Run(func(orphanBackdata *QueuesBackdata) error {
				return f(blockByCollectionBackdata, queueBackdata, orphanBackdata)
			})
		})
	})
}

type BlockByCollectionMempool struct {
	*stdmap.Backend
}

type QueueMempool struct {
	*stdmap.Backend
}

type blockByCollection struct {
	CollectionID  flow.Identifier
	CompleteBlock *execution.CompleteBlock
}

func (b *blockByCollection) ID() flow.Identifier {
	return b.CollectionID
}

func (b *blockByCollection) Checksum() flow.Identifier {
	return b.CollectionID
}

// BlockByCollectionBackdata is mempool map for blockByCollection
type BlockByCollectionBackdata struct {
	*stdmap.Backdata
}

// QueuesBackdata is mempool map for ingestion.Queue (head Node ID -> Queue)
type QueuesBackdata struct {
	*stdmap.Backdata
}

func (b *BlockByCollectionBackdata) ByID(id flow.Identifier) (*blockByCollection, error) {
	entity, err := b.Backdata.ByID(id)
	if err != nil {
		return nil, err
	}
	block, ok := entity.(*blockByCollection)
	if !ok {
		panic(fmt.Sprintf("invalid entity in complete block mempool (%T)", entity))
	}
	return block, nil
}

func (b *QueuesBackdata) ByID(id flow.Identifier) (*Queue, error) {
	entity, err := b.Backdata.ByID(id)
	if err != nil {
		return nil, err
	}
	block, ok := entity.(*Queue)
	if !ok {
		panic(fmt.Sprintf("invalid entity in complete block mempool (%T)", entity))
	}
	return block, nil
}

func (b *QueuesBackdata) All() []*Queue {
	entities := b.Backdata.All()

	queues := make([]*Queue, len(entities))
	for i, entity := range entities {
		queue, ok := entity.(*Queue)
		if !ok {
			panic(fmt.Sprintf("invalid entity in queue mempool (%T)", entity))
		}
		queues[i] = queue
	}
	return queues
}

func newMempool() *Mempool {
	m := &Mempool{
		BlockByCollection: BlockByCollectionMempool{Backend: stdmap.NewBackend()},
		ExecutionQueue:    QueueMempool{Backend: stdmap.NewBackend()},
		OrphanQueue:       QueueMempool{Backend: stdmap.NewBackend()},
	}

	return m
}

func (b *BlockByCollectionMempool) Add(block *blockByCollection) error {
	return b.Backend.Add(block)
}

func (b *BlockByCollectionMempool) Get(id flow.Identifier) (*blockByCollection, error) {
	backdata := &BlockByCollectionBackdata{b.Backdata}
	return backdata.ByID(id)
}

func (b *BlockByCollectionMempool) Run(f func(backdata *BlockByCollectionBackdata) error) error {
	b.Lock()
	defer b.Unlock()

	err := f(&BlockByCollectionBackdata{b.Backdata})
	if err != nil {
		return err
	}
	return nil
}

func (b *QueueMempool) Add(queue *Queue) error {
	return b.Backend.Add(queue)
}

func (b *QueueMempool) Get(id flow.Identifier) (*Queue, error) {
	backdata := &QueuesBackdata{&b.Backdata}
	return backdata.ByID(id)
}

func (b *QueueMempool) Run(f func(backdata *QueuesBackdata) error) error {
	b.Lock()
	defer b.Unlock()

	err := f(&QueuesBackdata{&b.Backdata})
	if err != nil {
		return err
	}
	return nil
}

package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/queue"
)

type Queues struct {
	*Backend
}

// QueuesBackdata is mempool map for ingestion.Queues (head Node ID -> Queues)
type QueuesBackdata struct {
	*Backdata
}

func NewQueues() *Queues {
	return &Queues{NewBackend(WithEject(EjectPanic))}
}

func (b *QueuesBackdata) ByID(id flow.Identifier) (*queue.Queue, error) {
	entity, err := b.Backdata.ByID(id)
	if err != nil {
		return nil, err
	}
	block, ok := entity.(*queue.Queue)
	if !ok {
		panic(fmt.Sprintf("invalid entity in complete block mempool (%T)", entity))
	}
	return block, nil
}

func (b *QueuesBackdata) All() []*queue.Queue {
	entities := b.Backdata.All()

	queues := make([]*queue.Queue, len(entities))
	for i, entity := range entities {
		queue, ok := entity.(*queue.Queue)
		if !ok {
			panic(fmt.Sprintf("invalid entity in queue mempool (%T)", entity))
		}
		queues[i] = queue
	}
	return queues
}

func (b *Queues) Add(queue *queue.Queue) error {
	return b.Backend.Add(queue)
}

func (b *Queues) Get(id flow.Identifier) (*queue.Queue, error) {
	backdata := &QueuesBackdata{&b.Backdata}
	return backdata.ByID(id)
}

func (b *Queues) Run(f func(backdata *QueuesBackdata) error) error {
	b.Lock()
	defer b.Unlock()

	err := f(&QueuesBackdata{&b.Backdata})
	if err != nil {
		return err
	}
	return nil
}

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

func (b *QueuesBackdata) ByID(queueID flow.Identifier) (*queue.Queue, bool) {
	entity, exists := b.Backdata.ByID(queueID)
	if !exists {
		return nil, false
	}
	queue := entity.(*queue.Queue)
	return queue, true
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

func (b *Queues) Add(queue *queue.Queue) bool {
	return b.Backend.Add(queue)
}

func (b *Queues) Get(queueID flow.Identifier) (*queue.Queue, bool) {
	backdata := &QueuesBackdata{&b.Backdata}
	return backdata.ByID(queueID)
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

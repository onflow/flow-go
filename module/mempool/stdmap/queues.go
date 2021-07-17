package stdmap

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/utils/binstat"
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
	bs1 := binstat.EnterTime("~4lock:w:Backend.Run(Queues)")
	bs1.Run(func() {
		b.Lock()
	})
	bs1.Leave()

	var err error
	bs2 := binstat.EnterTime("~7Backend.Run(Queues)")
	bs2.Run(func() {
		defer b.Unlock()
		err = f(&QueuesBackdata{&b.Backdata})
	})
	bs2.Leave()

	if err != nil {
		return err
	}
	return nil
}
